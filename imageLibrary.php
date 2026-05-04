<?php

declare(strict_types=1);

if (is_file(__DIR__ . '/vendor/autoload.php')) {
    require_once __DIR__ . '/vendor/autoload.php';
}

loadProjectEnv(__DIR__ . '/.env');

require_once __DIR__ . '/r2Storage.php';

function envValue(string $name, ?string $default = null): ?string
{
    if (function_exists('getenv')) {
        $value = getenv($name);
        if ($value !== false) {
            return (string)$value;
        }
    }

    if (array_key_exists($name, $_ENV)) {
        return is_string($_ENV[$name]) ? $_ENV[$name] : (string)$_ENV[$name];
    }

    if (array_key_exists($name, $_SERVER)) {
        return is_string($_SERVER[$name]) ? $_SERVER[$name] : (string)$_SERVER[$name];
    }

    return $default;
}

function loadProjectEnv(string $envFile): void
{
    static $loaded = false;

    if ($loaded || !is_file($envFile) || !is_readable($envFile)) {
        $loaded = true;
        return;
    }

    $lines = file($envFile, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
    if (!is_array($lines)) {
        $loaded = true;
        return;
    }

    foreach ($lines as $line) {
        $line = trim($line);
        if ($line === '' || $line[0] === '#') {
            continue;
        }

        $pos = strpos($line, '=');
        if ($pos === false) {
            continue;
        }

        $name = trim(substr($line, 0, $pos));
        $value = trim(substr($line, $pos + 1));
        if ($name === '' || preg_match('/^[A-Z0-9_]+$/i', $name) !== 1) {
            continue;
        }

        if ((str_starts_with($value, '"') && str_ends_with($value, '"')) ||
            (str_starts_with($value, "'") && str_ends_with($value, "'"))
        ) {
            $value = substr($value, 1, -1);
        }

        $_ENV[$name] = $value;
        $_SERVER[$name] = $value;
        if (function_exists('putenv')) {
            @putenv("{$name}={$value}");
        }
    }

    $loaded = true;
}


function getDatabaseDriver(): string
{
    $driver = strtolower(trim((string)(envValue('IMG_DB_DRIVER', 'sqlite') ?: 'sqlite')));
    return in_array($driver, ['sqlite', 'mysql'], true) ? $driver : 'sqlite';
}

function getSqliteDatabasePath(): string
{
    $path = trim((string)(envValue('IMG_SQLITE_PATH', '') ?: ''));
    if ($path !== '') {
        return $path;
    }

    return __DIR__ . '/database/imget.sqlite';
}

function getDatabaseDsn(): string
{
    if (getDatabaseDriver() === 'mysql') {
        $host = (string)(envValue('IMG_DB_HOST', '127.0.0.1') ?: '127.0.0.1');
        $port = (string)(envValue('IMG_DB_PORT', '3306') ?: '3306');
        $name = (string)(envValue('IMG_DB_NAME', 'imget') ?: 'imget');
        $charset = (string)(envValue('IMG_DB_CHARSET', 'utf8mb4') ?: 'utf8mb4');
        return "mysql:host={$host};port={$port};dbname={$name};charset={$charset}";
    }

    $path = getSqliteDatabasePath();
    $dir = dirname($path);
    if (!is_dir($dir)) {
        @mkdir($dir, 0777, true);
    }

    return 'sqlite:' . $path;
}

function getDatabaseUser(): string
{
    return (string)(envValue('IMG_DB_USER', '') ?: '');
}

function getDatabasePassword(): string
{
    return (string)(envValue('IMG_DB_PASS', '') ?: '');
}

function getDatabase(): PDO
{
    static $pdo = null;

    if ($pdo instanceof PDO) {
        return $pdo;
    }

    $driver = getDatabaseDriver();
    $dsn = getDatabaseDsn();
    $user = $driver === 'mysql' ? getDatabaseUser() : null;
    $pass = $driver === 'mysql' ? getDatabasePassword() : null;

    $pdo = new PDO($dsn, $user, $pass, [
        PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
        PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
        PDO::ATTR_EMULATE_PREPARES => false,
    ]);

    if ($driver === 'sqlite') {
        try {
            $pdo->exec('PRAGMA journal_mode = WAL');
        } catch (PDOException $e) {
            // Another process may already hold a transient lock during first open.
        }
        try {
            $pdo->exec('PRAGMA busy_timeout = 5000');
        } catch (PDOException $e) {
            // Continue with SQLite defaults if PRAGMA cannot be applied.
        }
    }

    initializeDatabaseSchema($pdo, $driver);

    return $pdo;
}

function initializeDatabaseSchema(PDO $pdo, string $driver): void
{
    $adminUsersSql = $driver === 'mysql'
        ? <<<SQL
CREATE TABLE IF NOT EXISTS admin_users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at VARCHAR(35) NOT NULL,
    updated_at VARCHAR(35) NOT NULL,
    last_login_at VARCHAR(35) NULL
)
SQL
        : <<<SQL
CREATE TABLE IF NOT EXISTS admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at VARCHAR(35) NOT NULL,
    updated_at VARCHAR(35) NOT NULL,
    last_login_at VARCHAR(35) NULL
)
SQL;

    $requestProfilesSql = <<<SQL
CREATE TABLE IF NOT EXISTS request_profiles (
    profile_key VARCHAR(255) PRIMARY KEY,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    type VARCHAR(100) NOT NULL,
    keyword TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    first_requested_at VARCHAR(35) NOT NULL,
    last_requested_at VARCHAR(35) NOT NULL,
    last_seen_on VARCHAR(10) NOT NULL,
    last_daily_topup_on VARCHAR(10) NULL,
    last_daily_topup_saved INTEGER NOT NULL DEFAULT 0,
    initial_prefetch_done_at VARCHAR(35) NULL,
    last_manual_refresh_at VARCHAR(35) NULL,
    last_manual_refresh_saved INTEGER NOT NULL DEFAULT 0
)
SQL;

    $urlCacheSql = <<<SQL
CREATE TABLE IF NOT EXISTS url_cache (
    cache_key VARCHAR(255) PRIMARY KEY,
    selected_path TEXT NOT NULL,
    updated_at VARCHAR(35) NOT NULL
)
SQL;

    $refreshLogsSql = $driver === 'mysql'
        ? <<<SQL
CREATE TABLE IF NOT EXISTS refresh_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    profile_key VARCHAR(255) NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    type VARCHAR(100) NOT NULL,
    keyword TEXT NOT NULL,
    mode VARCHAR(20) NOT NULL,
    requested_count INTEGER NOT NULL,
    saved_count INTEGER NOT NULL,
    error_text TEXT NULL,
    created_at VARCHAR(35) NOT NULL
)
SQL
        : <<<SQL
CREATE TABLE IF NOT EXISTS refresh_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_key VARCHAR(255) NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    type VARCHAR(100) NOT NULL,
    keyword TEXT NOT NULL,
    mode VARCHAR(20) NOT NULL,
    requested_count INTEGER NOT NULL,
    saved_count INTEGER NOT NULL,
    error_text TEXT NULL,
    created_at VARCHAR(35) NOT NULL
)
SQL;

    $appSettingsSql = <<<SQL
CREATE TABLE IF NOT EXISTS app_settings (
    setting_key VARCHAR(100) PRIMARY KEY,
    setting_value TEXT NOT NULL,
    updated_at VARCHAR(35) NOT NULL
)
SQL;

    $pdo->exec($adminUsersSql);
    $pdo->exec($requestProfilesSql);
    $pdo->exec($urlCacheSql);
    $pdo->exec($refreshLogsSql);
    $pdo->exec($appSettingsSql);
    r2InitializeSchema($pdo, $driver);
    ensureRequestProfilesStatColumns($pdo, $driver);
    initializeDefaultAdminUser($pdo);
}

function ensureRequestProfilesStatColumns(PDO $pdo, string $driver): void
{
    ensureTableColumn(
        $pdo,
        $driver,
        'request_profiles',
        'view_count',
        'INTEGER NOT NULL DEFAULT 0',
        'BIGINT UNSIGNED NOT NULL DEFAULT 0'
    );
    ensureTableColumn(
        $pdo,
        $driver,
        'request_profiles',
        'download_count',
        'INTEGER NOT NULL DEFAULT 0',
        'BIGINT UNSIGNED NOT NULL DEFAULT 0'
    );
}

