<?php
declare(strict_types=1);

namespace PayOauth;

use PDOException;
use RuntimeException;

/**
 * 把 OAuth2 身份映射到 pay（ls-go）本地账号，并签发等价于原生登录的会话。
 *
 * 会话模型来自对 ls-go 的实际验证：
 *   Cookie: front_token = ls_user.token
 *   ls_user.token_expire 为 Unix 秒；过期返回「登录已过期」
 *   服务端同时接受 Authorization: Bearer <token>
 * 因此这里只需要复用 pay 自己的 token 字段，不伪造签名、不改动主程序。
 */
final class PayAccount
{
    private Config $config;
    private string $provider;

    public function __construct(Config $config)
    {
        $this->config = $config;
        $this->provider = substr(
            hash('sha256', (string) $config->get('oauth.issuer') . '|' . (string) $config->get('oauth.client_id')),
            0,
            24
        );
    }

    public function provider(): string
    {
        return $this->provider;
    }

    public function loginHours(): int
    {
        $key = (string) $this->config->get('pay.login_hours_key', 'user_login_token_hours');
        $row = Database::fetchOne('SELECT config_value FROM admin_config WHERE config_name = ? LIMIT 1', [$key]);
        $hours = (int) ($row['config_value'] ?? 0);
        if ($hours <= 0) {
            $hours = (int) $this->config->get('pay.default_login_hours', 168);
        }
        return max(1, min($hours, 24 * 365));
    }

    public function identity(string $subject): ?array
    {
        return Database::fetchOne(
            'SELECT * FROM ls_oauth_identity WHERE provider = ? AND subject = ? LIMIT 1',
            [$this->provider, $subject]
        );
    }

    public function userById(int $userId): ?array
    {
        return Database::fetchOne(
            'SELECT id, username, email, mobile, is_frozen, frozen_reason, token, token_expire FROM ls_user WHERE id = ? LIMIT 1',
            [$userId]
        );
    }

    public function userByEmail(string $email): ?array
    {
        if ($email === '') {
            return null;
        }
        return Database::fetchOne(
            'SELECT id, username, email, mobile, is_frozen, frozen_reason FROM ls_user WHERE email = ? ORDER BY id LIMIT 1',
            [$email]
        );
    }

    /**
     * 解析身份 → 本地用户；不存在时按策略自动建号。
     *
     * @param array $profile 已从 id_token / userinfo 提取：subject, username, email, name, email_verified
     * @return array{user:array, created:bool, bound:bool}
     */
    public function resolve(array $profile): array
    {
        $subject = (string) $profile['subject'];
        $existing = $this->identity($subject);
        if ($existing !== null) {
            $user = $this->userById((int) $existing['user_id']);
            if ($user === null) {
                // 身份指向的账号已被删除：清理脏绑定后走新建流程
                Database::execute('DELETE FROM ls_oauth_identity WHERE id = ?', [(int) $existing['id']]);
            } else {
                $this->touchIdentity((int) $existing['id'], $profile);
                return ['user' => $user, 'created' => false, 'bound' => false];
            }
        }

        if ((bool) $this->config->get('account.link_by_email', false)
            && (int) ($profile['email_verified'] ?? 0) === 1
            && (string) ($profile['email'] ?? '') !== ''
        ) {
            $user = $this->userByEmail((string) $profile['email']);
            if ($user !== null) {
                $this->bind((int) $user['id'], $profile);
                return ['user' => $user, 'created' => false, 'bound' => true];
            }
        }

        if (!(bool) $this->config->get('account.auto_register', true)) {
            throw new RuntimeException('该 SSO 身份尚未绑定本地账号，请先用账号密码登录后在「账号绑定」中关联');
        }

        $user = $this->createUser($profile);
        return ['user' => $user, 'created' => true, 'bound' => false];
    }

