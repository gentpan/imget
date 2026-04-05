<?php
declare(strict_types=1);

require_once __DIR__ . '/imageLibrary.php';

/**
 * Usage:
 *   php weeklyImageFetch.php [count] [width] [height] [type] [keyword]
 */

$targetCount = isset($argv[1]) ? max(1, (int)$argv[1]) : 200;
$width = isset($argv[2]) ? max(320, (int)$argv[2]) : 1920;
$height = isset($argv[3]) ? max(180, (int)$argv[3]) : 1080;
$type = (string)($argv[4] ?? 'banner');
$keyword = trim((string)($argv[5] ?? ''));

$result = fetchImages($targetCount, $width, $height, $type, $keyword);

if (!empty($result['files']) && is_array($result['files'])) {
    foreach ($result['files'] as $file) {
        echo $file . PHP_EOL;
    }
}

if (!empty($result['error'])) {
    fwrite(STDERR, $result['error'] . PHP_EOL);
}

if (($result['saved'] ?? 0) < $targetCount) {
    exit(2);
}

echo "Completed: {$result['saved']} images stored in {$result['target_dir']} ({$width}x{$height}, type={$type})" . PHP_EOL;

// Sync to R2
if (r2IsConfigured() && !empty($result['files']) && is_array($result['files'])) {
    echo "[r2] Uploading {$result['saved']} new images to R2..." . PHP_EOL;
    $r2Result = r2UploadBatch($result['files']);
    echo "[r2] uploaded={$r2Result['uploaded']} failed={$r2Result['failed']}" . PHP_EOL;
}

exit(0);