function ensureTableColumn(
    PDO $pdo,
    string $driver,
    string $table,
    string $column,
    string $sqliteDefinition,
    string $mysqlDefinition
): void {
    if (tableColumnExists($pdo, $driver, $table, $column)) {
        return;
    }

    $definition = $driver === 'mysql' ? $mysqlDefinition : $sqliteDefinition;

    try {
        $pdo->exec("ALTER TABLE {$table} ADD COLUMN {$column} {$definition}");
    } catch (PDOException $e) {
        if (!tableColumnExists($pdo, $driver, $table, $column)) {
            throw $e;
        }
    }
}

function tableColumnExists(PDO $pdo, string $driver, string $table, string $column): bool
{
    if ($driver === 'mysql') {
        $stmt = $pdo->prepare("SHOW COLUMNS FROM {$table} LIKE :column");
        $stmt->execute(['column' => $column]);
        return (bool)$stmt->fetch();
    }

    $stmt = $pdo->query("PRAGMA table_info({$table})");
    foreach ($stmt->fetchAll() as $row) {
        if ((string)($row['name'] ?? '') === $column) {
            return true;
        }
    }

    return false;
}

function initializeDefaultAdminUser(PDO $pdo): void
{
    $count = (int)$pdo->query('SELECT COUNT(*) FROM admin_users')->fetchColumn();
    if ($count > 0) {
        return;
    }

    $now = gmdate(DATE_ATOM);
    $stmt = $pdo->prepare(
        'INSERT INTO admin_users (username, password_hash, created_at, updated_at, last_login_at)
         VALUES (:username, :password_hash, :created_at, :updated_at, :last_login_at)'
    );
    $stmt->execute([
        'username' => 'admin',
        'password_hash' => password_hash('123456', PASSWORD_DEFAULT),
        'created_at' => $now,
        'updated_at' => $now,
        'last_login_at' => null,
    ]);
}

function storageGetUrlCache(string $cacheKey): ?string
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare('SELECT selected_path FROM url_cache WHERE cache_key = :cache_key');
    $stmt->execute(['cache_key' => $cacheKey]);
    $value = $stmt->fetchColumn();
    return $value === false ? null : (string)$value;
}

function storagePutUrlCache(string $cacheKey, string $selectedPath): void
{
    $pdo = getDatabase();
    $existing = storageGetUrlCache($cacheKey);
    $params = [
        'cache_key' => $cacheKey,
        'selected_path' => $selectedPath,
        'updated_at' => gmdate(DATE_ATOM),
    ];

    if ($existing === null) {
        $stmt = $pdo->prepare(
            'INSERT INTO url_cache (cache_key, selected_path, updated_at) VALUES (:cache_key, :selected_path, :updated_at)'
        );
        $stmt->execute($params);
        return;
    }

    $stmt = $pdo->prepare(
        'UPDATE url_cache SET selected_path = :selected_path, updated_at = :updated_at WHERE cache_key = :cache_key'
    );
    $stmt->execute($params);
}

function storageDeleteUrlCache(string $cacheKey): void
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare('DELETE FROM url_cache WHERE cache_key = :cache_key');
    $stmt->execute(['cache_key' => $cacheKey]);
}

function storageGetRequestedProfile(string $profileKey): ?array
{
    $pdo = getDatabase();
    return storageFetchRequestedProfile($pdo, $profileKey);
}

function storageFetchRequestedProfile(PDO $pdo, string $profileKey): ?array
{
    $stmt = $pdo->prepare('SELECT * FROM request_profiles WHERE profile_key = :profile_key');
    $stmt->execute(['profile_key' => $profileKey]);
    $row = $stmt->fetch();
    return is_array($row) ? $row : null;
}

function storageGetRequestedProfiles(int $limit = 0, int $offset = 0): array
{
    $pdo = getDatabase();
    $sql = 'SELECT * FROM request_profiles ORDER BY last_requested_at DESC, profile_key ASC';
    if ($limit > 0) {
        $sql .= ' LIMIT ' . max(1, min(500, $limit)) . ' OFFSET ' . max(0, $offset);
    }
    $stmt = $pdo->query($sql);
    $profiles = [];

    foreach ($stmt->fetchAll() as $row) {
        if (!is_array($row) || empty($row['profile_key'])) {
            continue;
        }

        $profiles[(string)$row['profile_key']] = $row;
    }

    return $profiles;
}

function storageDeleteRequestedProfile(string $profileKey): bool
{
    if ($profileKey === '') {
        return false;
    }
    $pdo = getDatabase();
    $stmt = $pdo->prepare('DELETE FROM request_profiles WHERE profile_key = :profile_key');
    $stmt->execute(['profile_key' => $profileKey]);
    return $stmt->rowCount() > 0;
}

function storageSaveRequestedProfile(array $profile): void
{
    $pdo = getDatabase();
    storageUpsertRequestedProfile($pdo, $profile);
}

function storageUpsertRequestedProfile(PDO $pdo, array $profile): void
{
    $profileKey = (string)($profile['profile_key'] ?? '');
    if ($profileKey === '') {
        return;
    }

    $existing = storageFetchRequestedProfile($pdo, $profileKey);
    $current = array_merge([
        'profile_key' => $profileKey,
        'width' => 0,
        'height' => 0,
        'type' => 'banner',
        'keyword' => '',
        'request_count' => 0,
        'view_count' => 0,
        'download_count' => 0,
        'first_requested_at' => gmdate(DATE_ATOM),
        'last_requested_at' => gmdate(DATE_ATOM),
        'last_seen_on' => gmdate('Y-m-d'),
        'last_daily_topup_on' => null,
        'last_daily_topup_saved' => 0,
        'initial_prefetch_done_at' => null,
        'last_manual_refresh_at' => null,
        'last_manual_refresh_saved' => 0,
    ], is_array($existing) ? $existing : [], $profile);

    $stmt = $pdo->prepare(
        'REPLACE INTO request_profiles (
            profile_key, width, height, type, keyword, request_count, view_count, download_count, first_requested_at,
            last_requested_at, last_seen_on, last_daily_topup_on, last_daily_topup_saved,
            initial_prefetch_done_at, last_manual_refresh_at, last_manual_refresh_saved
        ) VALUES (
            :profile_key, :width, :height, :type, :keyword, :request_count, :view_count, :download_count, :first_requested_at,
            :last_requested_at, :last_seen_on, :last_daily_topup_on, :last_daily_topup_saved,
            :initial_prefetch_done_at, :last_manual_refresh_at, :last_manual_refresh_saved
        )'
    );

    $stmt->execute([
        'profile_key' => $current['profile_key'],
        'width' => (int)$current['width'],
        'height' => (int)$current['height'],
        'type' => (string)$current['type'],
        'keyword' => (string)$current['keyword'],
        'request_count' => (int)$current['request_count'],
        'view_count' => (int)$current['view_count'],
        'download_count' => (int)$current['download_count'],
        'first_requested_at' => (string)$current['first_requested_at'],
        'last_requested_at' => (string)$current['last_requested_at'],
        'last_seen_on' => (string)$current['last_seen_on'],
        'last_daily_topup_on' => $current['last_daily_topup_on'] !== null ? (string)$current['last_daily_topup_on'] : null,
        'last_daily_topup_saved' => (int)$current['last_daily_topup_saved'],
        'initial_prefetch_done_at' => $current['initial_prefetch_done_at'] !== null ? (string)$current['initial_prefetch_done_at'] : null,
        'last_manual_refresh_at' => $current['last_manual_refresh_at'] !== null ? (string)$current['last_manual_refresh_at'] : null,
        'last_manual_refresh_saved' => (int)$current['last_manual_refresh_saved'],
    ]);
}