    public function bind(int $userId, array $profile): void
    {
        $subject = (string) $profile['subject'];
        try {
            Database::insert(
                'INSERT INTO ls_oauth_identity
                   (provider, subject, user_id, username, email, email_verified, created_at, last_login_at)
                 VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())',
                [
                    $this->provider,
                    $subject,
                    $userId,
                    substr((string) ($profile['username'] ?? ''), 0, 100),
                    substr((string) ($profile['email'] ?? ''), 0, 191),
                    (int) ($profile['email_verified'] ?? 0),
                ]
            );
        } catch (PDOException $e) {
            if (!Database::isDuplicateKey($e)) {
                throw $e;
            }
            $row = $this->identity($subject);
            if ($row === null || (int) $row['user_id'] !== $userId) {
                throw new RuntimeException('该 SSO 身份已绑定到其他账号');
            }
        }
    }

    private function touchIdentity(int $id, array $profile): void
    {
        Database::execute(
            'UPDATE ls_oauth_identity
                SET last_login_at = NOW(), last_ip = ?, username = ?, email = ?, email_verified = ?
              WHERE id = ?',
            [
                substr(Support::clientIp(), 0, 64),
                substr((string) ($profile['username'] ?? ''), 0, 100),
                substr((string) ($profile['email'] ?? ''), 0, 191),
                (int) ($profile['email_verified'] ?? 0),
                $id,
            ]
        );
    }

    /**
     * 建号：密码为随机 bcrypt（无人知晓，只走 SSO），并补齐 ls_userbasic。
     * 唯一键冲突时重试，保证并发下只有一个账号。
     */
    private function createUser(array $profile): array
    {
        $freeze = (bool) $this->config->get('account.freeze_new_user', false);
        $superiorId = (int) $this->config->get('account.superior_id', 0);
        $password = password_hash(bin2hex(random_bytes(16)), PASSWORD_BCRYPT);

        for ($attempt = 0; $attempt < 5; $attempt++) {
            $username = $this->uniqueUsername((string) ($profile['username'] ?? ''), (string) $profile['subject'], $attempt);
            $inviteCode = Support::randomCode(8);
            try {
                $userId = Database::transaction(function () use ($username, $inviteCode, $password, $profile, $freeze, $superiorId) {
                    $id = Database::insert(
                        "INSERT INTO ls_user
                           (username, password, salt, invite_code, superior_id, aff_visits, email, qq, commission,
                            user_key, is_bindqq, is_bindwx, totp_secret, totp_enabled, is_frozen, is_reg_paid, create_time)
                         VALUES (?, ?, '', ?, ?, 0, ?, '', 0, ?, 0, 0, '', 0, ?, 0, NOW())",
                        [
                            $username,
                            $password,
                            $inviteCode,
                            $superiorId > 0 ? $superiorId : null,
                            substr((string) ($profile['email'] ?? ''), 0, 50),
                            bin2hex(random_bytes(16)),
                            $freeze ? 1 : 0,
                        ]
                    );
                    Database::insert('INSERT INTO ls_userbasic (user_id) VALUES (?)', [$id]);
                    $this->bind($id, $profile);
                    return $id;
                });
                $user = $this->userById($userId);
                if ($user === null) {
                    throw new RuntimeException('建号后读取用户失败');
                }
                return $user;
            } catch (PDOException $e) {
                if (!Database::isDuplicateKey($e)) {
                    throw $e;
                }
                // invite_code 或 (provider, subject) 撞车：重新生成后重试
            }
        }
        throw new RuntimeException('多次重试仍无法创建本地账号');
    }

    private function uniqueUsername(string $preferred, string $subject, int $attempt): string
    {
        $max = max(8, min((int) $this->config->get('account.username_max', 24), 40));
        $base = preg_replace('/[^A-Za-z0-9_]/', '', $preferred) ?? '';
        $base = trim($base, '_');
        if (strlen($base) < 3) {
            $base = 'sso_' . substr(hash('sha256', $subject), 0, 8);
        }
        $base = substr($base, 0, $max);
        if ($attempt === 0 && $this->usernameFree($base)) {
            return $base;
        }
        for ($i = 1; $i <= 20; $i++) {
            $candidate = substr($base, 0, $max - 4) . '_' . Support::randomCode(3, '0123456789');
            if ($this->usernameFree($candidate)) {
                return $candidate;
            }
        }
        return substr($base, 0, $max - 9) . '_' . substr(hash('sha256', $subject . $attempt . microtime()), 0, 8);
    }

    private function usernameFree(string $username): bool
    {
        $row = Database::fetchOne('SELECT id FROM ls_user WHERE username = ? LIMIT 1', [$username]);
        return $row === null;
    }

    /** 签发会话：直接复用 pay 的 token 字段 + 过期时间 */
    public function mintToken(int $userId, int $hours): string
    {
        $token = Support::tokenLikeLsGo();
        $expire = time() + $hours * 3600;
        Database::execute(
            'UPDATE ls_user SET token = ?, token_expire = ? WHERE id = ?',
            [$token, $expire, $userId]
        );
        return $token;
    }

    /** 退出登录时同时作废服务端 token（避免只清浏览器 Cookie） */
    public function revokeToken(int $userId, string $token): void
    {
        if ($token === '' || $userId <= 0) {
            return;
        }
        Database::execute(
            "UPDATE ls_user SET token = '', token_expire = 0 WHERE id = ? AND token = ?",
            [$userId, $token]
        );
    }

    public function userByToken(string $token): ?array
    {
        if ($token === '') {
            return null;
        }
        return Database::fetchOne(
            'SELECT id, username, email, token_expire FROM ls_user WHERE token = ? LIMIT 1',
            [$token]
        );
    }

    public function assertActive(array $user): void
    {
        if ((int) ($user['is_frozen'] ?? 0) === 1) {
            $reason = trim((string) ($user['frozen_reason'] ?? ''));
            throw new RuntimeException('账号已被冻结' . ($reason === '' ? '' : '：' . $reason));
        }
    }
}
