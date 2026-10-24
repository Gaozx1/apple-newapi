<?php
declare(strict_types=1);

namespace PayOauth;

use RuntimeException;

/**
 * 通用 OAuth 2.0 / OpenID Connect 客户端。
 *
 * - 授权码 + PKCE(S256)，机密客户端同时使用 client_secret
 * - 支持 discovery 或手工端点
 * - id_token 校验：alg 白名单、JWKS 签名、iss/aud/exp/nbf/nonce
 * - 无 id_token 时回退 userinfo
 */
final class OAuth2Client
{
    private const SUPPORTED_ALGS = ['RS256', 'ES256', 'HS256'];

    private Config $config;
    private string $cacheDir;
    private array $meta = [];

    public function __construct(Config $config, string $cacheDir)
    {
        $this->config = $config;
        $this->cacheDir = rtrim($cacheDir, '/');
        if (!is_dir($this->cacheDir)) {
            @mkdir($this->cacheDir, 0700, true);
        }
    }

    public function metadata(): array
    {
        if ($this->meta !== []) {
            return $this->meta;
        }
        $issuer = rtrim((string) $this->config->get('oauth.issuer', ''), '/');
        $manual = [
            'issuer'                 => $issuer,
            'authorization_endpoint' => (string) $this->config->get('oauth.authorization_endpoint', ''),
            'token_endpoint'         => (string) $this->config->get('oauth.token_endpoint', ''),
            'userinfo_endpoint'      => (string) $this->config->get('oauth.userinfo_endpoint', ''),
            'jwks_uri'               => (string) $this->config->get('oauth.jwks_uri', ''),
            'end_session_endpoint'   => (string) $this->config->get('oauth.end_session_endpoint', ''),
        ];
        $useDiscovery = (bool) $this->config->get('oauth.discovery', true);
        if ($useDiscovery && $issuer !== '') {
            $meta = $this->discover($issuer);
            foreach ($manual as $key => $value) {
                if ($value !== '') {
                    $meta[$key] = $value;
                }
            }
        } else {
            $meta = $manual;
        }
        foreach (['authorization_endpoint', 'token_endpoint'] as $required) {
            if (($meta[$required] ?? '') === '') {
                throw new RuntimeException('缺少 ' . $required . '，请检查 oauth 配置或 IdP discovery');
            }
        }
        $meta['issuer'] = rtrim((string) ($meta['issuer'] ?: $issuer), '/');
        $this->meta = $meta;
        return $meta;
    }

    private function discover(string $issuer): array
    {
        $cacheFile = $this->cacheDir . '/oidc-metadata.json';
        $cacheKey = hash('sha256', $issuer);
        if (is_file($cacheFile)) {
            $cached = json_decode((string) file_get_contents($cacheFile), true);
            if (is_array($cached) && ($cached['_key'] ?? '') === $cacheKey && (int) ($cached['_at'] ?? 0) > time() - 3600) {
                return $cached['meta'];
            }
        }
        $url = $issuer . '/.well-known/openid-configuration';
        $result = $this->httpGetJson($url);
        if (!is_array($result) || !isset($result['authorization_endpoint'])) {
            throw new RuntimeException('IdP discovery 失败: ' . $url);
        }
        @file_put_contents(
            $cacheFile,
            json_encode(['_key' => $cacheKey, '_at' => time(), 'meta' => $result], JSON_UNESCAPED_SLASHES),
            LOCK_EX
        );
        @chmod($cacheFile, 0600);
        return $result;
    }

    public function buildAuthorizeUrl(string $state, string $nonce, string $challenge, string $redirectUri): string
    {
        $meta = $this->metadata();
        $params = [
            'response_type'         => 'code',
            'client_id'             => (string) $this->config->get('oauth.client_id'),
            'redirect_uri'          => $redirectUri,
            'scope'                 => implode(' ', (array) $this->config->get('oauth.scopes', ['openid'])),
            'state'                 => $state,
            'nonce'                 => $nonce,
            'code_challenge'        => $challenge,
            'code_challenge_method' => 'S256',
        ];
        $endpoint = (string) $meta['authorization_endpoint'];
        $separator = str_contains($endpoint, '?') ? '&' : '?';
        return $endpoint . $separator . http_build_query($params, '', '&', PHP_QUERY_RFC3986);
    }