function storageRegisterRequestedProfile(int $width, int $height, string $type, string $keyword, string $profileKey): array
{
    $pdo = getDatabase();
    $today = gmdate('Y-m-d');
    $now = gmdate(DATE_ATOM);

    $pdo->beginTransaction();

    try {
        $current = storageFetchRequestedProfile($pdo, $profileKey);
        $isNew = !is_array($current);
        $current = array_merge([
            'profile_key' => $profileKey,
            'width' => $width,
            'height' => $height,
            'type' => $type,
            'keyword' => $keyword,
            'request_count' => 0,
            'view_count' => 0,
            'download_count' => 0,
            'first_requested_at' => $now,
            'last_requested_at' => $now,
            'last_seen_on' => $today,
            'last_daily_topup_on' => null,
            'last_daily_topup_saved' => 0,
            'initial_prefetch_done_at' => null,
            'last_manual_refresh_at' => null,
            'last_manual_refresh_saved' => 0,
        ], is_array($current) ? $current : []);

        $current['width'] = $width;
        $current['height'] = $height;
        $current['type'] = $type;
        $current['keyword'] = $keyword;
        $current['request_count'] = (int)($current['request_count'] ?? 0) + 1;
        $current['last_requested_at'] = $now;
        $current['last_seen_on'] = $today;

        storageUpsertRequestedProfile($pdo, $current);
        $pdo->commit();

        return [
            'is_new' => $isNew,
            'profile_key' => $profileKey,
        ];
    } catch (Throwable $e) {
        if ($pdo->inTransaction()) {
            $pdo->rollBack();
        }

        return [
            'is_new' => false,
            'profile_key' => $profileKey,
        ];
    }
}

function storageAddRefreshLog(array $log): void
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare(
        'INSERT INTO refresh_logs (
            profile_key, width, height, type, keyword, mode, requested_count, saved_count, error_text, created_at
        ) VALUES (
            :profile_key, :width, :height, :type, :keyword, :mode, :requested_count, :saved_count, :error_text, :created_at
        )'
    );

    $stmt->execute([
        'profile_key' => (string)($log['profile_key'] ?? ''),
        'width' => (int)($log['width'] ?? 0),
        'height' => (int)($log['height'] ?? 0),
        'type' => (string)($log['type'] ?? 'banner'),
        'keyword' => (string)($log['keyword'] ?? ''),
        'mode' => (string)($log['mode'] ?? 'manual'),
        'requested_count' => (int)($log['requested_count'] ?? 0),
        'saved_count' => (int)($log['saved_count'] ?? 0),
        'error_text' => ($log['error_text'] ?? null) !== null ? (string)$log['error_text'] : null,
        'created_at' => (string)($log['created_at'] ?? gmdate(DATE_ATOM)),
    ]);
}

function storageGetRefreshLogs(int $limit = 50, int $offset = 0): array
{
    $limit = max(1, min(500, $limit));
    $offset = max(0, $offset);
    $pdo = getDatabase();
    $stmt = $pdo->query("SELECT * FROM refresh_logs ORDER BY id DESC LIMIT {$limit} OFFSET {$offset}");
    return $stmt->fetchAll();
}

function storageCountRefreshLogs(): int
{
    $pdo = getDatabase();
    $value = $pdo->query('SELECT COUNT(*) FROM refresh_logs')->fetchColumn();
    return max(0, (int)$value);
}

function storageClearRefreshLogs(): bool
{
    $pdo = getDatabase();
    return $pdo->exec('DELETE FROM refresh_logs') !== false;
}

function storageGetDatabaseSummary(): array
{
    $driver = getDatabaseDriver();
    $summary = [
        'driver' => $driver,
        'label' => $driver === 'mysql'
            ? ((string)(envValue('IMG_DB_HOST', '127.0.0.1') ?: '127.0.0.1')) . '/' . ((string)(envValue('IMG_DB_NAME', 'imget') ?: 'imget'))
            : getSqliteDatabasePath(),
    ];

    if ($driver === 'sqlite' && is_file(getSqliteDatabasePath())) {
        $summary['size_bytes'] = filesize(getSqliteDatabasePath()) ?: 0;
    }

    return $summary;
}

function storageGetAppSettings(): array
{
    $pdo = getDatabase();
    $rows = $pdo->query('SELECT setting_key, setting_value FROM app_settings')->fetchAll();
    $settings = [];

    foreach ($rows as $row) {
        if (!is_array($row)) {
            continue;
        }
        $key = (string)($row['setting_key'] ?? '');
        if ($key === '') {
            continue;
        }
        $settings[$key] = (string)($row['setting_value'] ?? '');
    }

    return $settings;
}

function storageGetAppSetting(string $key, string $default = ''): string
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare('SELECT setting_value FROM app_settings WHERE setting_key = :setting_key LIMIT 1');
    $stmt->execute(['setting_key' => $key]);
    $value = $stmt->fetchColumn();

    return $value !== false ? (string)$value : $default;
}

function storageSetAppSetting(string $key, string $value): void
{
    $pdo = getDatabase();
    $now = gmdate(DATE_ATOM);
    $driver = getDatabaseDriver();

    if ($driver === 'mysql') {
        $stmt = $pdo->prepare(
            'INSERT INTO app_settings (setting_key, setting_value, updated_at)
             VALUES (:setting_key, :setting_value, :updated_at)
             ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value), updated_at = VALUES(updated_at)'
        );
        $stmt->execute([
            'setting_key' => $key,
            'setting_value' => $value,
            'updated_at' => $now,
        ]);
        return;
    }

    $stmt = $pdo->prepare(
        'INSERT INTO app_settings (setting_key, setting_value, updated_at)
         VALUES (:setting_key, :setting_value, :updated_at)
         ON CONFLICT(setting_key) DO UPDATE SET setting_value = excluded.setting_value, updated_at = excluded.updated_at'
    );
    $stmt->execute([
        'setting_key' => $key,
        'setting_value' => $value,
        'updated_at' => $now,
    ]);
}

function getConfiguredAssetBaseUrl(): string
{
    return r2GetCdnBaseUrl();
}

function getConfiguredSiteBaseUrl(): string
{
    // 直接写死配置为 img.et
    return 'https://img.et';
}

function getConfiguredOriginalCdnUrl(): string
{
    return r2GetCdnBaseUrl();
}

function storageGetAdminUserById(int $adminUserId): ?array
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare('SELECT * FROM admin_users WHERE id = :id');
    $stmt->execute(['id' => $adminUserId]);
    $row = $stmt->fetch();
    return is_array($row) ? $row : null;
}

function storageGetAdminUserByUsername(string $username): ?array
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare('SELECT * FROM admin_users WHERE username = :username');
    $stmt->execute(['username' => $username]);
    $row = $stmt->fetch();
    return is_array($row) ? $row : null;
}

function storageAuthenticateAdminUser(string $username, string $password): ?array
{
    $user = storageGetAdminUserByUsername($username);
    if (!is_array($user)) {
        return null;
    }

    $hash = (string)($user['password_hash'] ?? '');
    if ($hash === '' || !password_verify($password, $hash)) {
        return null;
    }

    $pdo = getDatabase();
    $stmt = $pdo->prepare('UPDATE admin_users SET last_login_at = :last_login_at WHERE id = :id');
    $stmt->execute([
        'last_login_at' => gmdate(DATE_ATOM),
        'id' => (int)$user['id'],
    ]);

    return storageGetAdminUserById((int)$user['id']);
}

function storageUpdateAdminUserCredentials(int $adminUserId, string $username, string $password): array
{
    $username = trim($username);
    $password = trim($password);

    if ($username === '' || $password === '') {
        return [
            'ok' => false,
            'error' => 'username_or_password_empty',
        ];
    }

    $existing = storageGetAdminUserByUsername($username);
    if (is_array($existing) && (int)$existing['id'] !== $adminUserId) {
        return [
            'ok' => false,
            'error' => 'username_taken',
        ];
    }

    $pdo = getDatabase();
    $stmt = $pdo->prepare(
        'UPDATE admin_users
         SET username = :username, password_hash = :password_hash, updated_at = :updated_at
         WHERE id = :id'
    );
    $stmt->execute([
        'username' => $username,
        'password_hash' => password_hash($password, PASSWORD_DEFAULT),
        'updated_at' => gmdate(DATE_ATOM),
        'id' => $adminUserId,
    ]);

    return [
        'ok' => true,
        'user' => storageGetAdminUserById($adminUserId),
    ];
}

