<?php

declare(strict_types=1);

ini_set('display_errors', '0');
ini_set('html_errors', '0');
error_reporting(E_ALL & ~E_DEPRECATED & ~E_USER_DEPRECATED);

require_once dirname(__DIR__) . '/imageLibrary.php';

const ADMIN_PROFILE_LIMIT = 10;
const ADMIN_LOG_LIMIT = 10;
const ADMIN_MAX_REFRESH = 1000;
const ADMIN_SESSION_KEY = 'imget_admin_user_id';
const ADMIN_FLASH_KEY = 'imget_admin_flash';

function formatRefreshError(string $error): string
{
  $error = trim($error);
  if ($error === '') {
    return '';
  }

  if ($error === 'Typed image fetch requires PIXABAY_API_KEY for keyword search') {
    return 'Pixabay API key 未配置，当前分类无法补图';
  }

  if ($error === 'Pixabay API cooldown active') {
    $status = getPixabayRateLimitStatus();
    $remaining = (int)($status['remaining_seconds'] ?? 0);
    return $remaining > 0
      ? "Pixabay 请求冷却中，约 {$remaining} 秒后再试"
      : 'Pixabay 请求冷却中，请稍后再试';
  }

  if (stripos($error, 'rate limit') !== false) {
    return 'Pixabay 请求过于频繁，已进入限速保护';
  }

  if ($error === 'type_limit_reached') {
    return '当前分类已达到数量上限';
  }

  return $error;
}

startAdminSession();

$flash = consumeAdminFlash();
$loginError = null;
$currentAdmin = getCurrentAdminUser();

if (($_GET['action'] ?? '') === 'logout') {
  logoutAdminUser();
  header('Location: /admin/index.php');
  exit;
}

if (($_SERVER['REQUEST_METHOD'] ?? 'GET') === 'POST') {
  $action = (string)($_POST['action'] ?? '');

  if ($action === 'login') {
    $username = trim((string)($_POST['username'] ?? ''));
    $password = (string)($_POST['password'] ?? '');
    $adminUser = storageAuthenticateAdminUser($username, $password);

    if (is_array($adminUser)) {
      loginAdminUser((int)$adminUser['id']);
      header('Location: /admin/index.php');
      exit;
    }

    $loginError = '账号或密码错误';
  } elseif ($currentAdmin !== null && $action === 'refresh_profile') {
    $width = max(50, (int)($_POST['width'] ?? 0));
    $height = max(50, (int)($_POST['height'] ?? 0));
    $type = normalizeImageType((string)($_POST['type'] ?? 'banner'));
    $keyword = trim((string)($_POST['keyword'] ?? ''));
    $count = max(1, min(ADMIN_MAX_REFRESH, (int)($_POST['count'] ?? 1)));

    $result = refreshRequestedProfileImages($width, $height, $type, $keyword, $count, 1000, 'manual');
    $saved = (int)($result['saved'] ?? 0);
    $target = (int)($result['target'] ?? 0);
    $renderGenerated = (int)($result['render_generated'] ?? 0);
    $renderAttempted = (int)($result['render_attempted'] ?? 0);
    $error = trim((string)($result['error'] ?? $result['skipped'] ?? ''));
    $displayError = formatRefreshError($error);

    if (isAjaxRequest()) {
      header('Content-Type: application/json; charset=UTF-8');
      echo json_encode([
        'ok' => $saved > 0 || $renderGenerated > 0,
        'width' => $width,
        'height' => $height,
        'type' => $type,
        'keyword' => $keyword,
        'saved' => $saved,
        'target' => $target,
        'render_generated' => $renderGenerated,
        'render_attempted' => $renderAttempted,
        'message' => "{$width}x{$height} type={$type} 原图 {$saved}/{$target}，缓存 {$renderGenerated}/{$renderAttempted}" . ($displayError !== '' ? " ({$displayError})" : ''),
        'error' => $displayError,
      ], JSON_UNESCAPED_UNICODE);
      exit;
    }

    setAdminFlash([
      'type' => $saved > 0 ? 'success' : 'warning',
      'message' => "{$width}x{$height} type={$type} 原图 {$saved}/{$target}，缓存 {$renderGenerated}/{$renderAttempted}" . ($displayError !== '' ? " ({$displayError})" : ''),
    ]);
    header('Location: /admin/index.php');
    exit;
  } elseif ($currentAdmin !== null && $action === 'update_credentials') {
    $username = trim((string)($_POST['username'] ?? ''));
    $password = trim((string)($_POST['password'] ?? ''));
    $result = storageUpdateAdminUserCredentials((int)$currentAdmin['id'], $username, $password);

    if (($result['ok'] ?? false) === true) {
      setAdminFlash([
        'type' => 'success',
        'message' => '后台账号密码已更新',
      ]);
    } else {
      $error = (string)($result['error'] ?? 'update_failed');
      setAdminFlash([
        'type' => 'warning',
        'message' => $error === 'username_taken' ? '用户名已存在' : '账号或密码不能为空',
      ]);
    }
    header('Location: /admin/index.php');
    exit;
  }
}

