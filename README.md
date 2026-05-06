# imget

A small, single-binary image service that turns `https://example.com/{w}/{h}` into a freshly-rendered random image. Originals come from Pixabay and Picsum; output is WebP (default) or AVIF; everything is cached locally on disk and optionally mirrored to Cloudflare R2 (or any S3-compatible store).

- **Single binary deploy** — no admin UI, no PHP, no runtime template files. CSS/JS/templates are embedded.
- **libvips-powered pipeline** — decode + resize + encode in one in-process call, no subprocess fork-exec overhead.
- **SQLite + on-disk cache** — works fine on a single VPS up to several million images.
- **Open-source friendly** — every visible string (site name, brand, copyright email, fetch counts) is overridable via `.env`.

## Quick start

### Option A — Docker Compose (no Go toolchain required)

```bash
git clone <your fork> && cd imgetgo
cp .env.example .env
# Edit .env — at minimum set SITE_BASE_URL; optional R2/Pixabay credentials.
docker compose up -d
# Service is on http://localhost:8080
```

The image is built from source on first run (~1 min) and includes `libvips42`. Persistent state lives in `./data/database` and `./data/images` on the host.

To rebuild after a code or `.env` change:
```bash
docker compose up -d --build
```

### Option B — Build from source

Requires Go 1.23+ and **libvips** (8.13 or later) on the build & runtime hosts. Image decoding, resizing and WebP/AVIF encoding all flow through libvips via govips — no per-format CLI tools needed.

```bash
# macOS
brew install vips

# Debian / Ubuntu (build)
sudo apt install -y libvips-dev pkg-config build-essential
# Debian / Ubuntu (runtime only — for the deployed binary)
sudo apt install -y libvips42

# Arch
sudo pacman -S libvips
```

Build and run:
```bash
go build -o imget ./cmd/imget
cp .env.example .env
./imget serve
```

> Note: the build is CGO-enabled (govips links libvips). The resulting binary is dynamically linked and requires `libvips42` on the runtime host.

### Option C — Production-grade Linux deploy (systemd + nginx)

For a real server, cross-build a Linux binary inside Docker (so its libvips ABI matches the runtime), drop it into `/opt/imget/`, register a systemd unit, and reverse-proxy via nginx:

```bash
# Cross-build linux/amd64 binary using the Dockerfile's builder stage
docker buildx build --platform linux/amd64 \
    --target builder \
    --output type=local,dest=./out .
# Binary lands at ./out/out/imget

# On the target server (one-time): apt install -y libvips42
scp ./out/out/imget .env user@host:/opt/imget/
```

