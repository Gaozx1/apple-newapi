<?php
declare(strict_types=1);

/**
 * pay-oauth2 —— 给 ls-go（聚合免签系统）用户中心加上 OAuth 2.0 / OIDC 登录。
 *
 * 路由（由 nginx 把 /oauth2/ 全部交给本文件）：
 *   GET /oauth2/login?redirect=/user/   发起授权
 *   GET /oauth2/callback                接收授权码并建立 pay 会话
 *   GET /oauth2/logout                  退出（清 Cookie + 作废服务端 token）
 *   GET /oauth2/health                  健康检查（不含任何密钥）
 *   GET /oauth2/                        说明页
 */

require __DIR__ . '/lib/Support.php';
require __DIR__ . '/lib/Config.php';
require __DIR__ . '/lib/Database.php';
require __DIR__ . '/lib/Audit.php';
require __DIR__ . '/lib/OAuth2Client.php';
require __DIR__ . '/lib/PayAccount.php';

use PayOauth\Audit;
use PayOauth\Config;
use PayOauth\Database;
use PayOauth\OAuth2Client;
use PayOauth\PayAccount;
use PayOauth\Support;

date_default_timezone_set('Asia/Shanghai');
ini_set('display_errors', '0');
ini_set('log_errors', '0');

const TX_COOKIE = 'ls_sso_tx';

$rootDir = __DIR__;
$config = Config::load($rootDir . '/config.php', $rootDir);
$basePath = rtrim((string) $config->get('base_path', '/oauth2'), '/');
$path = Support::requestPath();
$route = '/' . ltrim(substr($path, strlen($basePath)), '/');

try {
    switch ($route) {
        case '/login':
            handleLogin($config, $basePath);
            break;
        case '/callback':
            handleCallback($config, $basePath);
            break;
        case '/logout':
            handleLogout($config);
            break;
        case '/health':
            handleHealth($config);
            break;
        case '/':
        case '':
            handleIndex($config);
            break;
        default:
            if (str_starts_with($route, '/assets/')) {
                serveAsset($route, $config);
            }
            renderError('404', '未知的 SSO 路由', null, $config);
    }
} catch (\Throwable $e) {
    Support::log('unhandled: ' . $e->getMessage() . ' @ ' . $e->getFile() . ':' . $e->getLine());
    renderError('500', '单点登录处理失败，请稍后重试或使用账号密码登录', null, $config);
}

// ---------------------------------------------------------------------------

function handleLogin(Config $config, string $basePath): void
{
    Support::securityHeaders();
    if (!$config->isConfigured()) {
        renderError('503', '本系统尚未配置 OAuth 2.0 登录', null, $config);
    }
    if (!Audit::hit('login', (int) $config->get('security.rate_limit_per_minute', 20))) {
        Audit::record('login_start', 'rate_limited');
        renderError('429', '请求过于频繁，请稍后再试', null, $config);
    }

    $redirect = (string) ($_GET['redirect'] ?? '/user/');
    if (!Support::isSafeLocalRedirect($redirect) || !redirectAllowed($config, $redirect)) {
        $redirect = '/user/';
    }
    $mode = ((string) ($_GET['mode'] ?? 'login')) === 'bind' ? 'bind' : 'login';

    Database::boot($config);
    $account = new PayAccount($config);
    $currentUser = null;
    if ($mode === 'bind') {
        $currentUser = $account->userByToken(Support::cookieGet((string) $config->get('pay.cookie_name', 'front_token')));
        if ($currentUser === null) {
            renderError('400', '绑定需要先登录本地账号', null, $config);
        }
    }

    $state = Support::randomHex(16);
    $nonce = Support::randomHex(16);
    $verifier = Support::base64urlEncode(random_bytes(48));
    $challenge = Support::base64urlEncode(hash('sha256', $verifier, true));

    $tx = [
        'state'    => $state,
        'nonce'    => $nonce,
        'verifier' => $verifier,
        'redirect' => $redirect,
        'mode'     => $mode,
        'uid'      => $currentUser === null ? 0 : (int) $currentUser['id'],
        'exp'      => time() + (int) $config->get('security.state_ttl', 600),
    ];
    Support::cookieSet(
        TX_COOKIE,
        txSeal($tx),
        (int) $config->get('security.state_ttl', 600),
        $basePath === '' ? '/' : $basePath,
        true,
        (string) $config->get('pay.cookie_domain', '')
    );

    $client = new OAuth2Client($config, __DIR__ . '/var');
    $redirectUri = redirectUri($config);
    Audit::record('login_start', 'redirect', ['detail' => $mode]);
    Support::redirect($client->buildAuthorizeUrl($state, $nonce, $challenge, $redirectUri), 302);
}