if ($currentAdmin === null) {
  $getAction = ($_GET['action'] ?? '');
  if ($getAction === 'fetch_logs' || $getAction === 'fetch_profiles' || (($_SERVER['REQUEST_METHOD'] ?? '') === 'POST' && in_array((string)($_POST['action'] ?? ''), ['delete_profile', 'clear_logs'], true))) {
    http_response_code(403);
    header('Content-Type: application/json; charset=UTF-8');
    echo json_encode(['error' => 'unauthorized']);
    exit;
  }
  renderLoginPage($loginError);
  exit;
}

if (($_GET['action'] ?? '') === 'fetch_logs') {
  header('Content-Type: application/json; charset=UTF-8');
  $page = max(1, (int)($_GET['page'] ?? 1));
  $offset = ($page - 1) * ADMIN_LOG_LIMIT;
  $rows = storageGetRefreshLogs(ADMIN_LOG_LIMIT, $offset);
  $items = [];
  foreach ($rows as $log) {
    $items[] = [
      'created_at' => (string)($log['created_at'] ?? '-'),
      'mode' => (string)($log['mode'] ?? '-'),
      'width' => (int)($log['width'] ?? 0),
      'height' => (int)($log['height'] ?? 0),
      'type' => (string)($log['type'] ?? ''),
      'keyword' => (string)($log['keyword'] ?? ''),
      'requested_count' => (int)($log['requested_count'] ?? 0),
      'saved_count' => (int)($log['saved_count'] ?? 0),
      'error_text' => (string)($log['error_text'] ?? 'ok'),
    ];
  }
  echo json_encode(['page' => $page, 'per_page' => ADMIN_LOG_LIMIT, 'total' => storageCountRefreshLogs(), 'items' => $items]);
  exit;
}

if (($_GET['action'] ?? '') === 'fetch_profiles') {
  header('Content-Type: application/json; charset=UTF-8');
  $page = max(1, (int)($_GET['page'] ?? 1));
  $offset = ($page - 1) * ADMIN_PROFILE_LIMIT;
  $allProfiles = array_values(storageGetRequestedProfiles(ADMIN_PROFILE_LIMIT, $offset));
  $ts = getTypeImageStats();
  $ti = [];
  foreach ($ts as $r) { $ti[(string)$r['type']] = $r; }
  $items = [];
  foreach ($allProfiles as $p) {
    $t = normalizeImageType((string)($p['type'] ?? 'banner'));
    $st = $ti[$t] ?? null;
    $items[] = [
      'profile_key' => (string)($p['profile_key'] ?? ''),
      'width' => (int)($p['width'] ?? 0),
      'height' => (int)($p['height'] ?? 0),
      'type' => $t,
      'keyword' => (string)($p['keyword'] ?? ''),
      'request_count' => (int)($p['request_count'] ?? 0),
      'original_count' => (int)(is_array($st) ? $st['original_count'] : 0),
      'original_bytes' => (int)(is_array($st) ? $st['original_bytes'] : 0),
      'rendered_count' => countRenderedImagesForProfile($t, (int)($p['width'] ?? 0), (int)($p['height'] ?? 0)),
      'first_requested_at' => (string)($p['first_requested_at'] ?? '-'),
      'last_requested_at' => (string)($p['last_requested_at'] ?? '-'),
      'initial_prefetch_done_at' => (string)($p['initial_prefetch_done_at'] ?? '-'),
      'last_daily_topup_on' => (string)($p['last_daily_topup_on'] ?? '-'),
      'last_daily_topup_saved' => (int)($p['last_daily_topup_saved'] ?? 0),
      'last_manual_refresh_at' => (string)($p['last_manual_refresh_at'] ?? '-'),
      'last_manual_refresh_saved' => (int)($p['last_manual_refresh_saved'] ?? 0),
    ];
  }
  echo json_encode(['page' => $page, 'per_page' => ADMIN_PROFILE_LIMIT, 'items' => $items]);
  exit;
}