function getTypeKeywordMap(): array
{
    return [
        'banner' => '',
        'landscape' => 'landscape nature mountain lake',
        'beauty' => 'portrait woman beauty fashion',
        'anime' => 'anime illustration manga art',
        'city' => 'city skyline urban architecture street',
        'nature' => 'nature forest ocean sky plants',
        'car' => 'car vehicle supercar racing automotive',
        'game' => 'gaming esports virtual world neon',
        'food' => 'food dessert drink restaurant cuisine',
        'animal' => 'animal pet wildlife nature',
        'travel' => 'travel destination vacation resort beach',
        'space' => 'space galaxy universe stars nebula',
        'tech' => 'technology digital futuristic device ai',
        'business' => 'business office team meeting workspace',
        'sports' => 'sports fitness workout stadium competition',
        'architecture' => 'architecture interior building modern design',
    ];
}

function resolveSearchKeyword(string $type, string $keyword = ''): string
{
    $keyword = trim($keyword);
    if ($keyword !== '') {
        return $keyword;
    }

    $map = getTypeKeywordMap();
    return trim((string)($map[normalizeImageType($type)] ?? ''));
}

function getPixabayCategoryMap(): array
{
    return [
        'beauty' => 'people',
        'city' => 'places',
        'nature' => 'nature',
        'food' => 'food',
        'travel' => 'places',
        'sports' => 'sports',
        'business' => 'business',
        'architecture' => 'buildings',
    ];
}

function resolvePixabayCategory(string $type): string
{
    $map = getPixabayCategoryMap();
    return (string)($map[normalizeImageType($type)] ?? '');
}

function getPixabayImageTypeMap(): array
{
    return [
        'anime' => 'illustration',
        'space' => 'illustration',
        'tech' => 'illustration',
        'banner' => 'photo',
        'landscape' => 'photo',
        'beauty' => 'photo',
        'city' => 'photo',
        'nature' => 'photo',
        'car' => 'photo',
        'game' => 'illustration',
        'food' => 'photo',
        'animal' => 'photo',
        'travel' => 'photo',
        'business' => 'photo',
        'sports' => 'photo',
        'architecture' => 'photo',
    ];
}

function resolvePixabayImageType(string $type): string
{
    $map = getPixabayImageTypeMap();
    return (string)($map[normalizeImageType($type)] ?? 'photo');
}

function resolvePixabayAssetUrl(array $hit): string
{
    foreach (['imageURL', 'fullHDURL', 'largeImageURL', 'webformatURL', 'previewURL'] as $field) {
        if (!empty($hit[$field]) && is_string($hit[$field])) {
            return (string)$hit[$field];
        }
    }

    return '';
}

function getPixabayRateLimitStateFile(): string
{
    $lockDir = __DIR__ . '/database';
    if (!is_dir($lockDir)) {
        @mkdir($lockDir, 0777, true);
    }

    return $lockDir . '/pixabay_rate_limit.json';
}

function throttlePixabayApiRequest(int $minIntervalMs = 900, int $cooldownSeconds = 120): ?string
{
    $stateFile = getPixabayRateLimitStateFile();
    $handle = fopen($stateFile, 'c+');
    if (!$handle || !flock($handle, LOCK_EX)) {
        if (is_resource($handle)) {
            fclose($handle);
        }
        return null;
    }

    $now = time();
    $raw = stream_get_contents($handle);
    $state = json_decode(is_string($raw) ? $raw : '', true);
    if (!is_array($state)) {
        $state = [];
    }

    $cooldownUntil = (int)($state['cooldown_until'] ?? 0);
    if ($cooldownUntil > $now) {
        flock($handle, LOCK_UN);
        fclose($handle);
        return 'Pixabay API cooldown active';
    }

    $lastRequestAt = (float)($state['last_request_at'] ?? 0);
    $elapsedMs = (int)round((microtime(true) - $lastRequestAt) * 1000);
    if ($lastRequestAt > 0 && $elapsedMs < $minIntervalMs) {
        usleep(($minIntervalMs - $elapsedMs) * 1000);
    }

    $state['last_request_at'] = microtime(true);
    $state['cooldown_until'] = 0;

    rewind($handle);
    ftruncate($handle, 0);
    fwrite($handle, json_encode($state));
    fflush($handle);
    flock($handle, LOCK_UN);
    fclose($handle);

    return null;
}

function markPixabayRateLimitExceeded(int $cooldownSeconds = 300): void
{
    $stateFile = getPixabayRateLimitStateFile();
    $handle = fopen($stateFile, 'c+');
    if (!$handle || !flock($handle, LOCK_EX)) {
        if (is_resource($handle)) {
            fclose($handle);
        }
        return;
    }

    $state = [
        'last_request_at' => microtime(true),
        'cooldown_until' => time() + $cooldownSeconds,
    ];

    rewind($handle);
    ftruncate($handle, 0);
    fwrite($handle, json_encode($state));
    fflush($handle);
    flock($handle, LOCK_UN);
    fclose($handle);
}

function getPixabayRateLimitStatus(): array
{
    $stateFile = getPixabayRateLimitStateFile();
    if (!is_file($stateFile) || !is_readable($stateFile)) {
        return [
            'cooldown_until' => 0,
            'remaining_seconds' => 0,
            'active' => false,
        ];
    }

    $raw = @file_get_contents($stateFile);
    $state = json_decode(is_string($raw) ? $raw : '', true);
    if (!is_array($state)) {
        return [
            'cooldown_until' => 0,
            'remaining_seconds' => 0,
            'active' => false,
        ];
    }

    $cooldownUntil = (int)($state['cooldown_until'] ?? 0);
    $remaining = max(0, $cooldownUntil - time());

    return [
        'cooldown_until' => $cooldownUntil,
        'remaining_seconds' => $remaining,
        'active' => $remaining > 0,
    ];
}


function fetchImages(int $targetCount, int $width, int $height, string $type, string $keyword = ''): array
{
    $type = normalizeImageType($type);
    $keyword = trim($keyword);
    $searchKeyword = resolveSearchKeyword($type, $keyword);
    $sourceName = $searchKeyword !== '' ? 'pixabay' : 'picsum';
    $pixabayKey = envValue('PIXABAY_API_KEY', '') ?: '';

    $targetDir = getTypeOriginalDir($type);

    if (!ensureDirectory($targetDir)) {
        return [
            'saved' => 0,
            'target' => $targetCount,
            'target_dir' => $targetDir,
            'source' => $sourceName,
            'error' => "Failed to create directory: {$targetDir}",
        ];
    }

    $saved = 0;
    $attempt = 0;
    $maxAttempts = max($targetCount * 5, 5);
    $savedFiles = [];

    while ($saved < $targetCount && $attempt < $maxAttempts) {
        $attempt++;
        $source = buildSourceUrl($width, $height, $type, $searchKeyword, $pixabayKey, $attempt);
        if ($source === null) {
            return [
                'saved' => $saved,
                'target' => $targetCount,
                'target_dir' => $targetDir,
                'source' => $sourceName,
                'error' => $searchKeyword !== ''
                    ? 'Typed image fetch requires PIXABAY_API_KEY for keyword search'
                    : 'Pixabay keyword mode requires PIXABAY_API_KEY',
                'files' => $savedFiles,
            ];
        }
        if ($source === '') {
            continue;
        }

        $tmp = tempnam(sys_get_temp_dir(), 'imgsrc_');
        if ($tmp === false) {
            break;
        }

        $fp = fopen($tmp, 'wb');
        if ($fp === false) {
            @unlink($tmp);
            break;
        }

        $ch = curl_init($source);
        curl_setopt_array($ch, [
            CURLOPT_FILE => $fp,
            CURLOPT_FOLLOWLOCATION => true,
            CURLOPT_TIMEOUT => 20,
            CURLOPT_CONNECTTIMEOUT => 8,
            CURLOPT_USERAGENT => 'img.et daily fetch/1.0',
            CURLOPT_FAILONERROR => true,
        ]);

        $ok = curl_exec($ch);
        $httpCode = (int)curl_getinfo($ch, CURLINFO_HTTP_CODE);
        fclose($fp);

        if (!$ok || $httpCode < 200 || $httpCode >= 400 || !is_file($tmp) || filesize($tmp) < 1024) {
            @unlink($tmp);
            continue;
        }

        $mime = detectImageMime($tmp);
        $extension = mimeToExtension($mime);
        if ($extension === '') {
            @unlink($tmp);
            continue;
        }

        $hash = sha1_file($tmp);
        if ($hash === false) {
            @unlink($tmp);
            continue;
        }

        $dest = "{$targetDir}/{$hash}.{$extension}";
        if (is_file($dest)) {
            @unlink($tmp);
            continue;
        }

        $written = @rename($tmp, $dest);
        if (!$written) {
            $written = @copy($tmp, $dest);
            @unlink($tmp);
        }

        if ($written && is_file($dest)) {
            $saved++;
            $savedFiles[] = $dest;
            scheduleR2Upload($dest);
            continue;
        }

        @unlink($tmp);
        @unlink($dest);
    }

    return [
        'saved' => $saved,
        'target' => $targetCount,
        'target_dir' => $targetDir,
        'source' => $sourceName,
        'files' => $savedFiles,
        'error' => $saved < $targetCount ? "Done with partial result: {$saved}/{$targetCount}" : null,
    ];
}

