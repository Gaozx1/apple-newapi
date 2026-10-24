<?php
declare(strict_types=1);

namespace PayOauth;

/**
 * 配置读取：
 *  - 本插件自己的 config.php（IdP、账号策略、Cookie 策略）
 *  - pay 主程序的 config/config.yaml（复用其数据库凭据，避免二次维护）
 */
final class Config
{
    private array $data;

    private function __construct(array $data)
    {
        $this->data = $data;
    }

    public static function load(string $file, string $rootDir): self
    {
        $defaults = [
            'enabled'    => false,
            'base_path'  => '/oauth2',
            'pay' => [
                'config_file'        => '/www/wwwroot/pay/config/config.yaml',
                'cookie_name'        => 'front_token',
                'login_hours_key'    => 'user_login_token_hours',
                'default_login_hours' => 168,
                'cookie_domain'      => '',
            ],
            'oauth' => [
                'issuer'               => '',
                'discovery'            => true,
                'authorization_endpoint' => '',
                'token_endpoint'       => '',
                'userinfo_endpoint'    => '',
                'jwks_uri'             => '',
                'end_session_endpoint' => '',
                'client_id'            => '',
                'client_secret'        => '',
                'token_auth_method'    => 'client_secret_post',
                'scopes'               => ['openid', 'profile', 'email'],
                'redirect_uri'         => '',
                'verify_id_token'      => true,
                'claims' => [
                    'subject'  => 'sub',
                    'username' => 'preferred_username',
                    'email'    => 'email',
                    'name'     => 'name',
                ],
            ],
            'account' => [
                'auto_register'    => true,
                'link_by_email'    => false,
                'freeze_new_user'  => false,
                'superior_id'      => 0,
                'username_max'     => 24,
            ],
            'security' => [
                'state_ttl'               => 600,
                'clock_skew'              => 120,
                'rate_limit_per_minute'   => 20,
                'redirect_allow_prefixes' => ['/user', '/console', '/'],
            ],
            'ui' => [
                'button_text' => '使用 SSO 一键登录',
                'show_button' => true,
            ],
        ];

        $user = is_file($file) ? require $file : [];
        if (!is_array($user)) {
            $user = [];
        }
        return new self(self::merge($defaults, $user) + ['root_dir' => $rootDir]);
    }

    private static function merge(array $base, array $override): array
    {
        foreach ($override as $key => $value) {
            if (is_array($value) && isset($base[$key]) && is_array($base[$key])) {
                $base[$key] = self::merge($base[$key], $value);
                continue;
            }
            $base[$key] = $value;
        }
        return $base;
    }

    public function get(string $path, mixed $default = null): mixed
    {
        $node = $this->data;
        foreach (explode('.', $path) as $segment) {
            if (!is_array($node) || !array_key_exists($segment, $node)) {
                return $default;
            }
            $node = $node[$segment];
        }
        return $node;
    }

    public function rootDir(): string
    {
        return (string) $this->get('root_dir', sys_get_temp_dir());
    }

    public function isConfigured(): bool
    {
        return $this->get('enabled', false) === true
            && (string) $this->get('oauth.client_id', '') !== ''
            && (string) $this->get('oauth.issuer', '') !== '';
    }

    /**
     * 极简 YAML 读取：pay 的 config.yaml 只有两层「section: key: value」结构，
     * 不需要引入 symfony/yaml 之类的依赖。
     *
     * @return array<string,array<string,string>>
     */
    public static function readSimpleYaml(string $file): array
    {
        if (!is_file($file) || !is_readable($file)) {
            throw new \RuntimeException('无法读取 pay 配置文件: ' . $file);
        }
        $out = [];
        $section = '';
        foreach (file($file, FILE_IGNORE_NEW_LINES) ?: [] as $line) {
            $trimmed = trim($line);
            if ($trimmed === '' || str_starts_with($trimmed, '#')) {
                continue;
            }
            if (preg_match('/^([A-Za-z0-9_.\-]+):\s*(.*)$/', $line, $m) === 1) {
                $section = $m[1];
                $out[$section] = $out[$section] ?? [];
                if (trim($m[2]) !== '') {
                    $out[$section]['__value'] = self::scalar($m[2]);
                }
                continue;
            }
            if ($section !== '' && preg_match('/^\s+([A-Za-z0-9_.\-]+):\s*(.*)$/', $line, $m) === 1) {
                $out[$section][$m[1]] = self::scalar($m[2]);
            }
        }
        return $out;
    }

    private static function scalar(string $raw): string
    {
        $value = trim($raw);
        if ($value !== '' && ($value[0] === '"' || $value[0] === "'")) {
            $quote = $value[0];
            $end = strrpos($value, $quote);
            if ($end !== false && $end > 0) {
                return substr($value, 1, $end - 1);
            }
        }
        $hash = strpos($value, ' #');
        if ($hash !== false) {
            $value = substr($value, 0, $hash);
        }
        return trim($value);
    }
}
