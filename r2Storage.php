<?php

declare(strict_types=1);

use Aws\S3\S3Client;

/**
 * Cloudflare R2 storage module for img.et
 *
 * All R2 operations are guarded by r2IsConfigured().
 * When R2 env vars are missing, the system degrades to local-only.
 */

function r2IsConfigured(): bool
{
    static $configured = null;

    if ($configured !== null) {
        return $configured;
    }

    $endpoint = (string)(envValue('R2_ENDPOINT', '') ?: '');
    $accessKey = (string)(envValue('R2_ACCESS_KEY_ID', '') ?: '');
    $secretKey = (string)(envValue('R2_SECRET_ACCESS_KEY', '') ?: '');
    $bucket = (string)(envValue('R2_BUCKET_NAME', '') ?: '');

    $configured = ($endpoint !== '' && $accessKey !== '' && $secretKey !== '' && $bucket !== '');
    return $configured;
}

function getR2Client(): S3Client
{
    static $client = null;

    if ($client instanceof S3Client) {
        return $client;
    }

    $client = new S3Client([
        'region' => 'auto',
        'version' => 'latest',
        'endpoint' => (string)(envValue('R2_ENDPOINT', '') ?: ''),
        'use_path_style_endpoint' => true,
        'credentials' => [
            'key' => (string)(envValue('R2_ACCESS_KEY_ID', '') ?: ''),
            'secret' => (string)(envValue('R2_SECRET_ACCESS_KEY', '') ?: ''),
        ],
    ]);

    return $client;
}

function r2GetBucketName(): string
{
    return (string)(envValue('R2_BUCKET_NAME', 'imget-r2') ?: 'imget-r2');
}

function r2GetCdnBaseUrl(): string
{
    $url = (string)(envValue('R2_CDN_BASE_URL', 'https://pic.bluecdn.com') ?: 'https://pic.bluecdn.com');
    return rtrim($url, '/');
}

/**
 * Convert a local file path to R2 object key.
 * e.g. /path/to/imget/images/original/landscape/hash.jpg → original/landscape/hash.jpg
 * e.g. /path/to/imget/images/landscape/1920x1080-hash.webp → landscape/1920x1080-hash.webp
 */
function r2RelativePath(string $localFile): string
{
    $imagesRoot = str_replace('\\', '/', getImagesRoot());
    $normalized = str_replace('\\', '/', $localFile);

    if (strpos($normalized, $imagesRoot . '/') === 0) {
        return ltrim(substr($normalized, strlen($imagesRoot)), '/');
    }

    return basename($normalized);
}

/**
 * Build full CDN URL for a local file.
 */
function r2BuildCdnUrl(string $localFile): string
{
    return r2GetCdnBaseUrl() . '/' . r2RelativePath($localFile);
}

/**
 * Upload a single file to R2.
 */
function r2Upload(string $localFile): bool
{
    if (!r2IsConfigured() || !is_file($localFile) || !is_readable($localFile)) {
        return false;
    }

    $key = r2RelativePath($localFile);
    $mime = detectImageMime($localFile);
    if ($mime === '') {
        $mime = 'application/octet-stream';
    }

    try {
        $client = getR2Client();
        $bucket = r2GetBucketName();
        $client->putObject([
            'Bucket' => $bucket,
            'Key' => $key,
            'SourceFile' => $localFile,
            'ContentType' => $mime,
            'CacheControl' => 'public, max-age=31536000, immutable',
        ]);

        // Verify upload succeeded
        $head = $client->headObject([
            'Bucket' => $bucket,
            'Key' => $key,
        ]);
        $remoteSize = (int)($head['ContentLength'] ?? 0);
        $localSize = (int)(filesize($localFile) ?: 0);
        if ($remoteSize < 1 || ($localSize > 0 && abs($remoteSize - $localSize) > 1024)) {
            error_log('r2Upload verify failed: size mismatch remote=' . $remoteSize . ' local=' . $localSize . ' file=' . $localFile);
            return false;
        }

        return true;
    } catch (Throwable $e) {
        error_log('r2Upload failed: ' . $e->getMessage() . ' file=' . $localFile);
        return false;
    }
}

