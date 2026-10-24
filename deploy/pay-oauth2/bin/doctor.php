<?php
declare(strict_types=1);

/**
 * CLI 自检：配置 / 数据库 / 表 / IdP 连通性。
 * 用法：/www/server/php/81/bin/php /www/wwwroot/pay-oauth2/bin/doctor.php
 */

if (PHP_SAPI !== 'cli') {
    http_response_code(404);
    exit;
}

require dirname(__DIR__) . '/lib/Support.php';
require dirname(__DIR__) . '/lib/Config.php';
require dirname(__DIR__) . '/lib/Database.php';
require dirname(__DIR__) . '/lib/Audit.php';
require dirname(__DIR__) . '/lib/OAuth2Client.php';
require dirname(__DIR__) . '/lib/PayAccount.php';

use PayOauth\Config;
use PayOauth\Database;
use PayOauth\OAuth2Client;
use PayOauth\PayAccount;

$root = dirname(__DIR__);
$config = Config::load($root . '/config.php', $root);

function line(string $label, string $value, bool $ok = true): void
{
    printf("[%s] %-18s %s\n", $ok ? 'OK  ' : 'FAIL', $label, $value);
}

$failed = 0;

line('php', PHP_VERSION);
line('enabled', $config->get('enabled') ? 'true' : 'false', (bool) $config->get('enabled'));
line('issuer', (string) $config->get('oauth.issuer', '') ?: '(未填写)', (string) $config->get('oauth.issuer', '') !== '');
line('client_id', (string) $config->get('oauth.client_id', '') ?: '(未填写)', (string) $config->get('oauth.client_id', '') !== '');
line('client_secret', (string) $config->get('oauth.client_secret', '') !== '' ? '已填写' : '(未填写)', (string) $config->get('oauth.client_secret', '') !== '');

try {
    Database::boot($config);
    $meta = Database::meta();
    line('database', $meta['host'] . '/' . $meta['dbname']);
} catch (Throwable $e) {
    line('database', $e->getMessage(), false);
    $failed++;
}

foreach (['ls_oauth_identity', 'ls_oauth_audit', 'ls_oauth_rate'] as $table) {
    try {
        Database::fetchOne('SELECT 1 FROM ' . $table . ' LIMIT 1');
        line('table', $table);
    } catch (Throwable $e) {
        line('table', $table . ' 缺失，请先导入 install.sql', false);
        $failed++;
    }
}

try {
    $row = Database::fetchOne('SELECT config_value FROM admin_config WHERE config_name = ? LIMIT 1', ['user_login_token_hours']);
    line('token_hours', (string) ($row['config_value'] ?? '168') . ' 小时');
} catch (Throwable $e) {
    line('token_hours', $e->getMessage(), false);
}

if ($config->isConfigured()) {
    try {
        $client = new OAuth2Client($config, $root . '/var');
        $idpMeta = $client->metadata();
        line('idp.authorize', (string) $idpMeta['authorization_endpoint']);
        line('idp.token', (string) $idpMeta['token_endpoint']);
        line('idp.userinfo', (string) ($idpMeta['userinfo_endpoint'] ?? '(无)'));
        line('idp.jwks', (string) ($idpMeta['jwks_uri'] ?? '(无)'));
    } catch (Throwable $e) {
        line('idp', $e->getMessage(), false);
        $failed++;
    }
} else {
    line('idp', '未配置，跳过探测', false);
    $failed++;
}

try {
    $account = new PayAccount($config);
    line('provider_key', $account->provider());
    line('login_hours', $account->loginHours() . ' 小时');
} catch (Throwable $e) {
    line('account', $e->getMessage(), false);
    $failed++;
}

$varDir = $root . '/var';
line(
    'var.dir',
    is_dir($varDir) ? $varDir . ' (writable=' . (is_writable($varDir) ? 'yes' : 'no') . ')' : '缺失',
    is_dir($varDir) && is_writable($varDir)
);

echo $failed === 0 ? "\n结果：全部通过\n" : "\n结果：{$failed} 项需要处理\n";
exit($failed === 0 ? 0 : 1);
