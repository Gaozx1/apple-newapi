<?php
declare(strict_types=1);

namespace PayOauth;

/**
 * 审计与限流。
 * 审计只记录非敏感上下文（事件、IP、subject 摘要、用户 ID、结果），
 * 不记录 code、token、client_secret、密码或任何可用凭据。
 */
final class Audit
{
    public static function record(string $event, string $result, array $context = []): void
    {
        $subject = (string) ($context['subject'] ?? '');
        try {
            Database::insert(
                'INSERT INTO ls_oauth_audit (event, result, subject_hash, user_id, ip, detail, created_at)
                 VALUES (?, ?, ?, ?, ?, ?, NOW())',
                [
                    substr($event, 0, 32),
                    substr($result, 0, 32),
                    $subject === '' ? '' : substr(hash('sha256', $subject), 0, 32),
                    (int) ($context['user_id'] ?? 0),
                    substr(Support::clientIp(), 0, 64),
                    substr((string) ($context['detail'] ?? ''), 0, 500),
                ]
            );
        } catch (\Throwable $e) {
            Support::log('audit failed: ' . $e->getMessage());
        }
    }

    /**
     * 固定窗口限流：同一 IP 在 window 秒内最多 limit 次。
     * 用 MySQL 的原子 upsert 保证并发下计数准确。
     */
    public static function hit(string $bucket, int $limit, int $window = 60): bool
    {
        if ($limit <= 0) {
            return true;
        }
        $ip = Support::clientIp();
        $key = substr(hash('sha256', $ip . '|' . $bucket), 0, 40);
        $now = time();
        $windowStart = intdiv($now, $window) * $window;
        try {
            Database::execute(
                'INSERT INTO ls_oauth_rate (bucket, window_start, hits) VALUES (?, ?, 1)
                 ON DUPLICATE KEY UPDATE
                   hits = IF(window_start < VALUES(window_start), 1, hits + 1),
                   window_start = VALUES(window_start)',
                [$key, $windowStart]
            );
            $row = Database::fetchOne('SELECT hits FROM ls_oauth_rate WHERE bucket = ?', [$key]);
            return (int) ($row['hits'] ?? 0) <= $limit;
        } catch (\Throwable $e) {
            Support::log('rate limit failed: ' . $e->getMessage());
            return true; // 限流自身故障不阻断登录，但要留痕
        }
    }

    public static function cleanup(): void
    {
        try {
            Database::execute('DELETE FROM ls_oauth_rate WHERE window_start < ?', [time() - 3600]);
        } catch (\Throwable $e) {
            Support::log('rate cleanup failed: ' . $e->getMessage());
        }
    }
}
