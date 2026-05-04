<?php

declare(strict_types=1);

ini_set('display_errors', '0');
ini_set('html_errors', '0');
error_reporting(E_ALL & ~E_DEPRECATED & ~E_USER_DEPRECATED);

require_once __DIR__ . '/imageLibrary.php';

const INITIAL_PREFETCH_COUNT = 5;
const MAX_IMAGES_PER_TYPE = 1000;

/**
 * img.et image router
 * - Selects one local source image per request key.
 * - Renders exact width/height variants on demand and caches them.
 * - Supports image pools by `type` query string.
 */

header('Access-Control-Allow-Origin: *');

$imagesRoot = __DIR__ . '/images';
$originalRoot = $imagesRoot . '/original';

if (!is_dir($imagesRoot) && !@mkdir($imagesRoot, 0777, true) && !is_dir($imagesRoot)) {
    renderRequestErrorPage(503, '图片目录不可用', '服务器图片目录当前不可写或不存在，请稍后再试。');
}

$requestUri = $_SERVER['REQUEST_URI'] ?? '/';
$path = parse_url($requestUri, PHP_URL_PATH) ?: '/';
parse_str(parse_url($requestUri, PHP_URL_QUERY) ?? '', $query);

if (
    preg_match('#^/p/files/([a-z0-9_-]+)/([A-Za-z0-9_-]+\.[A-Za-z0-9]+)$#', $path, $detailMatches) ||
    preg_match('#^/p/([a-z0-9_-]+)/([A-Za-z0-9_-]+\.[A-Za-z0-9]+)$#', $path, $detailMatches)
) {
    $detailMode = str_starts_with($path, '/p/files/') ? 'file' : 'render';
    $detailType = normalizeRouteType($detailMatches[1]);
    $detailName = basename($detailMatches[2]);
    $detailRelativePath = buildRelativeImagePath($detailMode, $detailType, $detailName);
    $detailFile = buildLocalImagePath($detailMode, $detailType, $detailName);
    $detailCdnUrl = r2GetCdnUrl($detailRelativePath);

    if ((!is_file($detailFile) || !is_readable($detailFile)) && $detailCdnUrl === null) {
        renderRequestErrorPage(404, '图片不存在', '当前请求的图片文件不存在，或暂时无法读取。');
    }

    renderPublicImagePage($detailMode, $detailType, $detailName, $detailFile, $_SERVER, $detailCdnUrl);
    exit;
}

if (
    preg_match('#^/files/([a-z0-9_-]+)/([A-Za-z0-9_-]+\.[A-Za-z0-9]+)$#', $path, $publicMatches) ||
    preg_match('#^/([a-z0-9_-]+)/([A-Za-z0-9_-]+\.[A-Za-z0-9]+)$#', $path, $publicMatches)
) {
    $publicMode = str_starts_with($path, '/files/') ? 'file' : 'render';
    $publicType = normalizeRouteType($publicMatches[1]);
    $publicName = basename($publicMatches[2]);
    $publicRelativePath = buildRelativeImagePath($publicMode, $publicType, $publicName);
    $publicFile = buildLocalImagePath($publicMode, $publicType, $publicName);
    $publicCdnUrl = r2GetCdnUrl($publicRelativePath);

    if ($publicCdnUrl !== null && ($query['download'] ?? '') !== '1') {
        header('Location: ' . $publicCdnUrl, true, 302);
        exit;
    }

    if ((!is_file($publicFile) || !is_readable($publicFile)) && $publicCdnUrl === null) {
        renderRequestErrorPage(404, '图片不存在', '当前请求的图片文件不存在，或暂时无法读取。');
    }

    $publicDownload = ($query['download'] ?? '') === '1';
    if ($publicCdnUrl !== null && (!is_file($publicFile) || !is_readable($publicFile))) {
        header('Location: ' . $publicCdnUrl, true, 302);
        exit;
    }

    sendImageResponse($publicFile, 86400, $publicDownload, $publicName);
    exit;
}

if (!preg_match('#^/(\d{2,4})/(\d{2,4})/?$#', $path, $matches)) {
    renderRequestErrorPage(400, '请求地址格式不正确', '请使用 `/宽/高` 的地址格式，例如 `https://img.et/1920/1080?type=landscape`。');
}

$width = (int)$matches[1];
$height = (int)$matches[2];
if ($width < 50 || $height < 50 || $width > 4000 || $height > 4000) {
    renderRequestErrorPage(
        400,
        '图片尺寸超出允许范围',
        "当前请求尺寸为 {$width}x{$height}。本站目前允许的单边范围是 50 到 4000 像素。4K 常见分辨率是 3840x2160，仍在允许范围内。"
    );
}

$variant = (string)($query['r'] ?? $query['v'] ?? '');
$slot = (string)($query['s'] ?? $query['slot'] ?? $query['slot_id'] ?? '');
$type = strtolower((string)($query['type'] ?? $query['t'] ?? 'banner'));
$type = preg_replace('/[^a-z0-9_-]+/', '', $type) ?? '';
$type = $type !== '' ? $type : 'banner';
$keyword = trim((string)($query['keyword'] ?? $query['q'] ?? ''));
$format = strtolower((string)($query['format'] ?? $query['f'] ?? 'webp'));
$format = in_array($format, ['webp', 'avif'], true) ? $format : 'webp';
if (!supportsOutputFormat($format)) {
    $format = 'webp';
}
$forceRaw = ($query['raw'] ?? '') === '1';
$forceDownload = ($query['download'] ?? '') === '1';
$freshCount = isset($query['fresh']) ? max(0, min(MAX_IMAGES_PER_TYPE, (int)$query['fresh'])) : 0;
$trackEvent = trim((string)($query['event'] ?? ''));
$isTrackRequest = ($query['__track'] ?? '') === '1';
$shouldPersistMapping = ($variant !== '');