    public function exchangeCode(string $code, string $verifier, string $redirectUri): array
    {
        $meta = $this->metadata();
        $method = (string) $this->config->get('oauth.token_auth_method', 'client_secret_post');
        $body = [
            'grant_type'    => 'authorization_code',
            'code'          => $code,
            'redirect_uri'  => $redirectUri,
            'client_id'     => (string) $this->config->get('oauth.client_id'),
            'code_verifier' => $verifier,
        ];
        $headers = ['Accept: application/json'];
        $basicUser = null;
        $secret = (string) $this->config->get('oauth.client_secret', '');
        if ($secret !== '') {
            if ($method === 'client_secret_basic') {
                $basicUser = rawurlencode((string) $this->config->get('oauth.client_id')) . ':' . rawurlencode($secret);
            } else {
                $body['client_secret'] = $secret;
            }
        }
        $response = $this->httpPostForm((string) $meta['token_endpoint'], $body, $headers, $basicUser);
        if (!is_array($response)) {
            throw new RuntimeException('token 端点返回非 JSON');
        }
        if (isset($response['error'])) {
            throw new RuntimeException('token 端点错误: ' . (string) $response['error']);
        }
        if (empty($response['access_token']) && empty($response['id_token'])) {
            throw new RuntimeException('token 端点未返回 access_token/id_token');
        }
        return $response;
    }

    public function userinfo(string $accessToken): array
    {
        $meta = $this->metadata();
        $endpoint = (string) ($meta['userinfo_endpoint'] ?? '');
        if ($endpoint === '' || $accessToken === '') {
            return [];
        }
        $result = $this->httpGetJson($endpoint, ['Authorization: Bearer ' . $accessToken]);
        return is_array($result) ? $result : [];
    }

    /**
     * 校验 id_token 并返回 claims。
     * 不通过校验时抛异常，绝不降级为「信任」。
     */
    public function verifyIdToken(string $jwt, string $expectedNonce): array
    {
        $parts = explode('.', $jwt);
        if (count($parts) !== 3) {
            throw new RuntimeException('id_token 结构非法');
        }
        [$headerB64, $payloadB64, $sigB64] = $parts;
        $header = json_decode(Support::base64urlDecode($headerB64), true);
        $claims = json_decode(Support::base64urlDecode($payloadB64), true);
        if (!is_array($header) || !is_array($claims)) {
            throw new RuntimeException('id_token 不是合法 JSON');
        }
        $alg = (string) ($header['alg'] ?? '');
        if (!in_array($alg, self::SUPPORTED_ALGS, true)) {
            throw new RuntimeException('id_token 使用不支持的算法: ' . $alg);
        }
        $signingInput = $headerB64 . '.' . $payloadB64;
        $signature = Support::base64urlDecode($sigB64);

        if ($alg === 'HS256') {
            $secret = (string) $this->config->get('oauth.client_secret', '');
            if ($secret === '') {
                throw new RuntimeException('HS256 需要 client_secret');
            }
            $expected = hash_hmac('sha256', $signingInput, $secret, true);
            if (!hash_equals($expected, $signature)) {
                throw new RuntimeException('id_token 签名不匹配');
            }
        } else {
            $kid = (string) ($header['kid'] ?? '');
            $pem = $this->publicKeyPem($kid);
            if ($pem === '') {
                throw new RuntimeException('JWKS 中找不到签名公钥' . ($kid === '' ? '' : ' kid=' . $kid));
            }
            $derSignature = $alg === 'ES256' ? $this->ecdsaRawToDer($signature) : $signature;
            $ok = openssl_verify($signingInput, $derSignature, $pem, OPENSSL_ALGO_SHA256);
            if ($ok !== 1) {
                throw new RuntimeException('id_token 签名校验失败');
            }
        }

        $skew = (int) $this->config->get('security.clock_skew', 120);
        $now = time();
        $issuer = rtrim((string) $this->metadata()['issuer'], '/');
        if ($issuer !== '' && rtrim((string) ($claims['iss'] ?? ''), '/') !== $issuer) {
            throw new RuntimeException('id_token iss 不匹配');
        }
        $audiences = $claims['aud'] ?? [];
        $audiences = is_array($audiences) ? $audiences : [$audiences];
        if (!in_array((string) $this->config->get('oauth.client_id'), array_map('strval', $audiences), true)) {
            throw new RuntimeException('id_token aud 不匹配');
        }
        if (isset($claims['exp']) && (int) $claims['exp'] + $skew < $now) {
            throw new RuntimeException('id_token 已过期');
        }
        if (isset($claims['nbf']) && (int) $claims['nbf'] - $skew > $now) {
            throw new RuntimeException('id_token 尚未生效');
        }
        if ($expectedNonce !== '' && !hash_equals($expectedNonce, (string) ($claims['nonce'] ?? ''))) {
            throw new RuntimeException('id_token nonce 不匹配');
        }
        return $claims;
    }