function fetchImagesWithRetries(
    int $targetCount,
    int $width,
    int $height,
    string $type,
    string $keyword = '',
    int $rounds = 3,
    int $sleepMs = 500
): array {
    $rounds = max(1, $rounds);
    $final = [
        'saved' => 0,
        'target' => $targetCount,
        'target_dir' => getTypeOriginalDir($type),
        'source' => resolveSearchKeyword($type, $keyword) !== '' ? 'pixabay' : 'picsum',
        'files' => [],
        'error' => null,
    ];

    for ($round = 1; $round <= $rounds; $round++) {
        $result = fetchImages($targetCount, $width, $height, $type, $keyword);
        $final = $result;

        if ((int)($result['saved'] ?? 0) > 0) {
            return $result;
        }

        if ($round < $rounds && $sleepMs > 0) {
            usleep($sleepMs * 1000);
        }
    }

    return $final;
}

function buildFallbackSeededSourceUrl(int $width, int $height, string $type, string $keyword, int $attempt): string
{
    $seedParts = [
        date('oW'),
        normalizeImageType($type),
        $keyword !== '' ? sha1($keyword) : 'default',
        (string)$attempt,
        bin2hex(random_bytes(4)),
    ];
    $seed = implode('-', $seedParts);
    return "https://picsum.photos/seed/{$seed}/{$width}/{$height}";
}

function buildSourceUrl(
    int $width,
    int $height,
    string $type,
    string $keyword,
    string $pixabayKey,
    int $attempt
): ?string {
    if ($keyword === '') {
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }

    if ($pixabayKey === '') {
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }

    $throttleError = throttlePixabayApiRequest();
    if ($throttleError !== null) {
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }

    $query = http_build_query([
        'key' => $pixabayKey,
        'q' => $keyword,
        'image_type' => resolvePixabayImageType($type),
        'orientation' => $width >= $height ? 'horizontal' : 'vertical',
        'min_width' => $width,
        'min_height' => $height,
        'safesearch' => 'true',
        'per_page' => 200,
        'page' => (($attempt - 1) % 3) + 1,
        'order' => 'popular',
    ]);

    $category = resolvePixabayCategory($type);
    if ($category !== '') {
        $query .= '&category=' . rawurlencode($category);
    }

    $apiUrl = "https://pixabay.com/api/?{$query}";
    $json = @file_get_contents($apiUrl);
    if ($json === false) {
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }

    if (stripos($json, 'rate limit exceeded') !== false) {
        markPixabayRateLimitExceeded();
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }

    $data = json_decode($json, true);
    if (is_array($data) && !empty($data['error']) && stripos((string)$data['error'], 'rate limit') !== false) {
        markPixabayRateLimitExceeded();
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }
    if (!is_array($data) || empty($data['hits']) || !is_array($data['hits'])) {
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }

    $eligible = [];
    foreach ($data['hits'] as $hit) {
        if (!is_array($hit)) {
            continue;
        }

        $candidate = resolvePixabayAssetUrl($hit);

        if ($candidate === '') {
            continue;
        }

        $eligible[] = $candidate;
    }

    if ($eligible === []) {
        return buildFallbackSeededSourceUrl($width, $height, $type, $keyword, $attempt);
    }

    return $eligible[array_rand($eligible)];
}

function detectImageMime(string $file): string
{
    $info = @getimagesize($file);
    if (!is_array($info) || !isset($info['mime'])) {
        return '';
    }

    return (string)$info['mime'];
}

function supportsOutputFormat(string $format): bool
{
    switch (strtolower($format)) {
        case 'webp':
            return function_exists('imagewebp');
        case 'avif':
            return supportsAvifOutput();
        default:
            return false;
    }
}

function supportsAvifOutput(): bool
{
    if (function_exists('imageavif')) {
        return true;
    }

    if (!class_exists('Imagick') || !method_exists('Imagick', 'queryFormats')) {
        return false;
    }

    try {
        $formats = Imagick::queryFormats('AVIF');
        return is_array($formats) && $formats !== [];
    } catch (Throwable $e) {
        return false;
    }
}

function mimeToExtension(string $mime): string
{
    switch ($mime) {
        case 'image/jpeg':
            return 'jpg';
        case 'image/png':
            return 'png';
        case 'image/gif':
            return 'gif';
        case 'image/webp':
            return 'webp';
        case 'image/avif':
            return 'avif';
        default:
            return '';
    }
}

function createGeneratedFallbackImage(int $width, int $height, string $type, string $keyword = ''): ?string
{
    $type = normalizeImageType($type);
    $targetDir = getTypeOriginalDir($type);
    if (!ensureDirectory($targetDir)) {
        return null;
    }

    $canvas = imagecreatetruecolor($width, $height);
    if (!$canvas) {
        return null;
    }

    $background = imagecolorallocate($canvas, 244, 247, 251);
    $border = imagecolorallocate($canvas, 0, 82, 217);
    $muted = imagecolorallocate($canvas, 90, 109, 137);
    $accent = imagecolorallocate($canvas, 0, 82, 217);

    imagefilledrectangle($canvas, 0, 0, $width, $height, $background);
    imagerectangle($canvas, 0, 0, $width - 1, $height - 1, $border);

    $labelType = $type !== '' ? strtoupper($type) : 'BANNER';
    $labelSize = "{$width}x{$height}";
    $labelKeyword = $keyword !== '' ? substr($keyword, 0, 24) : 'AUTO GENERATED';

    imagestring($canvas, 5, 18, max(18, (int)floor($height * 0.22)), $labelType, $accent);
    imagestring($canvas, 4, 18, max(42, (int)floor($height * 0.42)), $labelSize, $muted);
    imagestring($canvas, 3, 18, max(64, (int)floor($height * 0.62)), $labelKeyword, $muted);

    $hash = sha1("fallback|{$type}|{$width}|{$height}|{$keyword}");
    $dest = "{$targetDir}/fallback-{$hash}.png";
    if (!imagepng($canvas, $dest)) {
        return null;
    }

    return is_file($dest) ? $dest : null;
}

function createTemporaryFallbackImage(int $width, int $height, string $type, string $keyword = ''): ?string
{
    $canvas = imagecreatetruecolor($width, $height);
    if (!$canvas) {
        return null;
    }

    $background = imagecolorallocate($canvas, 244, 247, 251);
    $border = imagecolorallocate($canvas, 0, 82, 217);
    $muted = imagecolorallocate($canvas, 90, 109, 137);
    $accent = imagecolorallocate($canvas, 0, 82, 217);

    imagefilledrectangle($canvas, 0, 0, $width, $height, $background);
    imagerectangle($canvas, 0, 0, $width - 1, $height - 1, $border);

    $labelType = strtoupper(normalizeImageType($type));
    $labelSize = "{$width}x{$height}";
    $labelKeyword = $keyword !== '' ? substr($keyword, 0, 24) : 'TEMP FALLBACK';

    imagestring($canvas, 5, 18, max(18, (int)floor($height * 0.22)), $labelType, $accent);
    imagestring($canvas, 4, 18, max(42, (int)floor($height * 0.42)), $labelSize, $muted);
    imagestring($canvas, 3, 18, max(64, (int)floor($height * 0.62)), $labelKeyword, $muted);

    $hash = sha1("temp-fallback|{$type}|{$width}|{$height}|{$keyword}");
    $dest = sys_get_temp_dir() . '/imget-fallback-' . $hash . '.png';
    if (!imagepng($canvas, $dest)) {
        return null;
    }

    return is_file($dest) ? $dest : null;
}