/**
 * Upload multiple files to R2. Returns summary.
 */
function r2UploadBatch(array $localFiles): array
{
    $uploaded = 0;
    $failed = 0;

    foreach ($localFiles as $file) {
        if (!is_string($file) || $file === '') {
            continue;
        }
        if (r2Upload($file)) {
            $relativePath = r2RelativePath($file);
            $cdnUrl = r2BuildCdnUrl($file);
            $fileSize = is_file($file) ? (int)(filesize($file) ?: 0) : 0;
            r2MarkUploaded($relativePath, $cdnUrl, $fileSize);
            $uploaded++;
        } else {
            $failed++;
        }
    }

    return ['uploaded' => $uploaded, 'failed' => $failed];
}

/**
 * Schedule async R2 upload via shutdown function (non-blocking).
 * Reuses the same pattern as scheduleBackgroundTopUp().
 */
function scheduleR2Upload(string $localFile): void
{
    if (!r2IsConfigured()) {
        return;
    }

    static $queued = [];

    $key = $localFile;
    if (isset($queued[$key])) {
        return;
    }
    $queued[$key] = true;

    register_shutdown_function(static function () use ($localFile): void {
        ignore_user_abort(true);

        if (function_exists('fastcgi_finish_request')) {
            fastcgi_finish_request();
        }

        if (!r2IsConfigured() || !is_file($localFile)) {
            return;
        }

        $ok = r2Upload($localFile);
        if ($ok) {
            $relativePath = r2RelativePath($localFile);
            $cdnUrl = r2BuildCdnUrl($localFile);
            $fileSize = (int)(filesize($localFile) ?: 0);
            r2MarkUploaded($relativePath, $cdnUrl, $fileSize);
        }
    });
}

/**
 * Quick HEAD check to verify a CDN URL is actually accessible (200).
 * Uses a short timeout to avoid blocking requests.
 */
function r2VerifyCdnUrl(string $cdnUrl): bool
{
    $ch = curl_init($cdnUrl);
    curl_setopt_array($ch, [
        CURLOPT_NOBODY => true,
        CURLOPT_FOLLOWLOCATION => true,
        CURLOPT_TIMEOUT => 3,
        CURLOPT_CONNECTTIMEOUT => 2,
        CURLOPT_RETURNTRANSFER => true,
    ]);
    curl_exec($ch);
    $httpCode = (int)curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);

    return $httpCode >= 200 && $httpCode < 400;
}

/**
 * Remove a stale upload record (CDN file missing).
 */
function r2RemoveUploadRecord(string $relativePath): void
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare('DELETE FROM r2_uploads WHERE file_path = :file_path');
    $stmt->execute(['file_path' => $relativePath]);
}

// ── Database helpers for r2_uploads table ──

function r2InitializeSchema(PDO $pdo, string $driver): void
{
    $sql = <<<SQL
CREATE TABLE IF NOT EXISTS r2_uploads (
    file_path TEXT PRIMARY KEY,
    r2_key TEXT NOT NULL,
    cdn_url TEXT NOT NULL,
    uploaded_at VARCHAR(35) NOT NULL,
    file_size INTEGER NOT NULL DEFAULT 0
)
SQL;

    $pdo->exec($sql);
}

/**
 * Mark a file as uploaded to R2.
 */