if (($_SERVER['REQUEST_METHOD'] ?? '') === 'POST' && ($_POST['action'] ?? '') === 'delete_profile') {
  header('Content-Type: application/json; charset=UTF-8');
  $profileKey = trim((string)($_POST['profile_key'] ?? ''));
  $deleted = storageDeleteRequestedProfile($profileKey);
  echo json_encode(['ok' => $deleted, 'profile_key' => $profileKey]);
  exit;
}

if (($_SERVER['REQUEST_METHOD'] ?? '') === 'POST' && ($_POST['action'] ?? '') === 'clear_logs') {
  header('Content-Type: application/json; charset=UTF-8');
  $cleared = storageClearRefreshLogs();
  echo json_encode(['ok' => $cleared, 'total' => storageCountRefreshLogs()]);
  exit;
}

$summary = getLibrarySummary();
$typeStats = getTypeImageStats();
$profiles = array_values(storageGetRequestedProfiles(ADMIN_PROFILE_LIMIT, 0));
$logs = storageGetRefreshLogs(ADMIN_LOG_LIMIT);
$logTotal = storageCountRefreshLogs();
$typeIndex = [];
foreach ($typeStats as $row) {
  $typeIndex[(string)$row['type']] = $row;
}
$adminTypeOptions = [
  'banner' => 'banner 通用横幅',
  'landscape' => 'landscape 风景山水',
  'beauty' => 'beauty 人物人像',
  'anime' => 'anime 动漫插画',
  'city' => 'city 城市建筑',
  'nature' => 'nature 森林海洋',
  'car' => 'car 汽车机车',
  'game' => 'game 游戏电竞',
  'food' => 'food 美食甜点',
  'animal' => 'animal 动物萌宠',
  'travel' => 'travel 旅行度假',
  'space' => 'space 星空宇宙',
  'tech' => 'tech 科技数码',
  'business' => 'business 商务办公',
  'sports' => 'sports 运动健身',
  'architecture' => 'architecture 建筑室内',
];

?>
<!doctype html>
<html lang="zh-CN">

<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>img.et Admin</title>
  <link rel="stylesheet" href="/admin/admin.min.css">
</head>

