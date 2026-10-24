<?php
declare(strict_types=1);

/**
 * 端到端自测配置（对接本地 mock IdP）。验证完成后用 config.php.prod 还原，生产不要保留本文件。
 */
return [
    'enabled'   => true,
    'base_path' => '/oauth2',

    'pay' => [
        'config_file'         => '/www/wwwroot/pay/config/config.yaml',
        'cookie_name'         => 'front_token',
        'login_hours_key'     => 'user_login_token_hours',
        'default_login_hours' => 168,
        'cookie_domain'       => '',
    ],

    'oauth' => [
        'issuer'    => 'http://127.0.0.1:19900',
        'discovery' => true,
        'client_id' => 'demo-client',
        'client_secret' => 'demo-secret',
        'token_auth_method' => 'client_secret_post',
        'scopes' => ['openid', 'profile', 'email'],
        'redirect_uri' => 'http://pay.mcapple.top/oauth2/callback',
        'verify_id_token' => true,
        'claims' => [
            'subject'  => 'sub',
            'username' => 'preferred_username',
            'email'    => 'email',
            'name'     => 'name',
        ],
    ],

    'account' => [
        'auto_register'   => true,
        'link_by_email'   => false,
        'freeze_new_user' => false,
        'superior_id'     => 0,
        'username_max'    => 24,
    ],

    'security' => [
        'state_ttl'               => 600,
        'clock_skew'              => 120,
        'rate_limit_per_minute'   => 60,
        'redirect_allow_prefixes' => ['/user', '/console', '/'],
    ],

    'ui' => [
        'button_text' => '使用 SSO 一键登录',
        'show_button' => true,
    ],
];