function isGeneratedFallbackImagePath(string $path): bool
{
    $base = strtolower(basename($path));
    return str_starts_with($base, 'fallback-') || str_starts_with($base, 'imget-fallback-');
}

function normalizeImageType(string $type): string
{
    static $allowed = [
        'banner' => true,
        'landscape' => true,
        'beauty' => true,
        'anime' => true,
        'city' => true,
        'nature' => true,
        'car' => true,
        'game' => true,
        'food' => true,
        'animal' => true,
        'travel' => true,
        'space' => true,
        'tech' => true,
        'business' => true,
        'sports' => true,
        'architecture' => true,
    ];

    $type = strtolower($type);
    $type = preg_replace('/[^a-z0-9_-]+/', '', $type) ?? '';
    if ($type === '' || !isset($allowed[$type])) {
        return 'banner';
    }

    return $type;
}

function getImagesRoot(): string
{
    return __DIR__ . '/images';
}

function getOriginalRoot(): string
{
    return getImagesRoot() . '/original';
}

function getTypeOriginalDir(string $type): string
{
    return getOriginalRoot() . '/' . normalizeImageType($type);
}

function ensureDirectory(string $dir): bool
{
    return is_dir($dir) || (mkdir($dir, 0777, true) || is_dir($dir));
}

function isSupportedImageExtension(string $extension): bool
{
    return in_array(strtolower($extension), ['jpg', 'jpeg', 'png', 'gif', 'webp', 'avif'], true);
}

function getRequestProfileLockFile(): string
{
    $lockDir = __DIR__ . '/database';
    if (!is_dir($lockDir)) {
        @mkdir($lockDir, 0777, true);
    }

    return $lockDir . '/request_profiles.lock';
}

function buildRequestProfileKey(int $width, int $height, string $type, string $keyword): string
{
    return "{$width}x{$height}|type={$type}|keyword=" . sha1($keyword);
}

function buildDefaultProfile(string $profileKey, int $width, int $height, string $type, string $keyword): array
{
    $now = gmdate(DATE_ATOM);

    return [
        'profile_key' => $profileKey,
        'width' => $width,
        'height' => $height,
        'type' => $type,
        'keyword' => $keyword,
        'request_count' => 0,
        'view_count' => 0,
        'download_count' => 0,
        'first_requested_at' => $now,
        'last_requested_at' => $now,
        'last_seen_on' => gmdate('Y-m-d'),
        'last_daily_topup_on' => null,
        'last_daily_topup_saved' => 0,
        'initial_prefetch_done_at' => null,
        'last_manual_refresh_at' => null,
        'last_manual_refresh_saved' => 0,
    ];
}

function registerRequestedProfile(int $width, int $height, string $type, string $keyword = ''): array
{
    if (!ensureDirectory(getImagesRoot())) {
        return ['is_new' => false, 'profile_key' => null];
    }

    $type = normalizeImageType($type);
    $keyword = trim($keyword);
    $profileKey = buildRequestProfileKey($width, $height, $type, $keyword);

    return storageRegisterRequestedProfile($width, $height, $type, $keyword, $profileKey);
}

function incrementRequestedProfileMetric(int $width, int $height, string $type, string $keyword, string $metric): array
{
    $allowed = ['view_count', 'download_count'];
    if (!in_array($metric, $allowed, true)) {
        return ['ok' => false];
    }

    $type = normalizeImageType($type);
    $keyword = trim($keyword);
    $profileKey = buildRequestProfileKey($width, $height, $type, $keyword);
    $pdo = getDatabase();

    $pdo->beginTransaction();

    try {
        $profile = storageFetchRequestedProfile($pdo, $profileKey);
        if (!is_array($profile)) {
            $profile = buildDefaultProfile($profileKey, $width, $height, $type, $keyword);
        }

        $profile['width'] = $width;
        $profile['height'] = $height;
        $profile['type'] = $type;
        $profile['keyword'] = $keyword;
        $profile[$metric] = (int)($profile[$metric] ?? 0) + 1;

        storageUpsertRequestedProfile($pdo, $profile);
        $pdo->commit();

        return [
            'ok' => true,
            'profile_key' => $profileKey,
            'view_count' => (int)($profile['view_count'] ?? 0),
            'download_count' => (int)($profile['download_count'] ?? 0),
        ];
    } catch (Throwable $e) {
        if ($pdo->inTransaction()) {
            $pdo->rollBack();
        }
        return ['ok' => false];
    }
}

function getRequestedProfiles(): array
{
    return storageGetRequestedProfiles();
}

function runDailyTopUp(int $dailyIncrement = 10, int $maxImagesPerType = 1000): array
{
    if (!ensureDirectory(getImagesRoot())) {
        return [];
    }

    $today = gmdate('Y-m-d');
    $lock = fopen(getRequestProfileLockFile(), 'c+');
    if (!$lock || !flock($lock, LOCK_EX)) {
        if (is_resource($lock)) {
            fclose($lock);
        }
        return [];
    }

    $profiles = storageGetRequestedProfiles();
    $results = [];

    foreach ($profiles as $profileKey => $profile) {
        if (!is_array($profile)) {
            continue;
        }

        $width = max(50, (int)($profile['width'] ?? 0));
        $height = max(50, (int)($profile['height'] ?? 0));
        $type = normalizeImageType((string)($profile['type'] ?? 'banner'));
        $keyword = trim((string)($profile['keyword'] ?? ''));

        if (($profile['last_daily_topup_on'] ?? null) === $today) {
            $results[] = [
                'profile' => $profileKey,
                'type' => $type,
                'width' => $width,
                'height' => $height,
                'saved' => 0,
                'target' => 0,
                'skipped' => 'already_topped_up_today',
            ];
            continue;
        }

        $results[] = refreshRequestedProfileImages(
            $width,
            $height,
            $type,
            $keyword,
            $dailyIncrement,
            $maxImagesPerType,
            'daily'
        );
    }

    flock($lock, LOCK_UN);
    fclose($lock);

    return $results;
}

function countOriginalImagesForType(string $type): int
{
    $stats = scanSupportedImagesInDirectory(getTypeOriginalDir($type));
    return $stats['count'];
}

function countRenderedImagesForType(string $type): int
{
    $stats = scanSupportedImagesInDirectory(getImagesRoot() . '/' . normalizeImageType($type));
    return $stats['count'];
}

function countRenderedImagesForProfile(string $type, int $width, int $height): int
{
    $dir = getImagesRoot() . '/' . normalizeImageType($type);
    if (!is_dir($dir)) {
        return 0;
    }

    $prefix = $width . 'x' . $height . '-';
    $count = 0;

    foreach (new DirectoryIterator($dir) as $entry) {
        if ($entry->isDot() || !$entry->isFile()) {
            continue;
        }
        if (isGeneratedFallbackImagePath($entry->getPathname())) {
            continue;
        }
        if (!isSupportedImageExtension($entry->getExtension())) {
            continue;
        }
        if (!str_starts_with($entry->getFilename(), $prefix)) {
            continue;
        }

        $count++;
    }

    return $count;
}

function collectOriginalImageSelectionsForType(string $type): array
{
    $imagesRoot = str_replace('\\', '/', getImagesRoot());
    $typeDir = getTypeOriginalDir($type);
    if (!is_dir($typeDir)) {
        return [];
    }

    $selections = [];
    $iterator = new RecursiveIteratorIterator(
        new RecursiveDirectoryIterator($typeDir, FilesystemIterator::SKIP_DOTS)
    );

    foreach ($iterator as $file) {
        if (!$file->isFile() || !isSupportedImageExtension($file->getExtension())) {
            continue;
        }

        $fullPath = str_replace('\\', '/', $file->getPathname());
        if (isGeneratedFallbackImagePath($fullPath) || !canDecodeRenderableSourceImage($fullPath)) {
            continue;
        }

        $relative = substr($fullPath, strlen($imagesRoot));
        if ($relative === false || $relative === '') {
            continue;
        }
        $relative = str_replace('\\', '/', $relative);
        if ($relative[0] !== '/') {
            $relative = '/' . ltrim($relative, '/');
        }
        $selections[] = $relative;
    }

    sort($selections);
    return array_values(array_unique($selections));
}

