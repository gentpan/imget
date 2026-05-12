# imget

[English](README.md) · **简体中文**

一个单二进制的图片服务:把 `https://example.com/{宽}/{高}` 变成现渲的随机图,把 `https://example.com/{分类}` 变成原始分辨率的壁纸短链。原图来自 Pexels(主源,4K-8K)、Pixabay(兜底)和 Picsum(程序化占位);输出 WebP(默认)或 AVIF;所有图本地缓存 + 自动上传 Cloudflare R2 或任意 S3 兼容存储。

- **单二进制部署** — 没有管理后台,没有 PHP,没有运行时模板文件,CSS/JS/HTML 全部 embed 进二进制。
- **libvips 全程内嵌** — 解码 + 缩放 + 编码一次进程内完成,不 fork 任何子进程。
- **SQLite + 本地缓存** — 单 VPS 跑到几百万张图毫无压力。
- **20 个内置分类** + 任意 `?keyword=` 自定义搜索词。
- **壁纸模式** — `/{分类}` 直接 302 到 4K-8K 原图,适合壁纸 app 接入。
- **开源友好** — 站名、品牌、版权邮箱、抓图频率等所有可见参数都能通过 `.env` 覆盖。

## 快速开始

### 方案 A — Docker Compose(不需要 Go 工具链)

```bash
git clone https://github.com/gentpan/imget.git && cd imget
cp .env.example .env
# 编辑 .env,至少设置 SITE_BASE_URL。
# 想拿真实照片至少配一个 PEXELS_API_KEY 或 PIXABAY_API_KEY。
docker compose up -d
# 服务跑在 http://localhost:8080
```

镜像首次启动会从源码 build(~1 分钟),自带 `libvips42`。持久化数据存在主机的 `./data/database` 和 `./data/images`。

改完代码或 `.env` 后重建:
```bash
docker compose up -d --build
```

### 方案 B — 源码 build

需要 Go 1.23+ 和 **libvips**(8.13 或更新)。解码、缩放、WebP/AVIF 编码全走 libvips,不需要装其他格式工具。

```bash
# macOS
brew install vips

# Debian / Ubuntu(build 环境)
sudo apt install -y libvips-dev pkg-config build-essential
# Debian / Ubuntu(只运行已编译好的二进制)
sudo apt install -y libvips42

# Arch
sudo pacman -S libvips
```

build + 启动:
```bash
go build -o imget ./cmd/imget
cp .env.example .env
./imget serve
```

> 注意:这是 CGO 编译(govips 动态链接 libvips),产生的二进制依赖运行机上有 `libvips42`。

### 方案 C — 生产级 Linux 部署(systemd + nginx)

