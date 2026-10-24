<?php
declare(strict_types=1);

namespace PayOauth;

/**
 * 通用工具：编码、随机、Cookie、重定向、请求解析。
 * 与业务无关，保持无副作用（除 Cookie/响应输出外）。
 */
final class Support
{
    public static function base64urlEncode(string $raw): string
    {
        return rtrim(strtr(base64_encode($raw), '+/', '-_'), '=');
    }

    public static function base64urlDecode(string $enc): string
    {
        $rest = strlen($enc) % 4;
        if ($rest > 0) {
            $enc .= str_repeat('=', 4 - $rest);
        }
        $raw = base64_decode(strtr($enc, '-_', '+/'), true);
        return $raw === false ? '' : $raw;
    }

    public static function randomHex(int $bytes): string
    {
        return bin2hex(random_bytes($bytes));
    }

    /** 与服务端既有 token 观感一致：16 位大小写字母数字 + 20 位时间戳数字 = 36 位 */
    public static function tokenLikeLsGo(): string
    {
        $alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        $prefix = '';
        for ($i = 0; $i < 16; $i++) {
            $prefix .= $alphabet[random_int(0, strlen($alphabet) - 1)];
        }
        return $prefix . sprintf('%020d', (int) floor(microtime(true) * 1e6));
    }

    public static function randomCode(int $len, string $alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'): string
    {
        $out = '';
        for ($i = 0; $i < $len; $i++) {
            $out .= $alphabet[random_int(0, strlen($alphabet) - 1)];
        }
        return $out;
    }

    /** 只允许站内相对路径，阻止 //evil.com、/\evil.com、CRLF 注入 */
    public static function isSafeLocalRedirect(string $path): bool
    {
        if ($path === '' || strlen($path) > 512) {
            return false;
        }
        if ($path[0] !== '/') {
            return false;
        }
        if (str_starts_with($path, '//') || str_starts_with($path, '/\\')) {
            return false;
        }
        if (preg_match('/[\x00-\x1f\x7f]/', $path) === 1) {
            return false;
        }
        return true;
    }

    public static function requestPath(): string
    {
        $uri = $_SERVER['REQUEST_URI'] ?? '/';
        $path = parse_url($uri, PHP_URL_PATH);
        return is_string($path) && $path !== '' ? $path : '/';
    }

    public static function currentHost(): string
    {
        $host = $_SERVER['HTTP_HOST'] ?? '';
        return preg_match('/^[A-Za-z0-9.\-:\[\]]{1,120}$/', $host) === 1 ? $host : '';
    }

    public static function isHttps(): bool
    {
        if (!empty($_SERVER['HTTPS']) && strtolower((string) $_SERVER['HTTPS']) !== 'off') {
            return true;
        }
        $proto = strtolower((string) ($_SERVER['HTTP_X_FORWARDED_PROTO'] ?? ''));
        return $proto === 'https';
    }

    public static function clientIp(): string
    {
        $candidates = [];
        if (!empty($_SERVER['HTTP_X_REAL_IP'])) {
            $candidates[] = (string) $_SERVER['HTTP_X_REAL_IP'];
        }
        if (!empty($_SERVER['HTTP_X_FORWARDED_FOR'])) {
            $candidates[] = trim(explode(',', (string) $_SERVER['HTTP_X_FORWARDED_FOR'])[0]);
        }
        $candidates[] = (string) ($_SERVER['REMOTE_ADDR'] ?? '');
        foreach ($candidates as $candidate) {
            if ($candidate !== '' && filter_var($candidate, FILTER_VALIDATE_IP) !== false) {
                return $candidate;
            }
        }
        return '';
    }

    public static function cookieGet(string $name): string
    {
        $value = $_COOKIE[$name] ?? '';
        return is_string($value) ? $value : '';
    }

    public static function cookieSet(
        string $name,
        string $value,
        int $ttl,
        string $path = '/',
        bool $httpOnly = true,
        string $domain = ''
    ): void {
        setcookie($name, $value, [
            'expires'  => time() + $ttl,
            'path'     => $path,
            'domain'   => $domain,
            'secure'   => self::isHttps(),
            'httponly' => $httpOnly,
            'samesite' => 'Lax',
        ]);
    }

    public static function cookieClear(string $name, string $path = '/', string $domain = ''): void
    {
        setcookie($name, '', [
            'expires'  => time() - 3600,
            'path'     => $path,
            'domain'   => $domain,
            'secure'   => self::isHttps(),
            'httponly' => true,
            'samesite' => 'Lax',
        ]);
    }

    public static function securityHeaders(): void
    {
        header('X-Content-Type-Options: nosniff');
        header('X-Frame-Options: DENY');
        header('Referrer-Policy: no-referrer');
        header('Cache-Control: no-store, no-cache, must-revalidate, private');
        header('Pragma: no-cache');
    }

    public static function redirect(string $location, int $code = 302): void
    {
        header('Location: ' . $location, true, $code);
        exit;
    }

    public static function json(array $payload, int $status = 200): void
    {
        http_response_code($status);
        header('Content-Type: application/json; charset=utf-8');
        echo json_encode($payload, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);
        exit;
    }

    public static function esc(string $value): string
    {
        return htmlspecialchars($value, ENT_QUOTES | ENT_SUBSTITUTE, 'UTF-8');
    }

    public static function nowUtc(): string
    {
        return gmdate('Y-m-d H:i:s');
    }

    public static function log(string $message): void
    {
        $dir = dirname(__DIR__) . '/var';
        if (!is_dir($dir)) {
            @mkdir($dir, 0700, true);
        }
        @file_put_contents(
            $dir . '/oauth2.log',
            sprintf("[%s] %s\n", date('c'), $message),
            FILE_APPEND | LOCK_EX
        );
    }
}