    private function publicKeyPem(string $kid): string
    {
        $meta = $this->metadata();
        $jwksUri = (string) ($meta['jwks_uri'] ?? '');
        if ($jwksUri === '') {
            return '';
        }
        $jwks = $this->jwks($jwksUri);
        foreach ((array) ($jwks['keys'] ?? []) as $jwk) {
            if (!is_array($jwk)) {
                continue;
            }
            if ($kid !== '' && (string) ($jwk['kid'] ?? '') !== $kid) {
                continue;
            }
            if (($jwk['kty'] ?? '') === 'RSA') {
                return $this->rsaPem((string) $jwk['n'], (string) $jwk['e']);
            }
            if (($jwk['kty'] ?? '') === 'EC' && ($jwk['crv'] ?? '') === 'P-256') {
                return $this->ecPem((string) $jwk['x'], (string) $jwk['y']);
            }
        }
        return '';
    }

    private function jwks(string $uri): array
    {
        $cacheFile = $this->cacheDir . '/jwks-' . substr(hash('sha256', $uri), 0, 16) . '.json';
        if (is_file($cacheFile)) {
            $cached = json_decode((string) file_get_contents($cacheFile), true);
            if (is_array($cached) && (int) ($cached['_at'] ?? 0) > time() - 1800) {
                return $cached['keys'] ?? [];
            }
        }
        $result = $this->httpGetJson($uri);
        if (!is_array($result)) {
            throw new RuntimeException('JWKS 获取失败');
        }
        @file_put_contents(
            $cacheFile,
            json_encode(['_at' => time(), 'keys' => $result], JSON_UNESCAPED_SLASHES),
            LOCK_EX
        );
        @chmod($cacheFile, 0600);
        return $result;
    }

