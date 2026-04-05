<?php
declare(strict_types=1);

require_once __DIR__ . '/imageLibrary.php';

/**
 * Usage:
 *   php dailyTopUp.php [daily_increment] [max_per_type]
 *
 * Example crontab:
 *   0 12 * * * /usr/bin/php /path/to/imget/dailyTopUp.php 10 1000
 */

$dailyIncrement = isset($argv[1]) ? max(1, (int)$argv[1]) : 10;
$maxPerType = isset($argv[2]) ? max(10, (int)$argv[2]) : 1000;

$results = runDailyTopUp($dailyIncrement, $maxPerType);

if ($results === []) {
    echo "No requested profiles found." . PHP_EOL;
    exit(0);
}

$exitCode = 0;

foreach ($results as $result) {
    $profile = (string)($result['profile'] ?? 'unknown');
    $type = (string)($result['type'] ?? 'banner');
    $size = (string)($result['width'] ?? 0) . 'x' . (string)($result['height'] ?? 0);
    $saved = (int)($result['saved'] ?? 0);
    $target = (int)($result['target'] ?? 0);
    $currentCount = (int)($result['current_count'] ?? 0);
    $renderGenerated = (int)($result['render_generated'] ?? 0);
    $renderAttempted = (int)($result['render_attempted'] ?? 0);
    $renderFormats = implode(',', (array)($result['render_formats'] ?? []));
    $skipped = (string)($result['skipped'] ?? '');
    $error = trim((string)($result['error'] ?? ''));

    if ($skipped !== '') {
        echo "[skip] {$profile} {$size} type={$type} reason={$skipped} current={$currentCount}" . PHP_EOL;
        continue;
    }

    echo "[run] {$profile} {$size} type={$type} saved={$saved}/{$target} current={$currentCount} rendered={$renderGenerated}/{$renderAttempted}" . ($renderFormats !== '' ? " formats={$renderFormats}" : '') . PHP_EOL;
    if ($error !== '') {
        fwrite(STDERR, "[warn] {$profile} {$error}" . PHP_EOL);
        $exitCode = 2;
    }
}

// Sync new images to R2
if (r2IsConfigured()) {
    echo PHP_EOL . "[r2] Syncing pending images to R2..." . PHP_EOL;
    $r2Result = r2UploadAllPending();
    echo "[r2] uploaded={$r2Result['uploaded']} failed={$r2Result['failed']} skipped={$r2Result['skipped']}" . PHP_EOL;
}

exit($exitCode);