if ($isTrackRequest) {
    header('Content-Type: application/json; charset=UTF-8');
    $metric = $trackEvent === 'download' ? 'download_count' : ($trackEvent === 'view' ? 'view_count' : '');
    if ($metric === '') {
        http_response_code(400);
        echo json_encode(['ok' => false, 'error' => 'invalid_event']);
        exit;
    }

    $tracked = incrementRequestedProfileMetric($width, $height, $type, $keyword, $metric);
    echo json_encode($tracked);
    exit;
}

$requestProfile = registerRequestedProfile($width, $height, $type, $keyword);
$selected = null;
$sourceFile = null;

if ($freshCount > 0) {
    $freshResult = refreshRequestedProfileImages($width, $height, $type, $keyword, $freshCount, MAX_IMAGES_PER_TYPE, 'fresh');
    $selected = selectFreshPathFromRefreshResult($freshResult['files'] ?? []);
}

$cacheKey = $path . '|type=' . $type . '|r=' . $variant . '|s=' . $slot . '|keyword=' . sha1($keyword);

if (!is_dir($originalRoot)) {
    @mkdir($originalRoot, 0777, true);
}
if (!is_dir($imagesRoot)) {
    @mkdir($imagesRoot, 0777, true);
}

if ($shouldPersistMapping && $selected === null) {
    $candidate = storageGetUrlCache($cacheKey);
    if ($candidate !== null) {
        $candidate = (string)$candidate;
    }
    if (is_file($imagesRoot . $candidate)) {
        $selected = $candidate;
    } elseif ($candidate !== null) {
        storageDeleteUrlCache($cacheKey);
    }
}

if ($selected === null) {
    $candidates = findSourceImages($imagesRoot, $originalRoot, $type);
    if ($candidates === []) {
        // Always try to synchronously fetch at least one image first,
        // so browser requests don't need multiple refreshes.
        ensureSourceImageAvailable($width, $height, $type, $keyword);
        $candidates = findSourceImages($imagesRoot, $originalRoot, $type);

        if ($candidates === []) {
            // Sync fetch failed — show preparing page for browser requests
            // or create a fallback for raw/API requests.
            if (
                !$forceRaw &&
                !$forceDownload &&
                shouldRenderImageDetailPage($_SERVER, $query)
            ) {
                $backgroundCount = (($requestProfile['is_new'] ?? false) === true) ? INITIAL_PREFETCH_COUNT : 1;
                $backgroundMode = (($requestProfile['is_new'] ?? false) === true) ? 'initial' : 'visit';
                scheduleBackgroundTopUp($width, $height, $type, $keyword, $backgroundCount, $backgroundMode);
                renderImagePreparingPage($path, $query, $width, $height, $type, $keyword);
                exit;
            }

            $sourceFile = createTemporaryFallbackImage($width, $height, $type, $keyword);
            if ($sourceFile === null || !is_file($sourceFile)) {
                renderRequestErrorPage(503, '分类图片暂时不可用', '当前分类还没有可用图片，且临时补图失败，请稍后重试。');
            }
            $selected = '/__generated__/' . basename($sourceFile);
        }
    }

    if ($selected === null) {
        $selected = $candidates[array_rand($candidates)];
    }
    if ($shouldPersistMapping && $sourceFile === null) {
        storagePutUrlCache($cacheKey, $selected);
    }
}

if ($shouldPersistMapping && $selected !== null && $freshCount > 0) {
    storagePutUrlCache($cacheKey, $selected);
}

$sourceFile = $sourceFile ?? ($imagesRoot . $selected);
if (!is_file($sourceFile)) {
    renderRequestErrorPage(404, '源图丢失', '选中的源图文件不存在，可能已被清理或移动，请刷新重试。');
}

$responseFile = $sourceFile;
$downloadName = buildDownloadName($width, $height, $type, $format, $sourceFile);
$cacheSeconds = getResponseCacheSeconds($shouldPersistMapping && $freshCount < 1);
$responseFile = ensureRenderedImage($imagesRoot, $type, $selected, $width, $height, $format, $sourceFile);
$downloadName = buildDownloadName($width, $height, $type, $format, $responseFile);

if (!$forceRaw && !$forceDownload && $freshCount < 1) {
    $backgroundCount = (($requestProfile['is_new'] ?? false) === true) ? INITIAL_PREFETCH_COUNT : 1;
    $backgroundMode = (($requestProfile['is_new'] ?? false) === true) ? 'initial' : 'visit';
    scheduleBackgroundTopUp($width, $height, $type, $keyword, $backgroundCount, $backgroundMode);
}

if (!$forceRaw && !$forceDownload && shouldRenderImageDetailPage($_SERVER, $query)) {
    if (isTemporaryGeneratedSelection($selected, $sourceFile)) {
        renderImagePreparingPage($path, $query, $width, $height, $type, $keyword);
        exit;
    }

    $trackedView = incrementRequestedProfileMetric($width, $height, $type, $keyword, 'view_count');
    if (($trackedView['ok'] ?? false) === true) {
        $requestProfile['view_count'] = (int)($trackedView['view_count'] ?? ($requestProfile['view_count'] ?? 0));
        $requestProfile['download_count'] = (int)($trackedView['download_count'] ?? ($requestProfile['download_count'] ?? 0));
    }

    renderImageDetailPage(
        $path,
        $query,
        $responseFile,
        $sourceFile,
        $width,
        $height,
        $type,
        $keyword,
        $format,
        $selected,
        $requestProfile
    );
    exit;
}

sendImageResponse($responseFile, $cacheSeconds, $forceDownload, $downloadName);
exit;

