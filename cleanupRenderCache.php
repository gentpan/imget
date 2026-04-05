<?php

declare(strict_types=1);

require_once __DIR__ . '/imageLibrary.php';

if (PHP_SAPI !== 'cli') {
    fwrite(STDERR, "This script must be run from CLI.\n");
    exit(1);
}

$options = getopt('', ['hours::', 'dry-run']);
$ttlHours = isset($options['hours']) ? max(1, (int)$options['hours']) : getRenderCacheTtlHours();
$dryRun = array_key_exists('dry-run', $options);

$imagesRoot = getImagesRoot();
$originalRoot = str_replace('\\', '/', getOriginalRoot());
$cutoff = time() - ($ttlHours * 3600);

if (!is_dir($imagesRoot)) {
    fwrite(STDERR, "Images directory not found: {$imagesRoot}\n");
    exit(1);
}

$checked = 0;
$deleted = 0;
$skipped = 0;
$bytesFreed = 0;

$iterator = new RecursiveIteratorIterator(
    new RecursiveDirectoryIterator($imagesRoot, FilesystemIterator::SKIP_DOTS)
);

foreach ($iterator as $file) {
    if (!$file->isFile()) {
        continue;
    }

    $fullPath = str_replace('\\', '/', $file->getPathname());
    if (strpos($fullPath, $originalRoot . '/') === 0) {
        continue;
    }

    $extension = strtolower($file->getExtension());
    if (!in_array($extension, ['jpg', 'jpeg', 'png', 'gif', 'webp', 'avif'], true)) {
        continue;
    }

    if (str_ends_with($fullPath, '.lock')) {
        continue;
    }

    $checked++;

    if ($file->getMTime() > $cutoff) {
        $skipped++;
        continue;
    }

    $relativePath = r2RelativePath($fullPath);
    $cdnUrl = r2GetCdnUrl($relativePath);
    if ($cdnUrl === null || $cdnUrl === '') {
        $skipped++;
        continue;
    }

    $fileSize = (int)($file->getSize() ?: 0);
    if (!$dryRun && !@unlink($fullPath)) {
        $skipped++;
        continue;
    }

    $lockFile = $fullPath . '.lock';
    if (!$dryRun && is_file($lockFile)) {
        @unlink($lockFile);
    }

    $deleted++;
    $bytesFreed += $fileSize;
}

echo sprintf(
    "[cleanupRenderCache] checked=%d deleted=%d skipped=%d freed=%dB mode=%s ttl=%dh\n",
    $checked,
    $deleted,
    $skipped,
    $bytesFreed,
    $dryRun ? 'dry-run' : 'delete',
    $ttlHours
);

function getRenderCacheTtlHours(): int
{
    $value = (int)(envValue('LOCAL_RENDER_CACHE_TTL_HOURS', '168') ?: '168');
    return max(1, $value);
}
