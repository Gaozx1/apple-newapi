<?php
declare(strict_types=1);

/**
 * 本地最小 OIDC 提供方，仅用于端到端自测（不对外暴露）。
 * 启动：php -S 127.0.0.1:19900 tools/mock-idp.php
 * 授权端点支持 ?login_as=xxx 模拟不同身份。
 */

const MOCK_DIR = '/tmp/pay-oauth2-mock';
const CLIENT_ID = 'demo-client';
const CLIENT_SECRET = 'demo-secret';
const ISSUER = 'http://127.0.0.1:19900';

function b64u(string $raw): string
{
    return rtrim(strtr(base64_encode($raw), '+/', '-_'), '=');
}

function b64uDecode(string $enc): string
{
    $rest = strlen($enc) % 4;
    if ($rest > 0) {
        $enc .= str_repeat('=', 4 - $rest);
    }
    return (string) base64_decode(strtr($enc, '-_', '+/'));
}

function ensureDir(): void
{
    if (!is_dir(MOCK_DIR . '/codes')) {
        mkdir(MOCK_DIR . '/codes', 0700, true);
    }
}

function jsonOut(array $payload, int $status = 200): void
{
    http_response_code($status);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode($payload, JSON_UNESCAPED_SLASHES);
    exit;
}

function signIdToken(array $claims): string
{
    $header = b64u(json_encode(['alg' => 'HS256', 'typ' => 'JWT', 'kid' => 'mock']));
    $payload = b64u(json_encode($claims, JSON_UNESCAPED_SLASHES));
    $sig = b64u(hash_hmac('sha256', $header . '.' . $payload, CLIENT_SECRET, true));
    return $header . '.' . $payload . '.' . $sig;
}

$path = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH) ?: '/';
$query = [];
parse_str((string) parse_url($_SERVER['REQUEST_URI'] ?? '', PHP_URL_QUERY), $query);
ensureDir();

switch ($path) {
    case '/.well-known/openid-configuration':
        jsonOut([
            'issuer'                 => ISSUER,
            'authorization_endpoint' => ISSUER . '/authorize',
            'token_endpoint'         => ISSUER . '/token',
            'userinfo_endpoint'      => ISSUER . '/userinfo',
            'jwks_uri'               => ISSUER . '/jwks',
            'response_types_supported' => ['code'],
            'subject_types_supported'  => ['public'],
            'id_token_signing_alg_values_supported' => ['HS256'],
        ]);
        break;

    case '/authorize':
        if (($query['client_id'] ?? '') !== CLIENT_ID) {
            jsonOut(['error' => 'invalid_client'], 400);
        }
        if (($query['response_type'] ?? '') !== 'code') {
            jsonOut(['error' => 'unsupported_response_type'], 400);
        }
        $redirect = (string) ($query['redirect_uri'] ?? '');
        if ($redirect === '') {
            jsonOut(['error' => 'invalid_request'], 400);
        }
        $code = bin2hex(random_bytes(16));
        $identity = preg_replace('/[^a-zA-Z0-9_.\-]/', '', (string) ($query['login_as'] ?? 'demo_user')) ?: 'demo_user';
        file_put_contents(MOCK_DIR . '/codes/' . $code . '.json', json_encode([
            'challenge' => (string) ($query['code_challenge'] ?? ''),
            'nonce'     => (string) ($query['nonce'] ?? ''),
            'redirect'  => $redirect,
            'identity'  => $identity,
            'used'      => false,
            'at'        => time(),
        ]), LOCK_EX);
        $sep = str_contains($redirect, '?') ? '&' : '?';
        header('Location: ' . $redirect . $sep . http_build_query([
            'code'  => $code,
            'state' => (string) ($query['state'] ?? ''),
        ]));
        http_response_code(302);
        exit;

    case '/token':
        $code = (string) ($_POST['code'] ?? '');
        $file = MOCK_DIR . '/codes/' . $code . '.json';
        if ($code === '' || !is_file($file)) {
            jsonOut(['error' => 'invalid_grant', 'error_description' => 'unknown code'], 400);
        }
        $stored = json_decode((string) file_get_contents($file), true);
        if (($stored['used'] ?? true) === true) {
            jsonOut(['error' => 'invalid_grant', 'error_description' => 'code already used'], 400);
        }
        if ((string) ($_POST['client_secret'] ?? '') !== CLIENT_SECRET) {
            jsonOut(['error' => 'invalid_client'], 401);
        }
        $verifier = (string) ($_POST['code_verifier'] ?? '');
        $expected = b64u(hash('sha256', $verifier, true));
        if ($verifier === '' || !hash_equals((string) $stored['challenge'], $expected)) {
            jsonOut(['error' => 'invalid_grant', 'error_description' => 'pkce mismatch'], 400);
        }
        $stored['used'] = true;
        file_put_contents($file, json_encode($stored), LOCK_EX);

        $identity = (string) $stored['identity'];
        $claims = [
            'iss'            => ISSUER,
            'sub'            => 'mock|' . $identity,
            'aud'            => CLIENT_ID,
            'iat'            => time(),
            'exp'            => time() + 600,
            'nonce'          => (string) $stored['nonce'],
            'preferred_username' => $identity,
            'email'          => $identity . '@example.test',
            'email_verified' => true,
            'name'           => 'Mock ' . $identity,
        ];
        $accessToken = bin2hex(random_bytes(16));
        file_put_contents(MOCK_DIR . '/at-' . $accessToken . '.json', json_encode($claims), LOCK_EX);
        jsonOut([
            'access_token' => $accessToken,
            'token_type'   => 'Bearer',
            'expires_in'   => 3600,
            'id_token'     => signIdToken($claims),
            'scope'        => 'openid profile email',
        ]);
        break;

    case '/userinfo':
        $auth = (string) ($_SERVER['HTTP_AUTHORIZATION'] ?? '');
        $token = str_starts_with($auth, 'Bearer ') ? substr($auth, 7) : '';
        $file = MOCK_DIR . '/at-' . preg_replace('/[^a-f0-9]/', '', $token) . '.json';
        if ($token === '' || !is_file($file)) {
            jsonOut(['error' => 'invalid_token'], 401);
        }
        $claims = json_decode((string) file_get_contents($file), true);
        jsonOut([
            'sub'            => $claims['sub'],
            'preferred_username' => $claims['preferred_username'],
            'email'          => $claims['email'],
            'email_verified' => true,
            'name'           => $claims['name'],
        ]);
        break;

    case '/jwks':
        jsonOut(['keys' => []]);
        break;

    default:
        http_response_code(404);
        echo 'mock idp';
}