A complete systemd unit + nginx vhost with HTTPS via acme.sh is below in [Server deployment](#server-deployment).

## Configuration (`.env`)

Every runtime knob lives in `.env`. There is no admin UI. Copy `.env.example` to `.env` and fill what you need — empty values fall back to documented defaults.

The keys you almost always want to set:

| Key | What it does |
|---|---|
| `SITE_BASE_URL` | Your public origin (e.g. `https://img.example.com`). Used in OG tags, code samples on the home page, `<canonical>`, etc. |
| `SITE_NAME` | Brand shown in `<title>`, footer, etc. |
| `SITE_TAGLINE` | Optional second-language subtitle (e.g. `图得`); empty hides it. |
| `DMCA_EMAIL` | Copyright contact in the footer. Empty hides the link entirely. |
| `R2_ENDPOINT` + `R2_ACCESS_KEY_ID` + `R2_SECRET_ACCESS_KEY` + `R2_BUCKET` | Cloudflare R2 credentials. Leave empty for local-only mode. |
| `R2_CDN_BASE_URL` | Public CDN root for uploaded files (e.g. `https://pic.example.com`). |
| `PIXABAY_API_KEY` | Optional; without it the service falls back to picsum.photos. |

Tuning knobs that matter:

| Key | Default | Notes |
|---|---|---|
| `ENABLED_FORMATS` | `webp,avif` | Allow-list. Set `webp` only to disable AVIF entirely. |
| `DEFAULT_FORMAT` | `webp` | Used when the request omits `?format=`. |
| `INITIAL_PREFETCH_COUNT` | `5` | First-time fetch for a new (w,h,type) combo. |
| `PER_VISIT_REFETCH` | `1` | Background top-up per visit; `0` to disable. |
| `DAILY_TOPUP_INCREMENT` | `10` | Used by `imget daily-topup`. |
| `DAILY_TOPUP_MAX_PER_TYPE` | `1000` | Cap per category. |
| `LOCAL_RENDER_CACHE_TTL_HOURS` | `168` | Used by `imget cleanup-cache`. |

See `.env.example` for the full list (HTTP address, DB path, R2 worker count, encoder paths, allowed types, analytics, …).

## URL patterns

Behavior matches the PHP version it replaces.

- `GET /{width}/{height}` — the main route. Returns an image when `?raw=1` is set or the browser sends `Sec-Fetch-Dest: image`; otherwise renders the HTML detail page.
- `GET /{width}/{height}?type=banner&format=webp&r=42&s=1`
  - `type` / `t` — image category (default `banner`)
  - `keyword` / `q` — explicit search term (overrides type's default)
  - `format` / `f` — `webp` or `avif`
  - `r` / `v` — persistent variant ID (same `r` always resolves to the same source)
  - `s` / `slot` — slot ID (different `s` with same `r` ⇒ different image)
  - `fresh=N` — refetch N images before selecting
  - `download=1` / `raw=1` — see above
- `GET /files/{type}/{file}` — direct file delivery (302 redirects to CDN if uploaded).
- `GET /p/{type}/{file}` and `GET /p/files/{type}/{file}` — public detail page for an existing file.
- `GET /healthz` — health check (200 + `ok` when the DB is reachable).
- `GET /assets/*` and `/favicon.ico` — embedded static assets.

The 16 built-in categories: `banner, landscape, beauty, anime, city, nature, car, game, food, animal, travel, space, tech, business, sports, architecture`.

## CLI

```bash
imget serve            # HTTP server (default)
imget daily-topup      # add N new originals to every active profile
imget r2-sync          # upload disk-resident files missing from R2
imget cleanup-cache    # delete rendered variants older than TTL
imget import-r2        # one-shot: list R2 bucket → seed source_images table
```

`daily-topup` and `cleanup-cache` are designed to run from `cron`. Example:

```cron
0 12 * * *   root  /opt/imget/imget daily-topup -n 10
0 4  * * *   root  /opt/imget/imget r2-sync
0 3  * * 0   root  /opt/imget/imget cleanup-cache --hours 168
```

## Server deployment

Reference setup for a Linux server (Debian / Ubuntu) running nginx (or behind 宝塔/BT panel).

### systemd unit (`/etc/systemd/system/imget.service`)

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

### nginx vhost (HTTP + HTTPS)

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

    # ACME http-01 — keep on plain HTTP for renewals.
    location ^~ /.well-known/acme-challenge/ { root /var/www/example.com; }

    # HTTP -> HTTPS for everything else.
    if ($server_port = 80) { return 301 https://$host$request_uri; }

    client_max_body_size 50m;

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

### TLS via acme.sh

```bash
curl https://get.acme.sh | sh -s email=admin@example.com
~/.acme.sh/acme.sh --issue -d example.com -d www.example.com \
    --webroot /var/www/example.com --server letsencrypt
~/.acme.sh/acme.sh --install-cert -d example.com \
    --key-file       /etc/ssl/example.com/privkey.pem \
    --fullchain-file /etc/ssl/example.com/fullchain.pem \
    --reloadcmd      "systemctl reload nginx"
```

## Architecture

```
imgetgo/
├── cmd/imget/                 CLI entry point
├── internal/
│   ├── config/                .env -> Config struct
│   ├── db/                    SQLite + WAL pragmas + 5 tables
│   │   └── migrations/        embedded SQL
│   ├── encoder/               libvips wrapper (decode + resize + encode)
│   ├── imgpipe/               core pipeline: select → fetch → render → upload
│   ├── jobs/                  CLI subcommand bodies
│   ├── r2/                    AWS SDK v2 wrapper (S3-compatible)
│   ├── server/                HTTP routes + handlers + SiteContext
│   │   ├── static/            embedded CSS/JS/logo/favicon
│   │   └── templates/         5 embedded *.html.tmpl files
│   └── source/                category catalog + Pixabay + Picsum providers
└── Dockerfile / docker-compose.yml
```

### Data tables (SQLite)

| Table | Purpose |
|---|---|
| `request_profiles` | Each unique (w,h,type,keyword) combination — request/view/download counters, refresh state. |
| `url_cache` | Variant cache — `?r=42` always resolves to the same source. |
| `refresh_logs` | Audit trail for `daily-topup`/manual refreshes. Pruned automatically. |
| `r2_uploads` | Track which files have been mirrored to R2. |
| `source_images` | R2 source pool — populated by `imget import-r2`; remote originals downloaded on demand. |

A few million rows fit comfortably; SQLite handles it with `WAL` + sane pragmas (set automatically on startup).

## Development

```bash
go test ./...
go vet ./...

# Override built-in templates by setting TEMPLATES_DIR=./templates
# (the binary will read *.html.tmpl from there instead of the embed.FS).
TEMPLATES_DIR=./internal/server/templates ./imget serve
```

When changing CSS/JS/templates: rebuild and restart — the embed is baked at compile time. There is no hot-reload by design (single-binary deploy is the trade).

## License

MIT.
