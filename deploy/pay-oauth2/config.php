<?php
declare(strict_types=1);

/**
 * pay-oauth2 配置
 *
 * 部署后最少要改四项：enabled / oauth.issuer / oauth.client_id / oauth.client_secret。
 * 支持任意标准 OAuth 2.0 / OIDC 提供方（Logto、Authentik、Keycloak、Auth0、Casdoor、
 * Google Workspace、GitHub(需 OAuth App + 手填端点)、自建 new-api 前端 SSO 网关等）。
 */
return [
    // 总开关：false 时 /oauth2/login 直接返回「未配置」，不影响原有账号密码登录
    'enabled'   => false,
    'base_path' => '/oauth2',

    'pay' => [
        // 直接复用 pay 主程序的数据库配置，无需重复填写
        'config_file'         => '/www/wwwroot/pay/config/config.yaml',
        // ls-go 用户中心会话 Cookie（实测：值为 ls_user.token）
        'cookie_name'         => 'front_token',
        // 登录有效期取自 pay 后台配置项，读不到时用 default_login_hours
        'login_hours_key'     => 'user_login_token_hours',
        'default_login_hours' => 168,
        // 需要跨子域共享会话时填 .example.com，否则留空（host-only，最安全）
        'cookie_domain'       => '',
    ],

    'oauth' => [
        // IdP 的 issuer（例如 https://sso.example.com/、https://accounts.google.com）
        'issuer'    => '',
        'discovery' => true,          // true 则自动读取 {issuer}/.well-known/openid-configuration

        // discovery 取不到（或 IdP 非 OIDC）时手工填写
        'authorization_endpoint' => '',
        'token_endpoint'         => '',
        'userinfo_endpoint'      => '',
        'jwks_uri'               => '',
        'end_session_endpoint'   => '',

        'client_id'     => '',
        'client_secret' => '',
        // client_secret_post（默认，推荐）或 client_secret_basic
        'token_auth_method' => 'client_secret_post',

        'scopes' => ['openid', 'profile', 'email'],
        // 必须与 IdP 后台登记的回调地址完全一致；留空则按当前域名自动推导
        'redirect_uri' => '',

        // true：校验 id_token 的签名与 iss/aud/exp/nonce（OIDC 强烈建议开启）
        // false：仅信任 userinfo 接口（非 OIDC 的纯 OAuth2 提供方只能这样）
        'verify_id_token' => true,

        // 字段映射：不同 IdP 的 claim 名不一样，按需改这里
        'claims' => [
            'subject'  => 'sub',
            'username' => 'preferred_username',
            'email'    => 'email',
            'name'     => 'name',
        ],
    ],

    'account' => [
        // 允许 SSO 首次登录自动创建本地账号
        'auto_register'   => true,
        // 是否允许按「已验证邮箱」自动关联到已有本地账号（存在邮箱接管风险，默认关闭）
        'link_by_email'   => false,
        // 新建账号是否默认冻结（人工审核后再解冻）
        'freeze_new_user' => false,
        // 新账号的上级用户 ID（0 = 无上级）
        'superior_id'     => 0,
        'username_max'    => 24,
    ],

    'security' => [
        'state_ttl'             => 600,   // 授权事务有效期（秒）
        'clock_skew'            => 120,   // id_token 时间校验容差（秒）
        'rate_limit_per_minute' => 20,    // 单 IP 每分钟发起/回调次数上限
        // 登录成功后允许跳转的站内路径前缀（防止开放重定向）
        'redirect_allow_prefixes' => ['/user', '/console', '/'],
    ],

    'ui' => [
        'button_text' => '使用 SSO 一键登录',
        'show_button' => true,
    ],
];