<body>
  <div class="wrap">
    <div class="header">
      <div class="header-top">
        <div class="brand-block">
          <div class="brand-logo" aria-hidden="true"><?php renderAdminLogo(); ?></div>
          <div>
            <h1>img.et 管理后台</h1>
          </div>
        </div>
        <div class="header-actions">
          <a class="button-link secondary" href="/admin/index.php?action=logout">退出登录</a>
        </div>
      </div>
      <div class="grid">
        <div class="stat"><strong>图片分类数</strong><span><?= e((string)$summary['type_count']) ?></span></div>
        <div class="stat"><strong>已登记组合</strong><span><?= e((string)$summary['profile_count']) ?></span></div>
        <div class="stat"><strong>原图库张数</strong><span><?= e((string)$summary['original_count']) ?></span></div>
        <div class="stat"><strong>缓存图张数</strong><span><?= e((string)$summary['rendered_count']) ?></span></div>
        <div class="stat"><strong>总占用空间</strong><span><?= e(formatBytes((int)$summary['total_bytes'])) ?></span></div>
      </div>
      <div class="db-note">
        数据库：<span class="mono"><?= e((string)$summary['db']['driver']) ?></span>
        <span class="mono"><?= e((string)$summary['db']['label']) ?></span>
      </div>
    </div>

    <?php if (is_array($flash)): ?>
      <div class="panel">
        <div class="flash <?= e((string)$flash['type']) ?>"><?= e((string)$flash['message']) ?></div>
      </div>
    <?php endif; ?>

    <div class="panel progress-panel" id="refresh-progress">
      <div class="progress-label" id="refresh-progress-label">正在刷新图片，请稍候...</div>
      <div class="progress-track">
        <div class="progress-bar"></div>
      </div>
    </div>

    <div class="panel">
      <h2>后台账号</h2>
      <form method="post" action="/admin/index.php">
        <input type="hidden" name="action" value="update_credentials">
        <div class="account-grid">
          <div>
            <label for="admin-username">用户名</label>
            <input id="admin-username" name="username" type="text" value="<?= e((string)$currentAdmin['username']) ?>">
          </div>
          <div>
            <label for="admin-password">新密码</label>
            <input id="admin-password" name="password" type="password" placeholder="输入新密码">
          </div>
          <div>
            <label>&nbsp;</label>
            <button type="submit">更新账号</button>
          </div>
        </div>
      </form>
      <p class="db-note">默认管理员会自动写入数据库：<span class="mono">admin / 123456</span>。登录后可在这里直接修改。</p>
    </div>

    <div class="panel">
      <h2>手动刷新</h2>
      <form method="post" action="/admin/index.php" class="refresh-form">
        <input type="hidden" name="action" value="refresh_profile">
        <div class="toolbar">
          <div>
            <label for="width">宽度</label>
            <input id="width" name="width" type="number" value="1920" min="50" max="4000">
          </div>
          <div>
            <label for="height">高度</label>
            <input id="height" name="height" type="number" value="1080" min="50" max="4000">
          </div>
          <div>
            <label for="type">分类</label>
            <select id="type" name="type">
              <?php foreach ($adminTypeOptions as $optionValue => $optionLabel): ?>
                <option value="<?= e($optionValue) ?>"<?= $optionValue === 'banner' ? ' selected' : '' ?>><?= e($optionLabel) ?></option>
              <?php endforeach; ?>
            </select>
          </div>
          <div>
            <label for="keyword">关键词</label>
            <input id="keyword" name="keyword" type="text" placeholder="可选，空则走 Picsum">
          </div>
          <div>
            <label for="count">刷新数量</label>
            <input id="count" name="count" type="number" value="50" min="1" max="<?= e((string)ADMIN_MAX_REFRESH) ?>">
          </div>
          <div>
            <label>&nbsp;</label>
            <button type="submit">立即刷新</button>
          </div>
        </div>
      </form>
      <p class="db-note">接口级强制刷新也可直接用：<span class="mono">https://img.et/1920/1080?type=banner&amp;fresh=1</span> 或 <span class="mono">fresh=100</span>。</p>
    </div>

    <div class="panel">
      <h2>分类统计</h2>
      <table class="log-table">
        <thead>
          <tr>
            <th>分类</th>
            <th>原图张数</th>
            <th>原图占用</th>
            <th>缓存张数</th>
            <th>缓存占用</th>
            <th>总张数</th>
            <th>总占用</th>
            <th>已登记组合</th>
            <th>请求次数</th>
          </tr>
        </thead>
        <tbody>
          <?php foreach ($typeStats as $row): ?>
            <tr>
              <td class="mono"><?= e((string)$row['type']) ?></td>
              <td><?= e((string)$row['original_count']) ?></td>
              <td><?= e(formatBytes((int)$row['original_bytes'])) ?></td>
              <td><?= e((string)$row['rendered_count']) ?></td>
              <td><?= e(formatBytes((int)$row['rendered_bytes'])) ?></td>
              <td><?= e((string)$row['total_count']) ?></td>
              <td><?= e(formatBytes((int)$row['total_bytes'])) ?></td>
              <td><?= e((string)$row['profile_count']) ?></td>
              <td><?= e((string)$row['request_count']) ?></td>
            </tr>
          <?php endforeach; ?>
        </tbody>
      </table>
    </div>

    <div class="panel">
      <h2>请求档案</h2>
      <table>
        <thead>
          <tr>
            <th>分辨率</th>
            <th>分类 / 关键词</th>
            <th>请求</th>
            <th>图池</th>
            <th>首次 / 最近</th>
            <th>补图状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody id="profile-tbody">
          <?php foreach ($profiles as $profile): ?>
            <?php
            $type = normalizeImageType((string)($profile['type'] ?? 'banner'));
            $stats = $typeIndex[$type] ?? null;
            ?>
            <tr data-key="<?= e((string)($profile['profile_key'] ?? '')) ?>">
              <td class="mono"><?= e((string)$profile['width']) ?>x<?= e((string)$profile['height']) ?></td>
              <td>
                <div class="mono"><?= e($type) ?></div>
                <div class="small muted"><?= e((string)($profile['keyword'] ?? '')) ?></div>
              </td>
              <td><?= e((string)($profile['request_count'] ?? 0)) ?></td>
              <td>
                <?= e((string)(is_array($stats) ? $stats['original_count'] : 0)) ?> 张原图
                <div class="small muted"><?= e(formatBytes((int)(is_array($stats) ? $stats['original_bytes'] : 0))) ?></div>
                <div class="small">缓存进度：<?= e((string)countRenderedImagesForProfile($type, (int)$profile['width'], (int)$profile['height'])) ?> 张</div>
              </td>
              <td>
                <div class="small">首次：<?= e((string)($profile['first_requested_at'] ?? '-')) ?></div>
                <div class="small">最近：<?= e((string)($profile['last_requested_at'] ?? '-')) ?></div>
              </td>
              <td>
                <div class="small">首次预热：<?= e((string)($profile['initial_prefetch_done_at'] ?? '-')) ?></div>
                <div class="small">每日补图：<?= e((string)($profile['last_daily_topup_on'] ?? '-')) ?> / <?= e((string)($profile['last_daily_topup_saved'] ?? 0)) ?></div>
                <div class="small">手动刷新：<?= e((string)($profile['last_manual_refresh_at'] ?? '-')) ?> / <?= e((string)($profile['last_manual_refresh_saved'] ?? 0)) ?></div>
              </td>
              <td>
                <form method="post" action="/admin/index.php" class="refresh-form profile-refresh-form">
                  <input type="hidden" name="action" value="refresh_profile">
                  <input type="hidden" name="width" value="<?= e((string)$profile['width']) ?>">
                  <input type="hidden" name="height" value="<?= e((string)$profile['height']) ?>">
                  <input type="hidden" name="type" value="<?= e($type) ?>">
                  <input type="hidden" name="keyword" value="<?= e((string)($profile['keyword'] ?? '')) ?>">
                  <div class="actions">
                    <button type="submit" name="count" value="1">1张</button>
                    <button type="submit" name="count" value="10" class="secondary">10张</button>
                    <button type="submit" name="count" value="50" class="secondary">50张</button>
                    <button type="button" class="danger delete-profile-btn" data-key="<?= e((string)($profile['profile_key'] ?? '')) ?>">删除</button>
                  </div>
                  <div class="ajax-progress" hidden></div>
                </form>
              </td>
            </tr>
          <?php endforeach; ?>
        </tbody>
      </table>
      <div class="log-pager" id="profile-pager">
        <button type="button" id="profile-prev" disabled aria-label="上一页">&laquo;</button>
        <span id="profile-page-info">第 1 页</span>
        <button type="button" id="profile-next"<?= count($profiles) < ADMIN_PROFILE_LIMIT ? ' disabled' : '' ?> aria-label="下一页">&raquo;</button>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <h2>最近刷新日志</h2>
        <div class="panel-head-actions">
          <span class="panel-badge" id="log-total-count"><?= e((string)$logTotal) ?> 条</span>
          <button type="button" class="secondary compact" id="clear-logs-btn">一键清除</button>
        </div>
      </div>
      <table class="log-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>模式</th>
            <th>分辨率</th>
            <th>分类</th>
            <th>关键词</th>
            <th>请求 / 成功</th>
            <th>结果</th>
          </tr>
        </thead>
        <tbody id="log-tbody">
          <?php foreach ($logs as $log): ?>
            <tr>
              <td><?= e((string)($log['created_at'] ?? '-')) ?></td>
              <td class="mono"><?= e((string)($log['mode'] ?? '-')) ?></td>
              <td class="mono"><?= e((string)($log['width'] ?? 0)) ?>x<?= e((string)($log['height'] ?? 0)) ?></td>
              <td class="mono"><?= e((string)($log['type'] ?? '')) ?></td>
              <td><?= e((string)($log['keyword'] ?? '')) ?></td>
              <td><?= e((string)($log['requested_count'] ?? 0)) ?> / <?= e((string)($log['saved_count'] ?? 0)) ?></td>
              <td><?= e((string)($log['error_text'] ?? 'ok')) ?></td>
            </tr>
          <?php endforeach; ?>
        </tbody>
      </table>
      <div class="log-pager" id="log-pager">
        <button type="button" id="log-prev" disabled aria-label="上一页">&laquo;</button>
        <span id="log-page-info">第 1 页</span>
        <button type="button" id="log-next"<?= count($logs) < ADMIN_LOG_LIMIT ? ' disabled' : '' ?> aria-label="下一页">&raquo;</button>
      </div>
    </div>
  </div>
  <script>
    (function() {
      var progressPanel = document.getElementById('refresh-progress');
      var progressLabel = document.getElementById('refresh-progress-label');

      document.querySelectorAll('.refresh-form').forEach(function(form) {
        form.addEventListener('submit', function() {
          if (form.classList.contains('profile-refresh-form')) {
            return;
          }

          var widthInput = form.querySelector('input[name="width"]');
          var heightInput = form.querySelector('input[name="height"]');
          var typeInput = form.querySelector('input[name="type"]');
          var countInput = form.querySelector('input[name="count"]');

          var width = widthInput ? widthInput.value : '';
          var height = heightInput ? heightInput.value : '';
          var type = typeInput ? typeInput.value : '';
          var count = countInput ? countInput.value : '';

          if (progressPanel) {
            progressPanel.classList.add('active');
          }

          if (progressLabel) {
            var parts = [];
            if (width && height) {
              parts.push(width + 'x' + height);
            }
            if (type) {
              parts.push('type=' + type);
            }
            if (count) {
              parts.push('count=' + count);
            }
            progressLabel.textContent = parts.length > 0 ?
              '正在刷新图片：' + parts.join(' ') :
              '正在刷新图片，请稍候...';
          }

          form.querySelectorAll('button').forEach(function(button) {
            button.disabled = true;
          });
        });
      });
    }());

    (function() {
      var currentPage = 1;
      var tbody = document.getElementById('log-tbody');
      var prevBtn = document.getElementById('log-prev');
      var nextBtn = document.getElementById('log-next');
      var pageInfo = document.getElementById('log-page-info');
      var totalCount = document.getElementById('log-total-count');
      var clearBtn = document.getElementById('clear-logs-btn');
      if (!tbody || !prevBtn || !nextBtn || !pageInfo) return;

      function esc(s) {
        var d = document.createElement('div');
        d.textContent = s;
        return d.innerHTML;
      }

      function loadPage(page) {
        prevBtn.disabled = true;
        nextBtn.disabled = true;
        fetch('/admin/index.php?action=fetch_logs&page=' + page)
          .then(function(r) { return r.json(); })
          .then(function(data) {
            currentPage = data.page;
            var rows = data.items || [];
            tbody.innerHTML = '';
            rows.forEach(function(r) {
              tbody.innerHTML +=
                '<tr>' +
                '<td>' + esc(r.created_at) + '</td>' +
                '<td class="mono">' + esc(r.mode) + '</td>' +
                '<td class="mono">' + r.width + 'x' + r.height + '</td>' +
                '<td class="mono">' + esc(r.type) + '</td>' +
                '<td>' + esc(r.keyword) + '</td>' +
                '<td>' + r.requested_count + ' / ' + r.saved_count + '</td>' +
                '<td>' + esc(r.error_text) + '</td>' +
                '</tr>';
            });
            if (totalCount) {
              totalCount.textContent = String(data.total || 0) + ' 条';
            }
            pageInfo.textContent = '第 ' + currentPage + ' 页';
            prevBtn.disabled = currentPage <= 1;
            nextBtn.disabled = rows.length < data.per_page;
          })
          .catch(function() {
            prevBtn.disabled = currentPage <= 1;
            nextBtn.disabled = false;
          });
      }

      prevBtn.addEventListener('click', function() {
        if (currentPage > 1) loadPage(currentPage - 1);
      });
      nextBtn.addEventListener('click', function() {
        loadPage(currentPage + 1);
      });

      if (clearBtn) {
        clearBtn.addEventListener('click', function() {
          if (!confirm('确定清空最近刷新日志？')) return;

          clearBtn.disabled = true;
          var fd = new FormData();
          fd.append('action', 'clear_logs');
          fetch('/admin/index.php', {
            method: 'POST',
            body: fd
          })
            .then(function(r) { return r.json(); })
            .then(function(payload) {
              if (!payload || !payload.ok) {
                throw new Error('clear_failed');
              }
              currentPage = 1;
              loadPage(1);
            })
            .catch(function() {
              alert('清空失败，请重试');
            })
            .finally(function() {
              clearBtn.disabled = false;
            });
        });
      }
    }());

    (function() {
      var currentPage = 1;
      var tbody = document.getElementById('profile-tbody');
      var prevBtn = document.getElementById('profile-prev');
      var nextBtn = document.getElementById('profile-next');
      var pageInfo = document.getElementById('profile-page-info');
      if (!tbody || !prevBtn || !nextBtn || !pageInfo) return;

      function esc(s) {
        var d = document.createElement('div');
        d.textContent = s;
        return d.innerHTML;
      }

      function formatBytes(b) {
        var u = ['B','KB','MB','GB'];
        var i = 0;
        while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
        return (i === 0 ? b : b.toFixed(2)) + ' ' + u[i];
      }

      function buildRow(p) {
        return '<tr data-key="' + esc(p.profile_key) + '">' +
          '<td class="mono">' + p.width + 'x' + p.height + '</td>' +
          '<td><div class="mono">' + esc(p.type) + '</div><div class="small muted">' + esc(p.keyword) + '</div></td>' +
          '<td>' + p.request_count + '</td>' +
          '<td>' + p.original_count + ' 张原图<div class="small muted">' + formatBytes(p.original_bytes) + '</div><div class="small">缓存进度：' + p.rendered_count + ' 张</div></td>' +
          '<td><div class="small">首次：' + esc(p.first_requested_at) + '</div><div class="small">最近：' + esc(p.last_requested_at) + '</div></td>' +
          '<td>' +
            '<div class="small">首次预热：' + esc(p.initial_prefetch_done_at) + '</div>' +
            '<div class="small">每日补图：' + esc(p.last_daily_topup_on) + ' / ' + p.last_daily_topup_saved + '</div>' +
            '<div class="small">手动刷新：' + esc(p.last_manual_refresh_at) + ' / ' + p.last_manual_refresh_saved + '</div>' +
          '</td>' +
          '<td>' +
            '<form method="post" action="/admin/index.php" class="refresh-form profile-refresh-form">' +
              '<input type="hidden" name="action" value="refresh_profile">' +
              '<input type="hidden" name="width" value="' + p.width + '">' +
              '<input type="hidden" name="height" value="' + p.height + '">' +
              '<input type="hidden" name="type" value="' + esc(p.type) + '">' +
              '<input type="hidden" name="keyword" value="' + esc(p.keyword) + '">' +
              '<div class="actions">' +
                '<button type="submit" name="count" value="1">1张</button>' +
                '<button type="submit" name="count" value="10" class="secondary">10张</button>' +
                '<button type="submit" name="count" value="50" class="secondary">50张</button>' +
                '<button type="button" class="danger delete-profile-btn" data-key="' + esc(p.profile_key) + '">删除</button>' +
              '</div>' +
              '<div class="ajax-progress" hidden></div>' +
            '</form>' +
          '</td></tr>';
      }

      function loadPage(page) {
        prevBtn.disabled = true;
        nextBtn.disabled = true;
        fetch('/admin/index.php?action=fetch_profiles&page=' + page)
          .then(function(r) { return r.json(); })
          .then(function(data) {
            currentPage = data.page;
            var rows = data.items || [];
            tbody.innerHTML = '';
            rows.forEach(function(p) { tbody.innerHTML += buildRow(p); });
            pageInfo.textContent = '第 ' + currentPage + ' 页';
            prevBtn.disabled = currentPage <= 1;
            nextBtn.disabled = rows.length < data.per_page;
            bindDeleteButtons();
            bindRefreshForms();
          })
          .catch(function() {
            prevBtn.disabled = currentPage <= 1;
            nextBtn.disabled = false;
          });
      }

      function deleteProfile(key, row) {
        if (!confirm('确定删除该请求档案？')) return;
        var fd = new FormData();
        fd.append('action', 'delete_profile');
        fd.append('profile_key', key);
        fetch('/admin/index.php', { method: 'POST', body: fd })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.ok) {
              row.remove();
            } else {
              alert('删除失败');
            }
          })
          .catch(function() { alert('请求失败'); });
      }

      function bindDeleteButtons() {
        tbody.querySelectorAll('.delete-profile-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            var key = btn.getAttribute('data-key');
            var row = btn.closest('tr');
            if (key && row) deleteProfile(key, row);
          });
        });
      }

      function bindRefreshForms() {
        tbody.querySelectorAll('.profile-refresh-form').forEach(function(form) {
          form.addEventListener('submit', function(event) {
            event.preventDefault();

            var progress = form.querySelector('.ajax-progress');
            var submitter = event.submitter;
            var total = Math.max(1, parseInt(submitter ? (submitter.value || '1') : '1', 10) || 1);
            var completed = 0;
            var savedTotal = 0;
            var renderedTotal = 0;

            if (progress) {
              progress.hidden = false;
              progress.className = 'ajax-progress is-loading';
              progress.textContent = '正在加载 0/' + total + '...';
            }

            form.querySelectorAll('button').forEach(function(button) {
              button.disabled = true;
            });

            function runStep() {
              var formData = new FormData(form);
              formData.set('count', '1');

              return fetch('/admin/index.php', {
                method: 'POST',
                body: formData,
                headers: {
                  'X-Requested-With': 'XMLHttpRequest',
                  'Accept': 'application/json'
                }
              })
                .then(function(r) { return r.json(); })
                .then(function(payload) {
                  if (!payload) {
                    throw new Error('refresh_failed');
                  }

                  completed += 1;
                  savedTotal += Number(payload.saved || 0);
                  renderedTotal += Number(payload.render_generated || 0);

                  if (progress) {
                    progress.hidden = false;
                    progress.className = 'ajax-progress is-loading';
                    progress.textContent = '正在加载 ' + completed + '/' + total + '，原图 ' + savedTotal + '，缓存 ' + renderedTotal;
                  }

                  if (completed < total) {
                    return runStep();
                  }

                  if (progress) {
                    progress.hidden = false;
                    progress.className = 'ajax-progress is-success';
                    progress.textContent = '加载完成 ' + completed + '/' + total + '，原图 ' + savedTotal + '，缓存 ' + renderedTotal;
                  }

                  loadPage(currentPage);
                });
            }

            runStep()
              .catch(function() {
                if (progress) {
                  progress.hidden = false;
                  progress.className = 'ajax-progress is-error';
                  progress.textContent = '加载到 ' + completed + '/' + total + ' 时失败，请重试';
                }
              })
              .finally(function() {
                form.querySelectorAll('button').forEach(function(button) {
                  button.disabled = false;
                });
              });
          });
        });
      }

      bindDeleteButtons();
      bindRefreshForms();

      prevBtn.addEventListener('click', function() {
        if (currentPage > 1) loadPage(currentPage - 1);
      });
      nextBtn.addEventListener('click', function() {
        loadPage(currentPage + 1);
      });
    }());
  </script>