function r2MarkUploaded(string $relativePath, string $cdnUrl, int $fileSize = 0): void
{
    $pdo = getDatabase();
    $now = gmdate(DATE_ATOM);

    $driver = getDatabaseDriver();
    if ($driver === 'mysql') {
        $stmt = $pdo->prepare(
            'INSERT INTO r2_uploads (file_path, r2_key, cdn_url, uploaded_at, file_size)
             VALUES (:file_path, :r2_key, :cdn_url, :uploaded_at, :file_size)
             ON DUPLICATE KEY UPDATE cdn_url = VALUES(cdn_url), uploaded_at = VALUES(uploaded_at), file_size = VALUES(file_size)'
        );
    } else {
        $stmt = $pdo->prepare(
            'INSERT INTO r2_uploads (file_path, r2_key, cdn_url, uploaded_at, file_size)
             VALUES (:file_path, :r2_key, :cdn_url, :uploaded_at, :file_size)
             ON CONFLICT(file_path) DO UPDATE SET cdn_url = excluded.cdn_url, uploaded_at = excluded.uploaded_at, file_size = excluded.file_size'
        );
    }

    $stmt->execute([
        'file_path' => $relativePath,
        'r2_key' => $relativePath,
        'cdn_url' => $cdnUrl,
        'uploaded_at' => $now,
        'file_size' => $fileSize,
    ]);
}

/**
 * Get CDN URL if file has been uploaded to R2.
 * Returns null if not yet uploaded (hot path, called on every request).
 */
function r2GetCdnUrl(string $relativePath): ?string
{
    $pdo = getDatabase();
    $stmt = $pdo->prepare('SELECT cdn_url FROM r2_uploads WHERE file_path = :file_path');
    $stmt->execute(['file_path' => $relativePath]);
    $value = $stmt->fetchColumn();
    return $value === false ? null : (string)$value;
}

/**
 * Get upload stats.
 */
function r2GetUploadStats(): array
{
    $pdo = getDatabase();
    $row = $pdo->query('SELECT COUNT(*) AS total, COALESCE(SUM(file_size), 0) AS total_bytes FROM r2_uploads')->fetch();
    return [
        'total' => (int)($row['total'] ?? 0),
        'total_bytes' => (int)($row['total_bytes'] ?? 0),
    ];
}

/**
 * Scan local images directory and upload all files not yet in r2_uploads.
 * Used by CLI scripts (dailyTopUp, r2Sync) for batch catch-up.
 */
function r2UploadAllPending(): array
{
    if (!r2IsConfigured()) {
        return ['uploaded' => 0, 'failed' => 0, 'skipped' => 0, 'error' => 'R2 not configured'];
    }

    $imagesRoot = getImagesRoot();
    if (!is_dir($imagesRoot)) {
        return ['uploaded' => 0, 'failed' => 0, 'skipped' => 0, 'error' => 'Images directory not found'];
    }

    $uploaded = 0;
    $failed = 0;
    $skipped = 0;

    $iterator = new RecursiveIteratorIterator(
        new RecursiveDirectoryIterator($imagesRoot, FilesystemIterator::SKIP_DOTS)
    );

    foreach ($iterator as $file) {
        if (!$file->isFile()) {
            continue;
        }

        $ext = strtolower($file->getExtension());
        if (!in_array($ext, ['jpg', 'jpeg', 'png', 'gif', 'webp', 'avif'], true)) {
            continue;
        }

        // Skip lock files
        if (str_ends_with($file->getFilename(), '.lock')) {
            continue;
        }

        $fullPath = str_replace('\\', '/', $file->getPathname());
        $relativePath = r2RelativePath($fullPath);

        // Check if already uploaded
        $existing = r2GetCdnUrl($relativePath);
        if ($existing !== null) {
            $skipped++;
            continue;
        }

        if (r2Upload($fullPath)) {
            $cdnUrl = r2BuildCdnUrl($fullPath);
            $fileSize = (int)(filesize($fullPath) ?: 0);
            r2MarkUploaded($relativePath, $cdnUrl, $fileSize);
            $uploaded++;
        } else {
            $failed++;
        }
    }

    return ['uploaded' => $uploaded, 'failed' => $failed, 'skipped' => $skipped];
}