function warmRequestedProfileRenderedImages(int $width, int $height, string $type, array $formats = ['webp', 'avif']): array
{
    $type = normalizeImageType($type);
    $formats = array_values(array_filter(array_unique(array_map('strtolower', $formats)), static function (string $format): bool {
        return supportsOutputFormat($format);
    }));

    if ($formats === []) {
        return ['attempted' => 0, 'generated' => 0, 'formats' => []];
    }

    $imagesRoot = getImagesRoot();
    $selections = collectOriginalImageSelectionsForType($type);
    if ($selections === []) {
        return ['attempted' => 0, 'generated' => 0, 'formats' => $formats];
    }

    $attempted = 0;
    $generated = 0;

    foreach ($selections as $selected) {
        $sourceFile = $imagesRoot . $selected;
        if (!is_file($sourceFile)) {
            continue;
        }

        foreach ($formats as $format) {
            $attempted++;
            if (ensureProfileRenderedImage($imagesRoot, $type, $selected, $width, $height, $format, $sourceFile)) {
                $generated++;
            }
        }
    }

    return [
        'attempted' => $attempted,
        'generated' => $generated,
        'formats' => $formats,
    ];
}

function ensureProfileRenderedImage(
    string $imagesRoot,
    string $type,
    string $selected,
    int $width,
    int $height,
    string $format,
    string $sourceFile
): bool {
    $renderHash = sha1($selected . '|' . $width . 'x' . $height);
    $renderDir = "{$imagesRoot}/{$type}";
    $renderName = "{$width}x{$height}-{$renderHash}.{$format}";
    $renderFile = "{$renderDir}/{$renderName}";

    if (is_file($renderFile)) {
        return false;
    }

    if (!is_dir($renderDir) && !mkdir($renderDir, 0777, true) && !is_dir($renderDir)) {
        return false;
    }

    $renderLock = fopen($renderFile . '.lock', 'c+');
    if (!$renderLock || !flock($renderLock, LOCK_EX)) {
        if (is_resource($renderLock)) {
            fclose($renderLock);
        }
        return false;
    }

    $created = false;

    if (!is_file($renderFile)) {
        $image = loadRenderableImageResource($sourceFile);
        if ($image === false) {
            flock($renderLock, LOCK_UN);
            fclose($renderLock);
            return false;
        }

        $srcWidth = imagesx($image);
        $srcHeight = imagesy($image);
        $dst = imagecreatetruecolor($width, $height);
        if ($dst === false) {
            imagedestroy($image);
            flock($renderLock, LOCK_UN);
            fclose($renderLock);
            return false;
        }

        imagealphablending($dst, true);
        imagesavealpha($dst, true);

        $srcRatio = $srcWidth / $srcHeight;
        $dstRatio = $width / $height;
        if ($srcRatio > $dstRatio) {
            $cropHeight = $srcHeight;
            $cropWidth = (int)round($srcHeight * $dstRatio);
            $srcX = (int)floor(($srcWidth - $cropWidth) / 2);
            $srcY = 0;
        } else {
            $cropWidth = $srcWidth;
            $cropHeight = (int)round($srcWidth / $dstRatio);
            $srcX = 0;
            $srcY = (int)floor(($srcHeight - $cropHeight) / 2);
        }

        $ok = imagecopyresampled(
            $dst,
            $image,
            0,
            0,
            $srcX,
            $srcY,
            $width,
            $height,
            $cropWidth,
            $cropHeight
        );

        $writeError = null;
        if ($ok && writeRenderableImage($dst, $renderFile, $format, $writeError)) {
            $created = true;
            scheduleR2Upload($renderFile);
        } else {
            @unlink($renderFile);
        }

        imagedestroy($dst);
        imagedestroy($image);
    }

    flock($renderLock, LOCK_UN);
    fclose($renderLock);

    return $created;
}

function canDecodeRenderableSourceImage(string $file): bool
{
    $extension = strtolower(pathinfo($file, PATHINFO_EXTENSION));

    switch ($extension) {
        case 'jpg':
        case 'jpeg':
            return function_exists('imagecreatefromjpeg');
        case 'png':
            return function_exists('imagecreatefrompng');
        case 'gif':
            return function_exists('imagecreatefromgif');
        case 'webp':
            return function_exists('imagecreatefromwebp');
        case 'avif':
            return function_exists('imagecreatefromavif');
        default:
            return false;
    }
}

function loadRenderableImageResource(string $file)
{
    $info = @getimagesize($file);
    if (!is_array($info) || !isset($info['mime'])) {
        return false;
    }

    switch ($info['mime']) {
        case 'image/jpeg':
            return @imagecreatefromjpeg($file);
        case 'image/png':
            return @imagecreatefrompng($file);
        case 'image/gif':
            return @imagecreatefromgif($file);
        case 'image/webp':
            return @imagecreatefromwebp($file);
        case 'image/avif':
            return function_exists('imagecreatefromavif') ? @imagecreatefromavif($file) : false;
        default:
            return false;
    }
}

function writeRenderableImage($image, string $renderFile, string $format, ?string &$error = null): bool
{
    if ($format === 'avif') {
        // Prefer GD's imageavif — lighter, fewer external deps.
        // Imagick AVIF requires libheif/libaom delegate which is often missing.
        if (function_exists('imageavif')) {
            // libaom encoding at default speed is very slow on large images
            // (1080p can take 30-60s). Bump time limit and pick a faster
            // speed so requests don't trip max_execution_time.
            // speed: 0 = slowest/best, 10 = fastest. 6 keeps quality reasonable.
            @set_time_limit(120);
            if (@imageavif($image, $renderFile, 60, 6)) {
                return true;
            }
        }
        // Fallback to Imagick only if GD is unavailable or failed.
        if (class_exists('Imagick')) {
            $imagickError = null;
            if (writeRenderableAvifWithImagick($image, $renderFile, $imagickError)) {
                return true;
            }
            $error = $imagickError;
            return false;
        }
        $error = 'AVIF is not supported on this server';
        return false;
    }

    if (!@imagewebp($image, $renderFile, 85)) {
        $error = 'WebP encode failed on this server';
        return false;
    }

    return true;
}

function writeRenderableAvifWithImagick($image, string $renderFile, ?string &$error = null): bool
{
    try {
        ob_start();
        imagepng($image);
        $blob = ob_get_clean();
        if ($blob === false || $blob === '') {
            $error = 'AVIF intermediate image create failed';
            return false;
        }

        $imagick = new Imagick();
        $imagick->readImageBlob($blob);
        $imagick->setImageFormat('AVIF');
        $imagick->setImageCompressionQuality(60);
        $ok = $imagick->writeImage($renderFile);
        $imagick->clear();
        $imagick->destroy();

        if (!$ok || !is_file($renderFile)) {
            $error = 'AVIF encode failed on this server';
            return false;
        }

        return true;
    } catch (Throwable $e) {
        $error = 'AVIF encode failed on this server';
        return false;
    }
}

function primeRequestedProfile(
    int $width,
    int $height,
    string $type,
    string $keyword = '',
    int $initialIncrement = 3,
    int $maxImagesPerType = 1000
): array {
    if ($initialIncrement < 1 || $maxImagesPerType < 1 || !ensureDirectory(getImagesRoot())) {
        return ['saved' => 0, 'target' => 0, 'skipped' => 'invalid_config'];
    }

    $type = normalizeImageType($type);
    $keyword = trim($keyword);
    $profileKey = buildRequestProfileKey($width, $height, $type, $keyword);

    $lock = fopen(getRequestProfileLockFile(), 'c+');
    if (!$lock || !flock($lock, LOCK_EX)) {
        if (is_resource($lock)) {
            fclose($lock);
        }
        return ['saved' => 0, 'target' => 0, 'skipped' => 'lock_failed'];
    }

    $profile = storageGetRequestedProfile($profileKey);
    if (!is_array($profile)) {
        $profile = buildDefaultProfile($profileKey, $width, $height, $type, $keyword);
        storageSaveRequestedProfile($profile);
    }

    if (!empty($profile['initial_prefetch_done_at'])) {
        flock($lock, LOCK_UN);
        fclose($lock);
        return ['saved' => 0, 'target' => 0, 'skipped' => 'already_prefetched'];
    }

    $profile['initial_prefetch_done_at'] = gmdate(DATE_ATOM);
    storageSaveRequestedProfile($profile);

    flock($lock, LOCK_UN);
    fclose($lock);

    return refreshRequestedProfileImages($width, $height, $type, $keyword, $initialIncrement, $maxImagesPerType, 'initial');
}

