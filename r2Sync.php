<?php
declare(strict_types=1);

require_once __DIR__ . '/imageLibrary.php';

/**
 * Standalone R2 sync script.
 * Scans all local images and uploads any that are not yet in R2.
 *
 * Usage:
 *   php r2Sync.php
 *
 * Example crontab (run every 6 hours as a catch-up sweep):
 *   0 0,6,12,18 * * * /usr/bin/php /path/to/imget/r2Sync.php
 */

if (!r2IsConfigured()) {
    fwrite(STDERR, "R2 is not configured. Set R2_ENDPOINT, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET_NAME in .env" . PHP_EOL);
    exit(1);
}

echo "Starting R2 sync..." . PHP_EOL;

$startTime = microtime(true);
$result = r2UploadAllPending();
$elapsed = round(microtime(true) - $startTime, 2);

$stats = r2GetUploadStats();

echo "[r2] uploaded={$result['uploaded']} failed={$result['failed']} skipped={$result['skipped']} time={$elapsed}s" . PHP_EOL;
echo "[r2] total in R2: {$stats['total']} files" . PHP_EOL;

if ($result['failed'] > 0) {
    exit(2);
}

exit(0);