function handleCallback(Config $config, string $basePath): void
{
    Support::securityHeaders();
    Database::boot($config);
    if (!$config->isConfigured()) {
        renderError('503', '本系统尚未配置 OAuth 2.0 登录', null, $config);
    }
    if (!Audit::hit('callback', (int) $config->get('security.rate_limit_per_minute', 20))) {
        Audit::record('callback', 'rate_limited');
        renderError('429', '请求过于频繁，请稍后再试', null, $config);
    }

    $sealed = Support::cookieGet(TX_COOKIE);
    Support::cookieClear(TX_COOKIE, $basePath === '' ? '/' : $basePath, (string) $config->get('pay.cookie_domain', ''));
    if ($sealed === '') {
        Audit::record('callback', 'missing_state_cookie');
        renderError('400', '登录会话已失效，请重新发起登录', null, $config);
    }
    $tx = txOpen($sealed);
    if ($tx === null || (int) ($tx['exp'] ?? 0) < time()) {
        Audit::record('callback', 'bad_state_cookie');
        renderError('400', '登录会话已失效，请重新发起登录', null, $config);
    }
    $state = (string) ($_GET['state'] ?? '');
    if ($state === '' || !hash_equals((string) $tx['state'], $state)) {
        Audit::record('callback', 'state_mismatch');
        renderError('400', '状态校验失败（可能存在 CSRF），请重新发起登录', null, $config);
    }
    if (isset($_GET['error'])) {
        Audit::record('callback', 'idp_error', ['detail' => (string) $_GET['error']]);
        renderError('401', '身份提供方返回错误：' . (string) $_GET['error'], null, $config);
    }
    $code = (string) ($_GET['code'] ?? '');
    if ($code === '') {
        renderError('400', '缺少授权码', null, $config);
    }

    $client = new OAuth2Client($config, __DIR__ . '/var');
    try {
        $tokens = $client->exchangeCode($code, (string) $tx['verifier'], redirectUri($config));
    } catch (\RuntimeException $e) {
        Audit::record('callback', 'exchange_failed', ['detail' => $e->getMessage()]);
        renderError('401', '授权码交换失败，请重新发起登录', $e->getMessage(), $config);
    }

    $claims = [];
    if (!empty($tokens['id_token']) && (bool) $config->get('oauth.verify_id_token', true)) {
        $claims = $client->verifyIdToken((string) $tokens['id_token'], (string) $tx['nonce']);
    }
    // userinfo 覆盖 id_token（部分 IdP 只在 userinfo 里给出 username/email）
    $userinfo = [];
    if (!empty($tokens['access_token'])) {
        $userinfo = $client->userinfo((string) $tokens['access_token']);
    }
    $merged = array_merge($claims, $userinfo);
    if ($merged === []) {
        throw new \RuntimeException('IdP 未返回 id_token 或 userinfo');
    }

    $profile = extractProfile($config, $merged);
    $account = new PayAccount($config);

    if ((string) $tx['mode'] === 'bind') {
        $currentUser = $account->userByToken(Support::cookieGet((string) $config->get('pay.cookie_name', 'front_token')));
        if ($currentUser === null) {
            renderError('400', '绑定会话已失效，请重新登录后再绑定', null, $config);
        }
        try {
            $account->bind((int) $currentUser['id'], $profile);
        } catch (\RuntimeException $e) {
            Audit::record('bind', 'conflict', ['subject' => $profile['subject'], 'detail' => $e->getMessage()]);
            renderError('409', $e->getMessage(), null, $config);
        }
        Audit::record('bind', 'ok', ['subject' => $profile['subject'], 'user_id' => (int) $currentUser['id']]);
        Support::redirect((string) $tx['redirect'], 302);
    }

    try {
        $resolved = $account->resolve($profile);
        $user = $resolved['user'];
        $account->assertActive($user);
    } catch (\RuntimeException $e) {
        Audit::record('callback', 'account_denied', ['subject' => $profile['subject'], 'detail' => $e->getMessage()]);
        renderError('403', $e->getMessage(), null, $config);
    }
    $hours = $account->loginHours();
    $token = $account->mintToken((int) $user['id'], $hours);
    Support::cookieSet(
        (string) $config->get('pay.cookie_name', 'front_token'),
        $token,
        $hours * 3600,
        '/',
        true,
        (string) $config->get('pay.cookie_domain', '')
    );
    Audit::record('callback', $resolved['created'] ? 'ok_registered' : 'ok_login', [
        'subject' => $profile['subject'],
        'user_id' => (int) $user['id'],
        'detail'  => $resolved['bound'] ? 'linked_by_email' : '',
    ]);
    Support::redirect((string) $tx['redirect'], 302);
}