function ensureRenderedImage(
    string $imagesRoot,
    string $type,
    string $selected,
    int $width,
    int $height,
    string $format,
    string $sourceFile
): string {
    $renderHash = sha1($selected . '|' . $width . 'x' . $height);
    $renderDir = "{$imagesRoot}/{$type}";
    $renderName = "{$width}x{$height}-{$renderHash}.{$format}";
    $renderFile = "{$renderDir}/{$renderName}";

    if (is_file($renderFile)) {
        return $renderFile;
    }

    if (!is_dir($renderDir) && !mkdir($renderDir, 0777, true) && !is_dir($renderDir)) {
        http_response_code(500);
        exit('Render directory create failed');
    }

    $renderLock = fopen($renderFile . '.lock', 'c+');
    if (!$renderLock || !flock($renderLock, LOCK_EX)) {
        http_response_code(500);
        exit('Render lock failed');
    }

    if (!is_file($renderFile)) {
        $image = loadImageResource($sourceFile);
        if (!$image) {
            flock($renderLock, LOCK_UN);
            fclose($renderLock);
            http_response_code(500);
            exit('Source decode failed');
        }

        $srcWidth = imagesx($image);
        $srcHeight = imagesy($image);
        $dst = imagecreatetruecolor($width, $height);
        if (!$dst) {
            flock($renderLock, LOCK_UN);
            fclose($renderLock);
            http_response_code(500);
            exit('Render create failed');
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
        if (!$ok || !writeRenderedImage($dst, $renderFile, $format, $writeError)) {
            @unlink($renderFile);
            flock($renderLock, LOCK_UN);
            fclose($renderLock);
            http_response_code(500);
            exit($writeError ?? 'Render failed');
        }
    }

    flock($renderLock, LOCK_UN);
    fclose($renderLock);

    scheduleR2Upload($renderFile);

    return $renderFile;
}

function findSourceImages(string $imagesRoot, string $originalRoot, string $type): array
{
    $files = [];
    $fallbackFiles = [];
    $type = normalizeImageType($type);

    $typeDir = $originalRoot . '/' . $type;
    if (is_dir($typeDir)) {
        [$primary, $fallback] = collectWebpFiles($imagesRoot, $typeDir);
        $files = array_merge($files, $primary);
        $fallbackFiles = array_merge($fallbackFiles, $fallback);
    }

    if ($files !== []) {
        return $files;
    }

    // For explicit categories, never borrow images from other type pools.
    // This keeps source selection stable and ensures renders always start
    // from the requested type's original-image library.
    if ($type !== 'banner') {
        return $fallbackFiles;
    }

    if (is_dir($originalRoot)) {
        [$primary, $fallback] = collectWebpFiles($imagesRoot, $originalRoot);
        $files = array_merge($files, $primary);
        $fallbackFiles = array_merge($fallbackFiles, $fallback);
    }

    if ($files !== []) {
        return $files;
    }

    if ($fallbackFiles !== []) {
        return $fallbackFiles;
    }

    return [];
}

function collectWebpFiles(string $imagesRoot, string $root): array
{
    $files = [];
    $fallbackFiles = [];
    $it = new RecursiveIteratorIterator(
        new RecursiveDirectoryIterator($root, FilesystemIterator::SKIP_DOTS)
    );

    foreach ($it as $file) {
        if (!$file->isFile() || !isSupportedImageExtension($file->getExtension())) {
            continue;
        }
        $fullPath = $file->getPathname();
        if (!canDecodeSourceImage($fullPath)) {
            continue;
        }
        $relPath = substr($fullPath, strlen($imagesRoot));
        $relPath = str_replace('\\', '/', $relPath);
        if ($relPath === false || $relPath === '') {
            continue;
        }
        if ($relPath[0] !== '/') {
            $relPath = '/' . ltrim($relPath, '/');
        }
        if (isGeneratedFallbackImagePath($fullPath)) {
            $fallbackFiles[] = $relPath;
        } else {
            $files[] = $relPath;
        }
    }

    return [$files, $fallbackFiles];
}

function loadImageResource(string $file)
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

function writeRenderedImage($image, string $renderFile, string $format, ?string &$error = null): bool
{
    if ($format === 'avif') {
        if (class_exists('Imagick')) {
            return writeAvifWithImagick($image, $renderFile, $error);
        }
        if (function_exists('imageavif') && @imageavif($image, $renderFile, 60)) {
            return true;
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

function writeAvifWithImagick($image, string $renderFile, ?string &$error = null): bool
{
    try {
        ob_start();
        imagepng($image);
        $blob = ob_get_clean();
        if ($blob === false || $blob === '') {
            $error = 'AVIF intermediate image create failed';
            return false;
        }

        $imagickClass = 'Imagick';
        $imagick = new $imagickClass();
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

function canDecodeSourceImage(string $file): bool
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

function scheduleBackgroundTopUp(
    int $width,
    int $height,
    string $type,
    string $keyword,
    int $count = 1,
    string $mode = 'visit'
): void
{
    static $queued = [];

    $queueKey = implode('|', [$width, $height, $type, sha1($keyword), $count, $mode]);
    if (isset($queued[$queueKey])) {
        return;
    }
    $queued[$queueKey] = true;

    register_shutdown_function(static function () use ($width, $height, $type, $keyword, $count, $mode): void {
        ignore_user_abort(true);

        if (function_exists('fastcgi_finish_request')) {
            fastcgi_finish_request();
        }

        refreshRequestedProfileImages($width, $height, $type, $keyword, $count, MAX_IMAGES_PER_TYPE, $mode);
    });
}

function ensureSourceImageAvailable(int $width, int $height, string $type, string $keyword): void
{
    $result = fetchImages(1, $width, $height, $type, $keyword);
    if ((int)($result['saved'] ?? 0) > 0 || countOriginalImagesForType($type) > 0) {
        return;
    }

    createGeneratedFallbackImage($width, $height, $type, $keyword);
}

function selectFreshPathFromRefreshResult($savedFiles): ?string
{
    if (!is_array($savedFiles)) {
        return null;
    }

    $imagesRoot = str_replace('\\', '/', getImagesRoot());

    foreach ($savedFiles as $savedFile) {
        if (!is_string($savedFile) || $savedFile === '' || !is_file($savedFile)) {
            continue;
        }

        if (!canDecodeSourceImage($savedFile)) {
            continue;
        }

        $fullPath = str_replace('\\', '/', $savedFile);
        if (strpos($fullPath, $imagesRoot) !== 0) {
            continue;
        }

        $relPath = substr($fullPath, strlen($imagesRoot));
        if ($relPath === false || $relPath === '') {
            continue;
        }

        if ($relPath[0] !== '/') {
            $relPath = '/' . ltrim($relPath, '/');
        }

        return $relPath;
    }

    return null;
}

function isTemporaryGeneratedSelection(?string $selected, string $sourceFile): bool
{
    if (is_string($selected) && str_starts_with($selected, '/__generated__/')) {
        return true;
    }

    return isGeneratedFallbackImagePath($sourceFile);
}

function getResponseCacheSeconds(bool $shouldPersistMapping): int
{
    return $shouldPersistMapping ? 86400 : 0;
}

function shouldRenderImageDetailPage(array $server, array $query): bool
{
    if (($query['raw'] ?? '') === '1' || ($query['download'] ?? '') === '1') {
        return false;
    }

    if (
        ($query['preview'] ?? '') === '1' ||
        ($query['page'] ?? '') === '1' ||
        ($query['detail'] ?? '') === '1'
    ) {
        return true;
    }

    $fetchDest = strtolower((string)($server['HTTP_SEC_FETCH_DEST'] ?? ''));
    $accept = strtolower((string)($server['HTTP_ACCEPT'] ?? ''));

    if ($fetchDest === 'image') {
        return false;
    }

    if ($accept !== '' && !str_contains($accept, 'text/html') && str_contains($accept, 'image/')) {
        return false;
    }

    return true;
}

function buildDownloadName(int $width, int $height, string $type, string $format, string $file): string
{
    $extension = strtolower(pathinfo($file, PATHINFO_EXTENSION));
    if ($extension === '') {
        $extension = $format === 'original'
            ? (strtolower(pathinfo($file, PATHINFO_EXTENSION)) ?: 'jpg')
            : $format;
    }

    return "{$type}-{$width}x{$height}.{$extension}";
}

function buildRouteUrl(string $path, array $query, array $overrides = [], array $remove = []): string
{
    $params = $query;

    foreach ($remove as $key) {
        unset($params[$key]);
    }

    foreach ($overrides as $key => $value) {
        if ($value === null || $value === '') {
            unset($params[$key]);
            continue;
        }
        $params[$key] = (string)$value;
    }

    $queryString = http_build_query($params);
    return $queryString !== '' ? "{$path}?{$queryString}" : $path;
}

function getFileMeta(string $file): array
{
    $size = is_file($file) ? (int)(filesize($file) ?: 0) : 0;
    $mime = detectImageMime($file);
    $width = 0;
    $height = 0;
    $info = @getimagesize($file);
    if (is_array($info)) {
        $width = (int)($info[0] ?? 0);
        $height = (int)($info[1] ?? 0);
    }

    return [
        'size_bytes' => $size,
        'size_human' => formatBytesForPage($size),
        'mime' => $mime !== '' ? $mime : 'application/octet-stream',
        'width' => $width,
        'height' => $height,
        'extension' => strtolower(pathinfo($file, PATHINFO_EXTENSION)),
    ];
}

function formatBytesForPage(int $bytes): string
{
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $size = max(0, $bytes);
    $index = 0;

    while ($size >= 1024 && $index < count($units) - 1) {
        $size /= 1024;
        $index++;
    }

    return sprintf($index === 0 ? '%d %s' : '%.2f %s', $size, $units[$index]);
}

function formatImageFormatLabel(string $format): string
{
    switch (strtolower($format)) {
        case 'jpg':
        case 'jpeg':
            return 'JPEG';
        case 'png':
            return 'PNG';
        case 'gif':
            return 'GIF';
        case 'webp':
            return 'WebP';
        case 'avif':
            return 'AVIF';
        default:
            return strtoupper($format);
    }
}

function renderImageDetailPage(
    string $path,
    array $query,
    string $displayFile,
    string $sourceFile,
    int $requestedWidth,
    int $requestedHeight,
    string $type,
    string $keyword,
    string $format,
    string $selected,
    array $requestProfile
): void {
    header('Content-Type: text/html; charset=UTF-8');
    header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
    header('Vary: Accept, Sec-Fetch-Mode, Sec-Fetch-Dest, Sec-Fetch-User');

    $currentMeta = getFileMeta($displayFile);
    $sourceMeta = getFileMeta($sourceFile);
    $pageUrl = buildRouteUrl($path, $query, [], ['raw', 'download']);
    $rawImageUrl = buildRouteUrl($path, $query, ['raw' => '1'], ['download']);
    $downloadUrl = buildRouteUrl($path, $query, ['download' => '1'], ['raw']);
    $homeUrl = buildAbsoluteUrl($server = $_SERVER, '/');
    $pageUrlFull = buildPrimarySiteAbsoluteUrl($pageUrl);
    $directImageUrlFull = getDirectAccessUrlForFile($displayFile, $server, $pageUrl);
    $downloadUrlFull = buildPrimarySiteAbsoluteUrl($downloadUrl);
    $currentFormatLabel = formatImageFormatLabel($currentMeta['extension'] !== '' ? $currentMeta['extension'] : $format);
    $trackBaseUrl = buildRouteUrl($path, $query, ['__track' => '1'], ['raw', 'download']);
    $viewCount = (int)($requestProfile['view_count'] ?? 0);
    $downloadCount = (int)($requestProfile['download_count'] ?? 0);
    $generatedAtTimestamp = (int)(filemtime($displayFile) ?: time());
    $sourceLibraryCount = countOriginalImagesForType($type);
    $profileRenderCount = countRenderedImagesForProfile($type, $requestedWidth, $requestedHeight);
    $shortLinkUrl = $pageUrlFull;
    $imageFormats = [
        '短链' => $shortLinkUrl,
        '直链' => $directImageUrlFull,
        'HTML' => '<img src="' . $directImageUrlFull . '" alt="' . $type . '">',
        'HTML w/ Link' => '<a href="' . $pageUrlFull . '"><img src="' . $pageUrlFull . '" alt="' . $type . '"></a>',
        'Markdown' => '![' . $type . '](' . $directImageUrlFull . ')',
        'BBCode' => '[img]' . $directImageUrlFull . '[/img]',
        'BBCode w/ Link' => '[url=' . $pageUrlFull . '][img]' . $pageUrlFull . '[/img][/url]',
        'URL' => $pageUrlFull,
    ];
?>
    <!doctype html>
    <html lang="zh-CN">

    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title><?= htmlspecialchars("{$type} {$requestedWidth}x{$requestedHeight}", ENT_QUOTES, 'UTF-8') ?> - img.et</title>
        <script defer src="https://tongji.giantaccel.com/script.js" data-website-id="dfe72df0-719f-4ad2-af39-cfea2465dd44"></script>
        <link rel="stylesheet" href="/assets/main.min.css?v=a0737c5b">
    </head>

    <body class="page-image-detail">
        <main class="wrap" data-track-base-url="<?= htmlspecialchars($trackBaseUrl, ENT_QUOTES, 'UTF-8') ?>">
            <div class="topbar">
                <a class="brand" href="<?= htmlspecialchars($homeUrl, ENT_QUOTES, 'UTF-8') ?>">
                    <span class="brand-mark"><img src="/assets/logo.png" alt="img.et logo"></span>
                    <span>img.et</span>
                </a>
                <div class="meta-badges">
                    <span class="type-badge"><?= htmlspecialchars(getTypeChineseLabel($type), ENT_QUOTES, 'UTF-8') ?> <em><?= htmlspecialchars($type, ENT_QUOTES, 'UTF-8') ?></em></span>
                    <span class="meta-badge"><strong>图库数量</strong><span><?= htmlspecialchars((string)$sourceLibraryCount, ENT_QUOTES, 'UTF-8') ?> 张</span></span>
                    <span class="meta-badge"><strong>缓存数量</strong><span><?= htmlspecialchars((string)$profileRenderCount, ENT_QUOTES, 'UTF-8') ?> 张</span></span>
                </div>
            </div>

            <section class="hero">
                <div class="preview" view-image>
                    <img class="viewer-image" src="<?= htmlspecialchars($directImageUrlFull, ENT_QUOTES, 'UTF-8') ?>" alt="img.et preview" loading="eager">
                    <div class="stats">
                        <div class="stat"><strong>图片格式</strong><span><?= htmlspecialchars($currentFormatLabel, ENT_QUOTES, 'UTF-8') ?></span></div>
                        <div class="stat"><strong>图片尺寸</strong><span><?= htmlspecialchars("{$currentMeta['width']}x{$currentMeta['height']}", ENT_QUOTES, 'UTF-8') ?></span></div>
                        <div class="stat"><strong>生成时间</strong><span data-timestamp="<?= $generatedAtTimestamp ?>"></span></div>
                        <div class="stat"><strong>文件大小</strong><span><?= htmlspecialchars($currentMeta['size_human'], ENT_QUOTES, 'UTF-8') ?></span></div>
                        <div class="stat"><strong>页面浏览</strong><span id="view-count"><?= htmlspecialchars((string)$viewCount, ENT_QUOTES, 'UTF-8') ?></span></div>
                        <div class="stat"><strong>下载次数</strong><span id="download-count"><?= htmlspecialchars((string)$downloadCount, ENT_QUOTES, 'UTF-8') ?></span></div>
                    </div>
                </div>
            </section>

            <section class="panel">
                <table class="info-table">
                    <?php if (($query['r'] ?? '') !== ''): ?>
                        <tr>
                            <th>固定参数</th>
                            <td><?= htmlspecialchars((string)$query['r'], ENT_QUOTES, 'UTF-8') ?></td>
                        </tr>
                    <?php endif; ?>
                    <?php if (($query['s'] ?? '') !== ''): ?>
                        <tr>
                            <th>图片位</th>
                            <td><?= htmlspecialchars((string)$query['s'], ENT_QUOTES, 'UTF-8') ?></td>
                        </tr>
                    <?php endif; ?>
                    <?php if ($keyword !== ''): ?>
                        <tr>
                            <th>关键词</th>
                            <td><?= htmlspecialchars($keyword, ENT_QUOTES, 'UTF-8') ?></td>
                        </tr>
                    <?php endif; ?>
                </table>

                <div class="links-grid">
                    <div class="link-card">
                        <div class="link-card-head">
                            <h3>引用链接</h3>
                            <div class="icon-actions">
                                <button type="button" data-open="<?= htmlspecialchars($directImageUrlFull, ENT_QUOTES, 'UTF-8') ?>" aria-label="打开转换后图片地址" title="打开">
                                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                                        <path d="M14 4h6v6"></path>
                                        <path d="M10 14L20 4"></path>
                                        <path d="M20 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h4"></path>
                                    </svg>
                                </button>
                                <button type="button" data-download="<?= htmlspecialchars($downloadUrlFull, ENT_QUOTES, 'UTF-8') ?>" aria-label="下载转换后图片" title="下载">
                                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                                        <path d="M12 3v12"></path>
                                        <path d="M7 10l5 5 5-5"></path>
                                        <path d="M5 21h14"></path>
                                    </svg>
                                </button>
                            </div>
                        </div>
                        <div class="link-card-body">
                        <div class="format-list">
                            <?php foreach ($imageFormats as $formatLabel => $formatValue): ?>
                                <div class="format-row">
                                    <div class="format-label"><?= htmlspecialchars($formatLabel, ENT_QUOTES, 'UTF-8') ?></div>
                                    <div class="format-value mono"><?= htmlspecialchars($formatValue, ENT_QUOTES, 'UTF-8') ?></div>
                                    <button type="button" data-copy="<?= htmlspecialchars($formatValue, ENT_QUOTES, 'UTF-8') ?>" aria-label="复制<?= htmlspecialchars($formatLabel, ENT_QUOTES, 'UTF-8') ?>" title="复制 <?= htmlspecialchars($formatLabel, ENT_QUOTES, 'UTF-8') ?>">
                                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                                            <rect x="9" y="9" width="10" height="10"></rect>
                                            <path d="M5 15V5h10"></path>
                                        </svg>
                                    </button>
                                </div>
                            <?php endforeach; ?>
                        </div>
                        </div>
                    </div>
                </div>
            </section>
        </main>
        <script>
        document.querySelectorAll('[data-timestamp]').forEach(function(el){
            var ts = parseInt(el.getAttribute('data-timestamp'), 10);
            if (!ts) return;
            var d = new Date(ts * 1000);
            var y = d.getFullYear();
            var m = String(d.getMonth() + 1).padStart(2, '0');
            var day = String(d.getDate()).padStart(2, '0');
            var h = String(d.getHours()).padStart(2, '0');
            var min = String(d.getMinutes()).padStart(2, '0');
            el.textContent = y + '-' + m + '-' + day + ' ' + h + ':' + min;
        });
        </script>
        <script defer src="/assets/main.min.js"></script>
    </body>

    </html>
<?php
}

function renderImagePreparingPage(
    string $path,
    array $query,
    int $requestedWidth,
    int $requestedHeight,
    string $type,
    string $keyword
): void {
    header('Content-Type: text/html; charset=UTF-8');
    header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');

    $homeUrl = buildAbsoluteUrl($server = $_SERVER, '/');
    $pageUrl = buildRouteUrl($path, $query, [], ['raw', 'download']);
    $pageUrlFull = buildPrimarySiteAbsoluteUrl($pageUrl);
    $sourceLibraryCount = countOriginalImagesForType($type);
?>
    <!doctype html>
    <html lang="zh-CN">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title><?= htmlspecialchars("{$type} {$requestedWidth}x{$requestedHeight}", ENT_QUOTES, 'UTF-8') ?> - img.et</title>
        <link rel="stylesheet" href="/assets/main.min.css?v=a0737c5b">
        <style>
            body.page-image-detail .prepare-box{min-height:420px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:16px;text-align:center}
            body.page-image-detail .prepare-spinner{width:64px;height:64px;display:block;animation:imget-spin .85s linear infinite}
            body.page-image-detail .prepare-spinner circle{fill:none;stroke-width:4}
            body.page-image-detail .prepare-spinner .ring-bg{stroke:#d6dfef}
            body.page-image-detail .prepare-spinner .ring-accent{stroke:#0052d9;stroke-linecap:round;stroke-dasharray:92 140}
            body.page-image-detail .prepare-title{font-size:26px;font-weight:800;color:#142033}
            body.page-image-detail .prepare-copy{max-width:640px;color:#5a6d89;font-size:14px;line-height:1.8}
            body.page-image-detail .prepare-meta{display:inline-flex;gap:8px;flex-wrap:wrap;justify-content:center}
            @keyframes imget-spin{to{transform:rotate(360deg)}}
        </style>
    </head>
    <body class="page-image-detail">
        <main class="wrap">
            <div class="topbar">
                <a class="brand" href="<?= htmlspecialchars($homeUrl, ENT_QUOTES, 'UTF-8') ?>">
                    <span class="brand-mark"><img src="/assets/logo.png" alt="img.et logo"></span>
                    <span>img.et</span>
                </a>
                <div class="meta-badges">
                    <span class="type-badge"><?= htmlspecialchars(getTypeChineseLabel($type), ENT_QUOTES, 'UTF-8') ?> <em><?= htmlspecialchars($type, ENT_QUOTES, 'UTF-8') ?></em></span>
                    <span class="meta-badge"><strong>图库数量</strong><span><?= htmlspecialchars((string)$sourceLibraryCount, ENT_QUOTES, 'UTF-8') ?> 张</span></span>
                </div>
            </div>

            <section class="hero">
                <div class="prepare-box">
                    <svg class="prepare-spinner" viewBox="0 0 64 64" aria-hidden="true">
                        <circle class="ring-bg" cx="32" cy="32" r="24"></circle>
                        <circle class="ring-accent" cx="32" cy="32" r="24"></circle>
                    </svg>
                    <div class="prepare-title">正在准备图片</div>
                    <div class="prepare-copy">
                        该分类正在补充第一批图片，请稍候片刻，页面会自动刷新。
                        <?php if ($keyword !== ''): ?>
                            <br>关键词：<?= htmlspecialchars($keyword, ENT_QUOTES, 'UTF-8') ?>
                        <?php endif; ?>
                    </div>
                    <div class="prepare-meta">
                        <span class="meta-badge"><strong>分类</strong><span><?= htmlspecialchars($type, ENT_QUOTES, 'UTF-8') ?></span></span>
                        <span class="meta-badge"><strong>尺寸</strong><span><?= htmlspecialchars("{$requestedWidth}x{$requestedHeight}", ENT_QUOTES, 'UTF-8') ?></span></span>
                    </div>
                </div>
            </section>
        </main>
        <script>
            window.setTimeout(function () {
                window.location.replace(<?= json_encode($pageUrlFull, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) ?>);
            }, 2000);
        </script>
    </body>
    </html>
<?php
}

function renderRequestErrorPage(int $statusCode, string $title, string $detail): void
{
    http_response_code($statusCode);
    header('Content-Type: text/html; charset=UTF-8');
    header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');

    $homeUrl = buildAbsoluteUrl($server = $_SERVER, '/');
?>
    <!doctype html>
    <html lang="zh-CN">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title><?= htmlspecialchars($title, ENT_QUOTES, 'UTF-8') ?> - img.et</title>
        <link rel="stylesheet" href="/assets/main.min.css?v=a0737c5b">
        <style>
            body.page-image-detail .error-box{min-height:420px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:16px;text-align:center}
            body.page-image-detail .error-code{display:inline-flex;align-items:center;justify-content:center;min-width:88px;padding:10px 16px;border:1px solid #f1c7cd;background:#fff2f4;color:#c24153;font-size:28px;font-weight:800}
            body.page-image-detail .error-title{font-size:30px;font-weight:800;color:#142033}
            body.page-image-detail .error-copy{max-width:760px;color:#5a6d89;font-size:15px;line-height:1.9}
            body.page-image-detail .error-actions{display:flex;gap:10px;flex-wrap:wrap;justify-content:center}
            body.page-image-detail .error-action{display:inline-flex;align-items:center;justify-content:center;min-width:132px;padding:11px 18px;border:1px solid #c8d9f5;background:#eef4ff;color:#1f57a8;text-decoration:none;font-weight:700}
        </style>
    </head>
    <body class="page-image-detail">
        <main class="wrap">
            <div class="topbar">
                <a class="brand" href="<?= htmlspecialchars($homeUrl, ENT_QUOTES, 'UTF-8') ?>">
                    <span class="brand-mark"><img src="/assets/logo.png" alt="img.et logo"></span>
                    <span>img.et</span>
                </a>
            </div>

            <section class="hero">
                <div class="error-box">
                    <div class="error-code"><?= htmlspecialchars((string)$statusCode, ENT_QUOTES, 'UTF-8') ?></div>
                    <div class="error-title"><?= htmlspecialchars($title, ENT_QUOTES, 'UTF-8') ?></div>
                    <div class="error-copy"><?= htmlspecialchars($detail, ENT_QUOTES, 'UTF-8') ?></div>
                    <div class="error-actions">
                        <a class="error-action" href="<?= htmlspecialchars($homeUrl, ENT_QUOTES, 'UTF-8') ?>">返回首页</a>
                        <a class="error-action" href="<?= htmlspecialchars((string)($_SERVER['REQUEST_URI'] ?? '/'), ENT_QUOTES, 'UTF-8') ?>">重新尝试</a>
                    </div>
                </div>
            </section>
        </main>
    </body>
    </html>
<?php
    exit;
}

function renderPublicImagePage(
    string $mode,
    string $type,
    string $fileName,
    string $file,
    array $server,
    ?string $cdnUrl = null
): void {
    header('Content-Type: text/html; charset=UTF-8');
    header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
    header('Vary: Accept, Sec-Fetch-Mode, Sec-Fetch-Dest, Sec-Fetch-User');

    $meta = getFileMeta($file);
    $publicPath = $mode === 'file'
        ? '/files/' . $type . '/' . $fileName
        : '/' . $type . '/' . $fileName;
    $pagePath = buildPublicDetailPagePath($mode, $type, $fileName);
    $pageUrl = buildPrimarySiteAbsoluteUrl($pagePath);
    $rawUrl = $cdnUrl ?? buildAbsoluteUrl($server, $publicPath);
    $downloadUrl = buildAssetAbsoluteUrl($server, appendQueryParam($publicPath, 'download', '1'));
?>
    <!doctype html>
    <html lang="zh-CN">

    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <title><?= htmlspecialchars("{$type} {$fileName}", ENT_QUOTES, 'UTF-8') ?> - img.et</title>
        <meta name="robots" content="index,follow">
        <link rel="canonical" href="<?= htmlspecialchars($pageUrl, ENT_QUOTES, 'UTF-8') ?>">
        <link rel="stylesheet" href="/assets/main.min.css?v=a0737c5b">
    </head>

    <body class="page-public-image">
        <div class="wrap">
            <div class="topbar">
                <a class="brand" href="<?= htmlspecialchars(buildPrimarySiteAbsoluteUrl('/'), ENT_QUOTES, 'UTF-8') ?>">img.et</a>
                <a class="action" href="<?= htmlspecialchars(buildPrimarySiteAbsoluteUrl('/'), ENT_QUOTES, 'UTF-8') ?>">返回主站</a>
            </div>
            <div class="panel">
                <h1><?= htmlspecialchars($type, ENT_QUOTES, 'UTF-8') ?></h1>
                <p>当前图片资源详情页，适合承接从资源域名直接打开的访问。</p>
                <div class="preview" view-image>
                    <img src="<?= htmlspecialchars($rawUrl, ENT_QUOTES, 'UTF-8') ?>" alt="<?= htmlspecialchars($fileName, ENT_QUOTES, 'UTF-8') ?>">
                </div>
                <div class="actions">
                    <a class="action" href="<?= htmlspecialchars($rawUrl, ENT_QUOTES, 'UTF-8') ?>" target="_blank" rel="noreferrer">打开图片</a>
                    <a class="action" href="<?= htmlspecialchars($downloadUrl, ENT_QUOTES, 'UTF-8') ?>">下载图片</a>
                </div>
                <div class="grid">
                    <div class="stat"><strong>类型</strong><span><?= htmlspecialchars($type, ENT_QUOTES, 'UTF-8') ?></span></div>
                    <div class="stat"><strong>文件名</strong><span><?= htmlspecialchars($fileName, ENT_QUOTES, 'UTF-8') ?></span></div>
                    <div class="stat"><strong>尺寸</strong><span><?= htmlspecialchars(($meta['width'] ?? 0) . ' x ' . ($meta['height'] ?? 0), ENT_QUOTES, 'UTF-8') ?></span></div>
                    <div class="stat"><strong>大小</strong><span><?= htmlspecialchars((string)($meta['size_human'] ?? '0 B'), ENT_QUOTES, 'UTF-8') ?></span></div>
                </div>
            </div>
        </div>
        <script defer src="/assets/main.min.js"></script>
    </body>

    </html>
<?php
}

function buildAbsoluteUrl(array $server, string $path): string
{
    $scheme = 'https';
    if (!empty($server['HTTP_X_FORWARDED_PROTO'])) {
        $scheme = strtolower((string)$server['HTTP_X_FORWARDED_PROTO']);
    } elseif (!empty($server['REQUEST_SCHEME'])) {
        $scheme = strtolower((string)$server['REQUEST_SCHEME']);
    } elseif (($server['HTTPS'] ?? '') !== '' && ($server['HTTPS'] ?? '') !== 'off') {
        $scheme = 'https';
    } elseif (($server['SERVER_PORT'] ?? '') === '80') {
        $scheme = 'http';
    }

    $host = (string)($server['HTTP_HOST'] ?? $server['SERVER_NAME'] ?? 'img.et');
    if ($host === '') {
        $host = 'img.et';
    }

    if ($path === '') {
        $path = '/';
    }
    if ($path[0] !== '/') {
        $path = '/' . $path;
    }

    return "{$scheme}://{$host}{$path}";
}

function buildPrimarySiteAbsoluteUrl(string $path): string
{
    $base = getConfiguredSiteBaseUrl();

    if ($path === '') {
        $path = '/';
    }
    if ($path[0] !== '/') {
        $path = '/' . $path;
    }

    return $base . $path;
}

function buildAssetAbsoluteUrl(array $server, string $path): string
{
    $base = getConfiguredAssetBaseUrl();
    if ($base === '') {
        return buildAbsoluteUrl($server, $path);
    }

    if ($path === '') {
        $path = '/';
    }
    if ($path[0] !== '/') {
        $path = '/' . $path;
    }

    return $base . $path;
}

function buildOriginalCdnUrl(string $path): string
{
    $base = getConfiguredOriginalCdnUrl();
    if ($base === '') {
        return $path;
    }

    if ($path === '') {
        $path = '/';
    }
    if ($path[0] !== '/') {
        $path = '/' . $path;
    }

    return $base . $path;
}

function appendQueryParam(string $path, string $key, string $value): string
{
    return str_contains($path, '?')
        ? "{$path}&{$key}=" . rawurlencode($value)
        : "{$path}?{$key}=" . rawurlencode($value);
}

function normalizeRouteType(string $type): string
{
    $type = strtolower($type);
    $type = preg_replace('/[^a-z0-9_-]+/', '', $type) ?? '';
    return $type !== '' ? $type : 'banner';
}

function getTypeChineseLabel(string $type): string
{
    static $map = [
        'banner' => '通用横幅',
        'landscape' => '风景山水',
        'beauty' => '人物美图',
        'anime' => '动漫插画',
        'city' => '城市建筑',
        'nature' => '自然风光',
        'car' => '汽车',
        'game' => '游戏电竞',
        'food' => '美食',
        'animal' => '动物',
        'travel' => '旅行',
        'space' => '星空宇宙',
        'tech' => '科技',
        'business' => '商务',
        'sports' => '运动',
        'architecture' => '建筑设计',
    ];
    return $map[$type] ?? $type;
}

function buildPublicDetailPagePath(string $mode, string $type, string $fileName): string
{
    return $mode === 'file'
        ? '/p/files/' . $type . '/' . $fileName
        : '/p/' . $type . '/' . $fileName;
}

function getPublicImageUrlForFile(string $file): ?string
{
    $imagesRoot = str_replace('\\', '/', getImagesRoot());
    $normalized = str_replace('\\', '/', $file);

    if (strpos($normalized, $imagesRoot . '/original/') === 0) {
        $relative = substr($normalized, strlen($imagesRoot . '/original/'));
        if ($relative === false || $relative === '') {
            return null;
        }
        return '/original/' . ltrim($relative, '/');
    }

    if (strpos($normalized, $imagesRoot . '/') === 0) {
        $relative = substr($normalized, strlen($imagesRoot . '/'));
        if ($relative === false || $relative === '' || str_starts_with($relative, 'original/')) {
            return null;
        }
        return '/' . ltrim($relative, '/');
    }

    return null;
}

function sendImageResponse(string $file, int $cacheSeconds, bool $download = false, string $downloadName = 'image'): void
{
    $cdnUrl = getR2CdnUrlForFile($file);
    if (!$download && $cdnUrl !== null && shouldRedirectDirectResponsesToR2()) {
        header('Location: ' . $cdnUrl, true, 302);
        exit;
    }

    if (!is_file($file) || !is_readable($file)) {
        http_response_code(404);
        exit('Image file missing');
    }

    // Ensure file is uploaded to R2 (async, non-blocking)
    if (r2IsConfigured()) {
        $relativePath = r2RelativePath($file);
        if (r2GetCdnUrl($relativePath) === null) {
            scheduleR2Upload($file);
        }
    }

    $mime = detectImageMime($file);
    if ($mime === '') {
        $mime = 'application/octet-stream';
    }

    if ($cacheSeconds > 0) {
        header("Cache-Control: public, max-age={$cacheSeconds}, s-maxage={$cacheSeconds}");
        header('Pragma: public');
    } else {
        header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
        header('Pragma: no-cache');
    }
    header('Vary: Accept, Sec-Fetch-Mode, Sec-Fetch-Dest, Sec-Fetch-User');

    header('Content-Type: ' . $mime);
    if ($download) {
        header('Content-Disposition: attachment; filename="' . rawurlencode($downloadName) . '"');
    }
    header('Content-Length: ' . (string)filesize($file));
    readfile($file);
}

function buildRelativeImagePath(string $mode, string $type, string $fileName): string
{
    return $mode === 'file'
        ? 'original/' . $type . '/' . $fileName
        : $type . '/' . $fileName;
}

function buildLocalImagePath(string $mode, string $type, string $fileName): string
{
    return $mode === 'file'
        ? __DIR__ . '/images/original/' . $type . '/' . $fileName
        : __DIR__ . '/images/' . $type . '/' . $fileName;
}

function shouldRedirectDirectResponsesToR2(): bool
{
    $value = strtolower(trim((string)(envValue('R2_REDIRECT_DIRECT', '1') ?: '1')));
    return !in_array($value, ['0', 'false', 'off', 'no'], true);
}

function getR2CdnUrlForFile(string $file): ?string
{
    if (!r2IsConfigured()) {
        return null;
    }

    return r2GetCdnUrl(r2RelativePath($file));
}

function getDirectAccessUrlForFile(string $file, array $server, string $fallbackPath): string
{
    $cdnUrl = getR2CdnUrlForFile($file);
    if ($cdnUrl !== null) {
        return $cdnUrl;
    }

    return buildPrimarySiteAbsoluteUrl($fallbackPath);
}