</body>

</html>
<?php

function startAdminSession(): void
{
  if (session_status() === PHP_SESSION_ACTIVE) {
    return;
  }

  session_name('IMGETADMIN');
  session_start();
}

function loginAdminUser(int $adminUserId): void
{
  $_SESSION[ADMIN_SESSION_KEY] = $adminUserId;
  if (function_exists('session_regenerate_id')) {
    session_regenerate_id(true);
  }
}

function logoutAdminUser(): void
{
  $_SESSION = [];
  if (session_status() === PHP_SESSION_ACTIVE) {
    session_destroy();
  }
}

function getCurrentAdminUser(): ?array
{
  $adminUserId = (int)($_SESSION[ADMIN_SESSION_KEY] ?? 0);
  if ($adminUserId < 1) {
    return null;
  }

  return storageGetAdminUserById($adminUserId);
}

function setAdminFlash(array $flash): void
{
  $_SESSION[ADMIN_FLASH_KEY] = [
    'type' => (string)($flash['type'] ?? 'success'),
    'message' => (string)($flash['message'] ?? ''),
  ];
}

function consumeAdminFlash(): ?array
{
  $flash = $_SESSION[ADMIN_FLASH_KEY] ?? null;
  unset($_SESSION[ADMIN_FLASH_KEY]);

  if (!is_array($flash)) {
    return null;
  }

  $message = trim((string)($flash['message'] ?? ''));
  if ($message === '') {
    return null;
  }

  return [
    'type' => (string)($flash['type'] ?? 'success'),
    'message' => $message,
  ];
}

