<?php
declare(strict_types=1);

namespace PayOauth;

use PDO;
use PDOException;

/**
 * 复用 pay 主程序的数据库连接信息（config/config.yaml），
 * 只做参数化查询，不做任何字符串拼接 SQL。
 */
final class Database
{
    private static ?PDO $pdo = null;
    private static array $meta = [];

    public static function boot(Config $config): void
    {
        if (self::$pdo instanceof PDO) {
            return;
        }
        $yaml = Config::readSimpleYaml((string) $config->get('pay.config_file'));
        $db = $yaml['database'] ?? [];
        $host = (string) ($db['host'] ?? '127.0.0.1');
        $port = (string) ($db['port'] ?? '3306');
        $name = (string) ($db['dbname'] ?? '');
        $user = (string) ($db['user'] ?? '');
        $pass = (string) ($db['password'] ?? '');
        $charset = (string) ($db['charset'] ?? 'utf8mb4');
        if ($name === '' || $user === '') {
            throw new \RuntimeException('pay config.yaml 中缺少 database 凭据');
        }
        $dsn = sprintf('mysql:host=%s;port=%s;dbname=%s;charset=%s', $host, $port, $name, $charset);
        self::$pdo = new PDO($dsn, $user, $pass, [
            PDO::ATTR_ERRMODE            => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
            PDO::ATTR_EMULATE_PREPARES   => false,
            PDO::ATTR_STRINGIFY_FETCHES  => false,
        ]);
        self::$pdo->exec("SET SESSION sql_mode='NO_ENGINE_SUBSTITUTION'");
        self::$meta = ['host' => $host, 'dbname' => $name];
    }

    public static function pdo(): PDO
    {
        if (!self::$pdo instanceof PDO) {
            throw new \RuntimeException('数据库尚未初始化');
        }
        return self::$pdo;
    }

    public static function meta(): array
    {
        return self::$meta;
    }

    public static function fetchOne(string $sql, array $params = []): ?array
    {
        $stmt = self::pdo()->prepare($sql);
        $stmt->execute($params);
        $row = $stmt->fetch();
        return $row === false ? null : $row;
    }

    public static function fetchAll(string $sql, array $params = []): array
    {
        $stmt = self::pdo()->prepare($sql);
        $stmt->execute($params);
        return $stmt->fetchAll();
    }

    public static function execute(string $sql, array $params = []): int
    {
        $stmt = self::pdo()->prepare($sql);
        $stmt->execute($params);
        return $stmt->rowCount();
    }

    public static function insert(string $sql, array $params = []): int
    {
        self::execute($sql, $params);
        return (int) self::pdo()->lastInsertId();
    }

    public static function transaction(callable $fn): mixed
    {
        $pdo = self::pdo();
        $pdo->beginTransaction();
        try {
            $result = $fn($pdo);
            $pdo->commit();
            return $result;
        } catch (\Throwable $e) {
            if ($pdo->inTransaction()) {
                $pdo->rollBack();
            }
            throw $e;
        }
    }

    public static function isDuplicateKey(PDOException $e): bool
    {
        return $e->getCode() === '23000' || (int) ($e->errorInfo[1] ?? 0) === 1062;
    }
}