    private function httpGetJson(string $url, array $headers = []): ?array
    {
        $ch = curl_init($url);
        curl_setopt_array($ch, [
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_FOLLOWLOCATION => false,
            CURLOPT_CONNECTTIMEOUT => 5,
            CURLOPT_TIMEOUT        => 10,
            CURLOPT_SSL_VERIFYPEER => true,
            CURLOPT_SSL_VERIFYHOST => 2,
            CURLOPT_HTTPHEADER     => array_merge(['Accept: application/json'], $headers),
            CURLOPT_USERAGENT      => 'pay-oauth2/1.0',
        ]);
        $body = curl_exec($ch);
        $status = (int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
        $error = curl_error($ch);
        curl_close($ch);
        if ($body === false) {
            throw new RuntimeException('请求失败: ' . $error);
        }
        if ($status >= 400) {
            throw new RuntimeException('HTTP ' . $status . ' from ' . parse_url($url, PHP_URL_HOST));
        }
        $decoded = json_decode((string) $body, true);
        return is_array($decoded) ? $decoded : null;
    }

    private function httpPostForm(string $url, array $body, array $headers, ?string $basicUser): ?array
    {
        $ch = curl_init($url);
        curl_setopt_array($ch, [
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_POST           => true,
            CURLOPT_POSTFIELDS     => http_build_query($body, '', '&', PHP_QUERY_RFC3986),
            CURLOPT_FOLLOWLOCATION => false,
            CURLOPT_CONNECTTIMEOUT => 5,
            CURLOPT_TIMEOUT        => 15,
            CURLOPT_SSL_VERIFYPEER => true,
            CURLOPT_SSL_VERIFYHOST => 2,
            CURLOPT_HTTPHEADER     => array_merge(
                ['Content-Type: application/x-www-form-urlencoded'],
                $headers
            ),
            CURLOPT_USERAGENT      => 'pay-oauth2/1.0',
        ]);
        if ($basicUser !== null) {
            curl_setopt($ch, CURLOPT_USERPWD, $basicUser);
            curl_setopt($ch, CURLOPT_HTTPAUTH, CURLAUTH_BASIC);
        }
        $response = curl_exec($ch);
        $status = (int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
        $error = curl_error($ch);
        curl_close($ch);
        if ($response === false) {
            throw new RuntimeException('token 请求失败: ' . $error);
        }
        $decoded = json_decode((string) $response, true);
        if (!is_array($decoded)) {
            throw new RuntimeException('token 端点 HTTP ' . $status . ' 返回非 JSON');
        }
        return $decoded;
    }

    private function rsaPem(string $nB64, string $eB64): string
    {
        $n = Support::base64urlDecode($nB64);
        $e = Support::base64urlDecode($eB64);
        if ($n === '' || $e === '') {
            return '';
        }
        $rsaKey = $this->der(0x30, $this->derInteger($n) . $this->derInteger($e));
        $algorithm = $this->der(
            0x30,
            $this->der(0x06, "\x2a\x86\x48\x86\xf7\x0d\x01\x01\x01") . $this->der(0x05, '')
        );
        $spki = $this->der(0x30, $algorithm . $this->derBitString($rsaKey));
        return $this->pem($spki);
    }

    private function ecPem(string $xB64, string $yB64): string
    {
        $x = Support::base64urlDecode($xB64);
        $y = Support::base64urlDecode($yB64);
        if (strlen($x) !== 32 || strlen($y) !== 32) {
            return '';
        }
        $algorithm = $this->der(
            0x30,
            $this->der(0x06, "\x2a\x86\x48\xce\x3d\x02\x01")
            . $this->der(0x06, "\x2a\x86\x48\xce\x3d\x03\x01\x07")
        );
        $spki = $this->der(0x30, $algorithm . $this->derBitString("\x04" . $x . $y));
        return $this->pem($spki);
    }

    private function pem(string $der): string
    {
        return "-----BEGIN PUBLIC KEY-----\n"
            . chunk_split(base64_encode($der), 64, "\n")
            . "-----END PUBLIC KEY-----\n";
    }

    private function der(int $tag, string $content): string
    {
        return chr($tag) . $this->derLength(strlen($content)) . $content;
    }

    private function derLength(int $length): string
    {
        if ($length < 0x80) {
            return chr($length);
        }
        $bytes = ltrim(pack('N', $length), "\x00");
        return chr(0x80 | strlen($bytes)) . $bytes;
    }

    private function derInteger(string $binary): string
    {
        $binary = ltrim($binary, "\x00");
        if ($binary === '') {
            $binary = "\x00";
        }
        if (ord($binary[0]) > 0x7f) {
            $binary = "\x00" . $binary;
        }
        return $this->der(0x02, $binary);
    }

    private function derBitString(string $content): string
    {
        return $this->der(0x03, "\x00" . $content);
    }

    /** JWS 的 ES256 签名是 R||S 原始格式，OpenSSL 需要 DER 序列 */
    private function ecdsaRawToDer(string $raw): string
    {
        if (strlen($raw) !== 64) {
            throw new RuntimeException('ES256 签名长度非法');
        }
        $r = ltrim(substr($raw, 0, 32), "\x00");
        $s = ltrim(substr($raw, 32), "\x00");
        return $this->der(0x30, $this->derInteger($r) . $this->derInteger($s));
    }
}