function refreshRequestedProfileImages(
    int $width,
    int $height,
    string $type,
    string $keyword,
    int $requestedCount,
    int $maxImagesPerType = 1000,
    string $mode = 'manual'
): array {
    $type = normalizeImageType($type);
    $keyword = trim($keyword);
    $requestedCount = max(1, $requestedCount);
    $profileKey = buildRequestProfileKey($width, $height, $type, $keyword);

    $profile = storageGetRequestedProfile($profileKey);
    if (!is_array($profile)) {
        registerRequestedProfile($width, $height, $type, $keyword);
        $profile = storageGetRequestedProfile($profileKey);
    }
    if (!is_array($profile)) {
        $profile = buildDefaultProfile($profileKey, $width, $height, $type, $keyword);
    }

    $currentCount = countOriginalImagesForType($type);
    $now = gmdate(DATE_ATOM);
    $today = gmdate('Y-m-d');

    if ($currentCount >= $maxImagesPerType) {
        if ($mode === 'daily') {
            $profile['last_daily_topup_on'] = $today;
            $profile['last_daily_topup_saved'] = 0;
        }
        if (in_array($mode, ['manual', 'fresh'], true)) {
            $profile['last_manual_refresh_at'] = $now;
            $profile['last_manual_refresh_saved'] = 0;
        }
        if (empty($profile['initial_prefetch_done_at']) && in_array($mode, ['initial', 'manual', 'fresh'], true)) {
            $profile['initial_prefetch_done_at'] = $now;
        }
        storageSaveRequestedProfile($profile);
        storageAddRefreshLog([
            'profile_key' => $profileKey,
            'width' => $width,
            'height' => $height,
            'type' => $type,
            'keyword' => $keyword,
            'mode' => $mode,
            'requested_count' => 0,
            'saved_count' => 0,
            'error_text' => 'type_limit_reached',
            'created_at' => $now,
        ]);

        return [
            'profile' => $profileKey,
            'type' => $type,
            'width' => $width,
            'height' => $height,
            'saved' => 0,
            'target' => 0,
            'skipped' => 'type_limit_reached',
            'current_count' => $currentCount,
        ];
    }

    $target = min($requestedCount, $maxImagesPerType - $currentCount);
    $result = fetchImages($target, $width, $height, $type, $keyword);
    $saved = (int)($result['saved'] ?? 0);
    $error = trim((string)($result['error'] ?? ''));
    $warmResult = warmRequestedProfileRenderedImages($width, $height, $type, ['webp', 'avif']);

    if ($mode === 'daily') {
        $profile['last_daily_topup_on'] = $today;
        $profile['last_daily_topup_saved'] = $saved;
    }
    if (in_array($mode, ['manual', 'fresh'], true)) {
        $profile['last_manual_refresh_at'] = $now;
        $profile['last_manual_refresh_saved'] = $saved;
    }
    if (empty($profile['initial_prefetch_done_at']) && in_array($mode, ['initial', 'manual', 'fresh'], true)) {
        $profile['initial_prefetch_done_at'] = $now;
    }
    storageSaveRequestedProfile($profile);

    storageAddRefreshLog([
        'profile_key' => $profileKey,
        'width' => $width,
        'height' => $height,
        'type' => $type,
        'keyword' => $keyword,
        'mode' => $mode,
        'requested_count' => $target,
        'saved_count' => $saved,
        'error_text' => $error !== '' ? $error : null,
        'created_at' => $now,
    ]);

    return [
        'profile' => $profileKey,
        'type' => $type,
        'width' => $width,
        'height' => $height,
        'saved' => $saved,
        'target' => $target,
        'files' => $result['files'] ?? [],
        'error' => $error !== '' ? $error : null,
        'current_count' => countOriginalImagesForType($type),
        'render_attempted' => (int)($warmResult['attempted'] ?? 0),
        'render_generated' => (int)($warmResult['generated'] ?? 0),
        'render_formats' => (array)($warmResult['formats'] ?? []),
    ];
}

function scanSupportedImagesInDirectory(string $dir): array
{
    if (!is_dir($dir)) {
        return ['count' => 0, 'bytes' => 0];
    }

    $count = 0;
    $bytes = 0;
    $iterator = new RecursiveIteratorIterator(
        new RecursiveDirectoryIterator($dir, FilesystemIterator::SKIP_DOTS)
    );

    foreach ($iterator as $file) {
        if (!$file->isFile() || !isSupportedImageExtension($file->getExtension())) {
            continue;
        }
        if (isGeneratedFallbackImagePath($file->getPathname())) {
            continue;
        }

        $count++;
        $bytes += max(0, (int)$file->getSize());
    }

    return [
        'count' => $count,
        'bytes' => $bytes,
    ];
}

function getTypeImageStats(): array
{
    $types = [];
    $originalRoot = getOriginalRoot();
    $imagesRoot = getImagesRoot();

    if (is_dir($originalRoot)) {
        foreach (new DirectoryIterator($originalRoot) as $entry) {
            if ($entry->isDot() || !$entry->isDir()) {
                continue;
            }
            $types[$entry->getFilename()] = true;
        }
    }

    if (is_dir($imagesRoot)) {
        foreach (new DirectoryIterator($imagesRoot) as $entry) {
            if ($entry->isDot() || !$entry->isDir()) {
                continue;
            }
            $name = $entry->getFilename();
            if ($name === 'original') {
                continue;
            }
            $types[$name] = true;
        }
    }

    $profiles = storageGetRequestedProfiles();
    $profilesByType = [];
    $requestsByType = [];
    foreach ($profiles as $profile) {
        if (!is_array($profile)) {
            continue;
        }

        $type = normalizeImageType((string)($profile['type'] ?? 'banner'));
        $types[$type] = true;
        $profilesByType[$type] = ($profilesByType[$type] ?? 0) + 1;
        $requestsByType[$type] = ($requestsByType[$type] ?? 0) + (int)($profile['request_count'] ?? 0);
    }

    $rows = [];
    foreach (array_keys($types) as $type) {
        $original = scanSupportedImagesInDirectory($originalRoot . '/' . $type);
        $rendered = scanSupportedImagesInDirectory($imagesRoot . '/' . $type);
        $rows[] = [
            'type' => $type,
            'original_count' => $original['count'],
            'original_bytes' => $original['bytes'],
            'rendered_count' => $rendered['count'],
            'rendered_bytes' => $rendered['bytes'],
            'total_count' => $original['count'] + $rendered['count'],
            'total_bytes' => $original['bytes'] + $rendered['bytes'],
            'profile_count' => $profilesByType[$type] ?? 0,
            'request_count' => $requestsByType[$type] ?? 0,
        ];
    }

    usort($rows, static function (array $a, array $b): int {
        return [$b['total_count'], $a['type']] <=> [$a['total_count'], $b['type']];
    });

    return $rows;
}

function getLibrarySummary(): array
{
    $typeStats = getTypeImageStats();
    $profiles = storageGetRequestedProfiles();
    $dbSummary = storageGetDatabaseSummary();

    $summary = [
        'type_count' => count($typeStats),
        'profile_count' => count($profiles),
        'original_count' => 0,
        'rendered_count' => 0,
        'total_count' => 0,
        'original_bytes' => 0,
        'rendered_bytes' => 0,
        'total_bytes' => 0,
        'db' => $dbSummary,
    ];

    foreach ($typeStats as $row) {
        $summary['original_count'] += (int)$row['original_count'];
        $summary['rendered_count'] += (int)$row['rendered_count'];
        $summary['total_count'] += (int)$row['total_count'];
        $summary['original_bytes'] += (int)$row['original_bytes'];
        $summary['rendered_bytes'] += (int)$row['rendered_bytes'];
        $summary['total_bytes'] += (int)$row['total_bytes'];
    }

    return $summary;
}