function handleLogout(Config $config): void
{
    Support::securityHeaders();
    $cookieName = (string) $config->get('pay.cookie_name', 'front_token');
    $domain = (string) $config->get('pay.cookie_domain', '');
    $token = Support::cookieGet($cookieName);
    if ($token !== '') {
        try {
            Database::boot($config);
            $account = new PayAccount($config);
            $user = $account->userByToken($token);
            if ($user !== null) {
                $account->revokeToken((int) $user['id'], $token);
                Audit::record('logout', 'ok', ['user_id' => (int) $user['id']]);
            }
        } catch (\Throwable $e) {
            Support::log('logout revoke failed: ' . $e->getMessage());
        }
    }
    Support::cookieClear($cookieName, '/', $domain);
    Support::cookieClear(TX_COOKIE, '/', $domain);
    Support::redirect('/user/', 302);
}

function handleHealth(Config $config): void
{
    $status = [
        'ok'          => true,
        'configured'  => $config->isConfigured(),
        'php'         => PHP_VERSION,
        'issuer'      => (string) $config->get('oauth.issuer', ''),
        'client_id'   => (string) $config->get('oauth.client_id', ''),
        'database'    => 'unknown',
        'idp'         => 'unknown',
        'redirect_uri' => redirectUri($config),
    ];
    try {
        Database::boot($config);
        Database::fetchOne('SELECT 1 AS ok');
        $status['database'] = 'ok';
    } catch (\Throwable $e) {
        $status['ok'] = false;
        $status['database'] = 'error: ' . $e->getMessage();
    }
    if ($config->isConfigured()) {
        try {
            $client = new OAuth2Client($config, __DIR__ . '/var');
            $client->metadata();
            $status['idp'] = 'ok';
        } catch (\Throwable $e) {
            $status['ok'] = false;
            $status['idp'] = 'error: ' . $e->getMessage();
        }
    }
    Support::json($status, $status['ok'] ? 200 : 503);
}