function isAjaxRequest(): bool
{
  $requestedWith = strtolower((string)($_SERVER['HTTP_X_REQUESTED_WITH'] ?? ''));
  if ($requestedWith === 'xmlhttprequest') {
    return true;
  }

  $accept = strtolower((string)($_SERVER['HTTP_ACCEPT'] ?? ''));
  return str_contains($accept, 'application/json');
}

function renderLoginPage(?string $loginError): void
{
?>
  <!doctype html>
  <html lang="zh-CN">

  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>img.et Admin Login</title>
    <link rel="stylesheet" href="/admin/admin.min.css">
  </head>

  <body class="login-page">
    <form class="login" method="post" action="/admin/index.php">
      <div class="login-brand">
        <div class="brand-logo login-logo" aria-hidden="true"><?php renderAdminLogo(); ?></div>
      </div>
      <h1>img.et 后台登录</h1>
      <input type="hidden" name="action" value="login">
      <div class="field">
        <label for="username">用户名</label>
        <input id="username" name="username" type="text" value="admin" autocomplete="username">
      </div>
      <div class="field">
        <label for="password">密码</label>
        <input id="password" name="password" type="password" autocomplete="current-password">
      </div>
      <button type="submit">登录后台</button>
      <?php if ($loginError !== null): ?>
        <div class="error"><?= e($loginError) ?></div>
      <?php endif; ?>
    </form>
  </body>

  </html>
<?php
}

function renderAdminLogo(): void
{
  echo '<img src="/assets/logo.png" alt="img.et logo">';
}

function e(string $value): string
{
  return htmlspecialchars($value, ENT_QUOTES, 'UTF-8');
}

function formatBytes(int $bytes): string
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