正式服务器上推荐用 Docker 跨平台 build 一份 Linux 二进制(保证 libvips ABI 匹配),扔进 `/opt/imget/`,配 systemd + nginx 反代。完整示例见下面的[服务器部署](#服务器部署)。

## API key(全部走 `.env`)

启动时通过 [godotenv](https://github.com/joho/godotenv) 从工作目录加载 `.env`,**没有单独的 secrets 文件、没有管理后台**。三家凭证全部 env 配置:

| 服务 | 必填 env | 说明 |
|---|---|---|
| **Pexels** | `PEXELS_API_KEY` | 免费,在 [pexels.com/api](https://www.pexels.com/api/) 注册。返回 `src.original`(真 4K-8K)。 |
| **Pixabay** | `PIXABAY_API_KEY`(可选 `PIXABAY_API_KEY_BACKUP`) | 免费标准版只到 1280px,要原图请 [申请 Full API](https://pixabay.com/service/about/api/),通过后**代码 0 改动**自动出 imageURL。 |
| **R2 / S3 兼容** | `R2_ACCESS_KEY_ID`、`R2_SECRET_ACCESS_KEY`、`R2_ENDPOINT`、`R2_BUCKET` | 前缀 "R2_" 是历史命名,任意 S3 兼容服务都行(Cloudflare R2、MinIO、SeaweedFS、Garage、AWS S3 …)。 |

Pexels/Pixabay 都留空 → 回落到 Picsum(占位图)。R2_* 全空 → 纯本地磁盘模式。

### 来源优先级

运行时 `source.Chain` 依次问每个 provider,**第一个有结果的胜出**:

```
Pexels(4K-8K src.original)
  ↓ 空?
Pixabay(默认 1280px largeImageURL;Full-API 通过后是 imageURL 4K-6K)
  ↓ 空?
Picsum(占位图,仅在请求明确指定宽高时启用)
```

壁纸入库(`topup-types`)路径会**跳过 Picsum**,避免污染分类池。

## 配置(`.env`)

所有运行参数都在 `.env`。**没有管理后台**。`cp .env.example .env` 后按需填,空值用默认值。

通常一定要设的:

| 字段 | 作用 |
|---|---|
| `SITE_BASE_URL` | 你的公网域名(如 `https://img.example.com`),首页代码示例、OG 标签、`<canonical>` 都用它。 |
| `SITE_NAME` | `<title>`、品牌、页脚显示。 |
| `SITE_TAGLINE` | 可选副标题(如 `图得`),空字符串不显示。 |
| `DMCA_EMAIL` | 页脚版权联系邮箱,空字符串隐藏。 |
| `PEXELS_API_KEY` | **主源**(4K-8K 原图)。空则 fallback 到 Pixabay。 |
| `PEXELS_MIN_WIDTH` | 丢弃 Pexels 上宽度小于此值的图。默认 `1920`,要 2K+ 设 `2560`,严格 4K 设 `3840`。 |
| `PIXABAY_API_KEY` | 兜底源。免费版 1280px。 |
| `R2_ENDPOINT` + `R2_ACCESS_KEY_ID` + `R2_SECRET_ACCESS_KEY` + `R2_BUCKET` | S3 兼容存储,留空 = 纯本地模式。 |
| `R2_CDN_BASE_URL` | 公网 CDN 根(如 `https://pic.example.com`)。 |

抓图节奏:

| 字段 | 默认 | 说明 |
|---|---|---|
| `INITIAL_PREFETCH_COUNT` | `5` | 新 (w,h,type) 组合首次访问下载几张种子图。 |
| `PER_VISIT_REFETCH` | `1` | 每次访问后台补几张;`0` 关闭。 |
| `TOPUP_TYPES_MIN_PER_TYPE` | `5` | `imget topup-types` 每分类随机最少几张。 |
| `TOPUP_TYPES_MAX_PER_TYPE` | `10` | 最多几张。设成 `30` / `50` 一次跑约 480-800 张。 |
| `DAILY_TOPUP_INCREMENT` | `10` | 旧 `daily-topup` 走 profile,只为兼容保留。 |
| `LOCAL_RENDER_CACHE_TTL_HOURS` | `168` | `cleanup-cache` 删多久前的渲染缓存。 |

输出:

| 字段 | 默认 | 说明 |
|---|---|---|
| `ENABLED_FORMATS` | `webp,avif` | 白名单,只填 `webp` 就禁用 AVIF。 |
| `DEFAULT_FORMAT` | `webp` | 请求不带 `?format=` 时的默认。 |
| `MAX_DIM` | `4000` | 用户请求宽高上限。 |

完整字段列表见 `.env.example`(HTTP 监听、DB 路径、R2 worker 数、analytics、允许的 type 白名单 …)。

## URL 路由

### 缩放模式 — `/{宽}/{高}`

主路由。`Sec-Fetch-Dest: image` 或 `?raw=1` 时直接返图;浏览器导航(`Accept: text/html`)时显示详情页。

```
GET /1920/1080
GET /1920/1080?type=landscape&format=webp&r=42&s=1
GET /800/600?type=animal&keyword=orange+tabby+cat
```

参数:
- `type` / `t` — 分类(默认 `banner`)
- `keyword` / `q` — 自定义搜索词(覆盖分类默认词)
- `format` / `f` — `webp` 或 `avif`
- `r` / `v` — 固定取图(同一 `r` 始终对应同一张源)
- `s` / `slot` — 位置(同 `r` 不同 `s` 拿同一文章的不同图位)
- `fresh=N` — 选图前先抓 N 张新的
- `download=1` / `raw=1` — 强制下载 / 强制返回原图

### 壁纸模式 — `/{分类}` 或 `/type/{分类}`

返回一张**原始分辨率**图(不缩放、不转格式)。R2 CDN 配置好时 302 跳转。同样有内容协商 — 浏览器看到预览页,`<img>` 拿到原图。

```
GET /landscape                       # 随机 4K-8K 风景,每次刷新换
GET /type/animal                     # 命名空间形式,等价
GET /landscape?r=42                  # 固定:同 r 同图,强缓存
GET /animal?keyword=orange+tabby     # 自定义关键词
```

### 其他路由

- `GET /files/{type}/{file}` — 直接拿文件(R2 上有的话 302 跳 CDN)。
- `GET /p/{type}/{file}`、`GET /p/files/{type}/{file}` — 已存文件的预览页。
- `GET /healthz` — 健康检查(DB 通就返 200 `ok`)。
- `GET /metrics` — Prometheus 指标(建议 nginx 限内网)。
- `GET /assets/*`、`/favicon.ico` — embed 进二进制的静态资源。

### 分类

20 个内置:`banner, landscape, beauty, anime, city, nature, car, game, food, animal, travel, space, tech, business, sports, architecture, wedding, kids, abstract, concert`。

每个分类有精心调过的关键词扩展(比如 `animal` → `"animal pet dog cat wildlife"`),所以单个分类也能拿到丰富的图。要更精确请用 `?keyword=` 覆盖,首页"自定义关键词"输入框就走这个参数。

## 命令行

```bash
imget serve              # HTTP 服务(默认)
imget topup-types        # 遍历 20 个分类,每个随机抓 N 张原图
imget daily-topup        # 旧:按 request_profiles 抓,每 profile 抓 N 张
imget r2-sync            # 把磁盘上还没上传的文件批量传 R2
imget cleanup-cache      # 清掉超过 TTL 的渲染缓存(原图永远保留)
imget import-r2          # 一次性:扫 R2 bucket 写入 source_images 表
imget r2-prune-variants  # 删 R2 里所有非 original/ 前缀的对象(--yes 实际执行)
imget r2-prune-orphans   # 删 R2 里 DB 不认识的对象(--yes 实际执行)
```

`topup-types`、`cleanup-cache`、`r2-sync` 建议跑 systemd timer 或 cron。示例(`/etc/systemd/system/imget-topup.timer`):

```ini
[Unit]
Description=Run imget topup-types once a day
Requires=imget-topup.service

[Timer]
OnCalendar=*-*-* 04:00:00 UTC
Persistent=true
RandomizedDelaySec=15m

[Install]
WantedBy=timers.target
```

配套的 `imget-topup.service`:
```ini
[Unit]
Description=imget daily topup (random N new originals per category)
After=imget.service network-online.target

[Service]
Type=oneshot
WorkingDirectory=/opt/imget
ExecStart=/opt/imget/imget topup-types
```

或者纯 crontab:
```cron
5 4 * * *   root  cd /opt/imget && ./imget topup-types  >> /var/log/imget-topup.log 2>&1
0 3 * * 0   root  cd /opt/imget && ./imget cleanup-cache --hours 168
0 5 1 * *   root  cd /opt/imget && ./imget r2-prune-orphans --yes
```

## 服务器部署

Debian / Ubuntu + nginx(或宝塔 BT 面板)的参考配置。

### systemd unit(`/etc/systemd/system/imget.service`)

```ini
[Unit]
Description=imget image service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/imget
ExecStart=/opt/imget/imget serve
Restart=on-failure
RestartSec=3
LimitNOFILE=65536
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/imget
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now imget
journalctl -u imget -f
```

### nginx vhost(HTTP + HTTPS)

```nginx
server {
    listen 80;
    listen 443 ssl;
    http2 on;
    server_name example.com www.example.com;

    ssl_certificate     /etc/ssl/example.com/fullchain.pem;
    ssl_certificate_key /etc/ssl/example.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    add_header Strict-Transport-Security "max-age=31536000" always;

    # ACME http-01,续证用,留 HTTP。
    location ^~ /.well-known/acme-challenge/ { root /var/www/example.com; }

    # 其余 HTTP -> HTTPS。
    if ($server_port = 80) { return 301 https://$host$request_uri; }

    client_max_body_size 50m;

    # /metrics 只允许 localhost,挡掉外部扫描。
    location = /metrics {
        allow 127.0.0.1;
        allow ::1;
        deny all;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 用 acme.sh 申请证书

```bash
curl https://get.acme.sh | sh -s email=admin@example.com
~/.acme.sh/acme.sh --issue -d example.com -d www.example.com \
    --webroot /var/www/example.com --server letsencrypt
~/.acme.sh/acme.sh --install-cert -d example.com \
    --key-file       /etc/ssl/example.com/privkey.pem \
    --fullchain-file /etc/ssl/example.com/fullchain.pem \
    --reloadcmd      "systemctl reload nginx"
```

## 项目结构

```
imget/
├── cmd/imget/                 CLI 入口 + 子命令注册
├── internal/
│   ├── config/                .env -> Config struct(godotenv)
│   ├── db/                    SQLite + WAL + 5 张表
│   │   └── migrations/        嵌入的 SQL
│   ├── encoder/               libvips 封装(解码 + 缩放 + 编码)
│   ├── imgpipe/               核心 pipeline:挑源 → 抓 → 渲染 → 上传
│   ├── jobs/                  子命令实现(topup-types、prune-orphans 等)
│   ├── r2/                    AWS SDK v2 封装(S3 兼容)
│   ├── server/                HTTP 路由 + handler + SiteContext
│   │   ├── static/            embed 的 CSS/JS/logo/favicon
│   │   └── templates/         6 个 embed 的 *.html.tmpl
│   │                          (home、detail、wallpaper、preparing、public-image、error)
│   └── source/                分类目录 + Pexels / Pixabay / Picsum providers
└── Dockerfile / docker-compose.yml
```

### SQLite 表结构

| 表 | 用途 |
|---|---|
| `request_profiles` | 每个 (w,h,type,keyword) 组合的请求/查看/下载计数,刷新状态。 |
| `url_cache` | 变体缓存,`?r=42` 永远拿到同一张源。 |
| `refresh_logs` | `topup-types` / `daily-topup` / 手动刷新的审计。 |
| `r2_uploads` | R2 真相表,`file_path` / `r2_key` / `cdn_url` / `file_size`。页脚统计就查这表。 |
| `source_images` | 由 `imget import-r2` 一次性 seed 的"冻结池",远端原图按需下载。 |

几百万行 SQLite 完全 hold 得住(`WAL` + 启动自动调好 pragma)。

## 开发

```bash
go test ./...
go vet ./...

# 想改模板看效果,设 TEMPLATES_DIR=./internal/server/templates,
# 二进制会从那读 *.html.tmpl 而不是 embed.FS。
TEMPLATES_DIR=./internal/server/templates ./imget serve
```

改了 CSS/JS/模板要 rebuild + 重启 +**记得把 `<link rel="stylesheet">` 里的 `?v=...` 缓存破坏字符串改一下**(`internal/server/templates/*.html.tmpl`),否则老客户端会复用缓存。

## License

MIT。