/** 静态资源（含动态生成的 config.js）：由 PHP 直接吐出，白名单 + 正确 MIME */
function serveAsset(string $route, Config $config): void
{
    $name = substr($route, strlen('/assets/'));
    if (preg_match('/^[A-Za-z0-9._-]{1,64}$/', $name) !== 1) {
        renderError('404', '资源不存在', null, $config);
    }
    if ($name === 'config.js') {
        header('Content-Type: application/javascript; charset=utf-8');
        header('Cache-Control: no-store');
        header('X-Content-Type-Options: nosniff');
        echo 'window.__LS_SSO__ = ' . json_encode([
            'enabled' => $config->isConfigured() && (bool) $config->get('ui.show_button', true),
            'login'   => rtrim((string) $config->get('base_path', '/oauth2'), '/') . '/login',
            'text'    => (string) $config->get('ui.button_text', '使用 SSO 一键登录'),
        ], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . ';';
        exit;
    }
    $file = __DIR__ . '/assets/' . $name;
    if (!is_file($file)) {
        renderError('404', '资源不存在', null, $config);
    }
    $mime = match (strtolower(pathinfo($name, PATHINFO_EXTENSION))) {
        'js'   => 'application/javascript; charset=utf-8',
        'css'  => 'text/css; charset=utf-8',
        'svg'  => 'image/svg+xml',
        'png'  => 'image/png',
        'ico'  => 'image/x-icon',
        default => 'application/octet-stream',
    };
    header('Content-Type: ' . $mime);
    header('Cache-Control: public, max-age=600');
    header('X-Content-Type-Options: nosniff');
    readfile($file);
    exit;
}

function handleIndex(Config $config): void
{
    Support::securityHeaders();
    header('Content-Type: text/html; charset=utf-8');
    $configured = $config->isConfigured();
    echo '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>SSO</title>'
        . '<body style="font-family:system-ui;max-width:560px;margin:80px auto;line-height:1.7">'
        . '<h2>OAuth 2.0 单点登录</h2>'
        . '<p>状态：' . ($configured ? '已配置' : '未配置（请在 config.php 填写 IdP 参数）') . '</p>'
        . '<p><a href="' . Support::esc(rtrim((string) $config->get('base_path', '/oauth2'), '/')) . '/login?redirect=/user/">使用 SSO 登录</a></p>'
        . '<p><a href="' . Support::esc(rtrim((string) $config->get('base_path', '/oauth2'), '/')) . '/health">健康检查</a></p>'
        . '</body></html>';
}

// ---------------------------------------------------------------------------

function extractProfile(Config $config, array $claims): array
{
    $map = (array) $config->get('oauth.claims', []);
    $subject = (string) ($claims[$map['subject'] ?? 'sub'] ?? '');
    if ($subject === '') {
        foreach (['sub', 'id', 'user_id', 'uuid', 'preferred_username', 'username', 'email'] as $fallback) {
            if (!empty($claims[$fallback])) {
                $subject = (string) $claims[$fallback];
                break;
            }
        }
    }
    if ($subject === '') {
        throw new \RuntimeException('IdP 响应中缺少可用于标识用户的 sub/username');
    }
    if (strlen($subject) > 191) {
        $subject = substr(hash('sha256', $subject), 0, 48);
    }
    $email = (string) ($claims[$map['email'] ?? 'email'] ?? '');
    $verified = $claims['email_verified'] ?? false;
    $verified = ($verified === true || $verified === 1 || $verified === '1' || $verified === 'true') ? 1 : 0;
    return [
        'subject'        => $subject,
        'username'       => (string) ($claims[$map['username'] ?? 'preferred_username'] ?? ''),
        'email'          => str_contains($email, '@') ? substr($email, 0, 191) : '',
        'email_verified' => $verified,
        'name'           => substr((string) ($claims[$map['name'] ?? 'name'] ?? ''), 0, 100),
    ];
}

function redirectUri(Config $config): string
{
    $configured = (string) $config->get('oauth.redirect_uri', '');
    if ($configured !== '') {
        return $configured;
    }
    $scheme = Support::isHttps() ? 'https' : 'http';
    return $scheme . '://' . Support::currentHost() . rtrim((string) $config->get('base_path', '/oauth2'), '/') . '/callback';
}

function redirectAllowed(Config $config, string $target): bool
{
    $path = parse_url($target, PHP_URL_PATH) ?: '/';
    foreach ((array) $config->get('security.redirect_allow_prefixes', ['/']) as $prefix) {
        $prefix = (string) $prefix;
        if ($prefix === '/' || $path === $prefix || str_starts_with($path, rtrim($prefix, '/') . '/')) {
            return true;
        }
    }
    return false;
}

function txSeal(array $tx): string
{
    $payload = Support::base64urlEncode(json_encode($tx, JSON_UNESCAPED_SLASHES));
    $sig = Support::base64urlEncode(hash_hmac('sha256', $payload, txSecret(), true));
    return $payload . '.' . $sig;
}

function txOpen(string $sealed): ?array
{
    $parts = explode('.', $sealed);
    if (count($parts) !== 2) {
        return null;
    }
    $expected = Support::base64urlEncode(hash_hmac('sha256', $parts[0], txSecret(), true));
    if (!hash_equals($expected, $parts[1])) {
        return null;
    }
    $tx = json_decode(Support::base64urlDecode($parts[0]), true);
    return is_array($tx) ? $tx : null;
}

/** 事务 Cookie 的签名密钥：首次运行自动生成，权限 0600 */
function txSecret(): string
{
    static $secret = null;
    if (is_string($secret)) {
        return $secret;
    }
    $dir = __DIR__ . '/var';
    if (!is_dir($dir)) {
        @mkdir($dir, 0700, true);
    }
    $file = $dir . '/secret.key';
    if (!is_file($file)) {
        $raw = random_bytes(32);
        @file_put_contents($file, bin2hex($raw), LOCK_EX);
        @chmod($file, 0600);
    }
    $secret = trim((string) @file_get_contents($file));
    if ($secret === '') {
        throw new \RuntimeException('无法初始化签名密钥（检查 var/ 目录权限）');
    }
    return $secret;
}

function renderError(string $code, string $message, ?string $detail, Config $config): void
{
    Support::securityHeaders();
    http_response_code((int) $code >= 400 ? (int) $code : 400);
    header('Content-Type: text/html; charset=utf-8');
    $back = rtrim((string) $config->get('base_path', '/oauth2'), '/');
    echo '<!doctype html><html lang="zh-CN"><meta charset="utf-8">'
        . '<meta name="viewport" content="width=device-width,initial-scale=1">'
        . '<title>SSO 登录</title>'
        . '<body style="margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;'
        . 'background:#f5f7fb;font-family:system-ui,-apple-system,\'Segoe UI\',sans-serif;color:#1f2937">'
        . '<div style="background:#fff;border-radius:14px;padding:32px 36px;max-width:520px;'
        . 'box-shadow:0 12px 32px rgba(15,23,42,.08)">'
        . '<div style="font-size:13px;letter-spacing:.08em;color:#94a3b8;text-transform:uppercase">SSO'
        . ' · ' . Support::esc($code) . '</div>'
        . '<h2 style="margin:8px 0 12px;font-size:20px">' . Support::esc($message) . '</h2>'
        . ($detail === null || $detail === ''
            ? ''
            : '<p style="color:#64748b;font-size:13px;word-break:break-all">' . Support::esc($detail) . '</p>')
        . '<div style="margin-top:20px;display:flex;gap:12px;flex-wrap:wrap">'
        . '<a href="' . Support::esc($back) . '/login?redirect=/user/" style="text-decoration:none;'
        . 'background:#2563eb;color:#fff;padding:10px 18px;border-radius:9px;font-size:14px">重新登录</a>'
        . '<a href="/user/" style="text-decoration:none;border:1px solid #cbd5e1;color:#334155;'
        . 'padding:10px 18px;border-radius:9px;font-size:14px">返回用户中心</a>'
        . '</div></div></body></html>';
    exit;
}
