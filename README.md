# imget

**English** · [简体中文](README.zh-CN.md)

A small, single-binary image service that turns `https://example.com/{w}/{h}` into a freshly-rendered random image, **and** `https://example.com/{type}` into an original-resolution wallpaper. Originals come from Pexels (primary, 4K–8K), Pixabay (fallback), and Picsum (procedural placeholder); output is WebP (default) or AVIF; everything is cached on disk and mirrored to Cloudflare R2 or any S3-compatible store.

- **Single binary deploy** — no admin UI, no PHP, no runtime template files. CSS/JS/templates are embedded.
- **libvips-powered pipeline** — decode + resize + encode in one in-process call, no subprocess fork-exec overhead.
- **SQLite + on-disk cache** — works fine on a single VPS up to several million images.
- **20 curated categories** + free-text `?keyword=` override for arbitrary searches.
- **Wallpaper mode** — `/{type}` returns the raw 4K-8K original (302 to CDN), perfect for wallpaper apps.
- **Open-source friendly** — every visible string (site name, brand, copyright email, fetch counts) is overridable via `.env`.

## Quick start

### Option A — Docker Compose (no Go toolchain required)

```bash
git clone https://github.com/gentpan/imget.git && cd imget
cp .env.example .env
# Edit .env — at minimum set SITE_BASE_URL.
# Set at least one of PEXELS_API_KEY / PIXABAY_API_KEY for real photos.
docker compose up -d
# Service is on http://localhost:8080
```

The image is built from source on first run (~1 min) and includes `libvips42`. Persistent state lives in `./data/database` and `./data/images` on the host.

To rebuild after a code or `.env` change:
```bash
docker compose up -d --build
```

### Option B — Build from source

Requires Go 1.26+ (set in `go.mod`) and **libvips** (8.13 or later) on the build & runtime hosts. Image decoding, resizing and WebP/AVIF encoding all flow through libvips via govips — no per-format CLI tools needed.

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

For a real server, cross-build a Linux binary inside Docker (so its libvips ABI matches the runtime), drop it into `/opt/imget/`, register a systemd unit, and reverse-proxy via nginx. A full reference (systemd unit, nginx vhost, acme.sh TLS, daily-topup timer) lives in [Server deployment](#server-deployment) below.

## API keys (all via `.env`)

The binary loads `.env` from its working directory at startup via [godotenv](https://github.com/joho/godotenv); there is no separate secrets file or admin UI. **All three provider credentials are env-based:**

| Service | Required env keys | Notes |
|---|---|---|
| **Pexels** | `PEXELS_API_KEY` | Free, [register here](https://www.pexels.com/api/). Returns `src.original` (true 4K-8K). |
| **Pixabay** | `PIXABAY_API_KEY` (+ optional `PIXABAY_API_KEY_BACKUP`) | Free standard tier caps at 1280px; apply for [Full API access](https://pixabay.com/service/about/api/) to unlock originals — no code change needed. |
| **R2 / S3-compatible** | `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_ENDPOINT`, `R2_BUCKET` | The "R2_" prefix is historical — points at any S3-compatible endpoint (Cloudflare R2, MinIO, SeaweedFS, self-hosted Garage, AWS S3 …). |

Leave Pexels/Pixabay blank to fall back to Picsum (procedural placeholders). Leave the R2_* block blank to run in pure local-disk mode.

### Provider chain

At runtime `source.Chain` queries providers in order and returns the **first non-empty result**:

```
Pexels (4K-8K src.original)
  ↓ empty?
Pixabay (1280px largeImageURL, or 4K imageURL if Full-API approved)
  ↓ empty?
Picsum (procedural; only used when explicit W/H is set)
```

For wallpaper ingestion (the `topup-types` job) Picsum is skipped entirely — placeholder images don't belong in a category pool.

## Configuration (`.env`)

Every runtime knob lives in `.env`. There is no admin UI. Copy `.env.example` to `.env` and fill what you need — empty values fall back to documented defaults.

The keys you almost always want to set:

| Key | What it does |
|---|---|
| `SITE_BASE_URL` | Your public origin (e.g. `https://img.example.com`). Used in OG tags, code samples on the home page, `<canonical>`, etc. |
| `SITE_NAME` | Brand shown in `<title>`, footer, etc. |
| `SITE_TAGLINE` | Optional second-language subtitle (e.g. `图得`); empty hides it. |
| `DMCA_EMAIL` | Copyright contact in the footer. Empty hides the link entirely. |
| `PEXELS_API_KEY` | **Primary source** of high-res originals. Without it, falls back to Pixabay. |
| `PEXELS_MIN_WIDTH` | Drop Pexels hits narrower than this. Default `1920`; raise to `2560` / `3840` for stricter 2K / 4K. |
| `PIXABAY_API_KEY` | Fallback source. Free tier = 1280px. |
| `R2_ENDPOINT` + `R2_ACCESS_KEY_ID` + `R2_SECRET_ACCESS_KEY` + `R2_BUCKET` | S3-compatible storage. Leave empty for local-only mode. |
| `R2_CDN_BASE_URL` | Public CDN root for uploaded files (e.g. `https://pic.example.com`). |

Ingestion tuning:

| Key | Default | Notes |
|---|---|---|
| `INITIAL_PREFETCH_COUNT` | `5` | First-time fetch for a new (w,h,type) combo. |
| `PER_VISIT_REFETCH` | `1` | Background top-up per visit; `0` to disable. |
| `TOPUP_TYPES_MIN_PER_TYPE` | `5` | Lower bound of `imget topup-types` per-category random count. |
| `TOPUP_TYPES_MAX_PER_TYPE` | `10` | Upper bound. Set `30` / `50` to add ~480-800 originals per daily run. |
| `DAILY_TOPUP_INCREMENT` | `10` | Legacy `daily-topup` only; profile-based. |
| `LOCAL_RENDER_CACHE_TTL_HOURS` | `168` | Used by `imget cleanup-cache`. |

Output:

| Key | Default | Notes |
|---|---|---|
| `ENABLED_FORMATS` | `webp,avif` | Allow-list. Set `webp` only to disable AVIF entirely. |
| `DEFAULT_FORMAT` | `webp` | Used when the request omits `?format=`. |
| `MAX_DIM` | `4000` | Cap on user-requested width/height. |

See `.env.example` for the full list (HTTP address, DB path, R2 worker count, encoder paths, allowed types, analytics, …).

## URL patterns

### Resize mode — `/{width}/{height}`

The main route. Returns an image when `?raw=1` is set or the browser sends `Sec-Fetch-Dest: image`; otherwise renders the HTML detail page.

```
GET /1920/1080
GET /1920/1080?type=landscape&format=webp&r=42&s=1
GET /800/600?type=animal&keyword=orange+tabby+cat
```

Query params:
- `type` / `t` — image category (default `banner`)
- `keyword` / `q` — free-text search override (overrides type's default keyword)
- `format` / `f` — `webp` or `avif`
- `r` / `v` — persistent variant ID (same `r` always resolves to the same source)
- `s` / `slot` / `slot_id` — slot ID (different `s` with same `r` ⇒ different image at the same article)
- `fresh=N` — refetch N images before selecting
- `download=1` / `raw=1` — force file download / force raw image even on browser navigation

### Wallpaper mode — `/{type}` or `/type/{type}`

Returns one **original-resolution** image (no resize, no format conversion). 302 redirects to the R2 CDN URL when configured. Same content-negotiation as the resize route — browsers get an HTML preview, `<img>` tags get the raw image.

```
GET /landscape                       # random 4K-8K landscape, fresh each hit
GET /type/animal                     # same thing, namespaced form
GET /landscape?r=42                  # fixed: same r => same wallpaper, strong cache
GET /animal?keyword=orange+tabby     # custom keyword
```

### Other routes

- `GET /files/{type}/{file}` — direct file delivery (302 redirects to CDN if uploaded).
- `GET /p/{type}/{file}` and `GET /p/files/{type}/{file}` — public detail page for an existing file.
- `GET /healthz` — health check (200 + `ok` when the DB is reachable).
- `GET /metrics` — Prometheus metrics (restrict via nginx ACL).
- `GET /assets/*` and `/favicon.ico` — embedded static assets.

### Categories

The 20 built-in categories: `banner, landscape, beauty, anime, city, nature, car, game, food, animal, travel, space, tech, business, sports, architecture, wedding, kids, abstract, concert`.

Each category has a curated keyword expansion (e.g. `animal` → `"animal pet dog cat wildlife"`), so the same category surfaces a diverse range. For narrower searches use `?keyword=` to override. The home page exposes a "自定义关键词" input that wires up to the same param.

## CLI

```bash
imget serve              # HTTP server (default)
imget topup-types        # iterate all 20 categories, fetch random N per type
imget daily-topup        # legacy: walk request_profiles, add N per profile
imget r2-sync            # upload disk-resident files missing from R2
imget cleanup-cache      # delete rendered variants older than TTL (originals safe)
imget import-r2          # one-shot: list R2 bucket → seed source_images table
imget r2-prune-variants  # delete every R2 object NOT under original/ (--yes for real)
imget r2-prune-orphans   # delete R2 objects not referenced by the DB (--yes for real)
```

`topup-types`, `cleanup-cache`, and `r2-sync` are designed to run from `cron` or a systemd timer. Example timer (`/etc/systemd/system/imget-topup.timer`):

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

With matching `imget-topup.service`:
```ini
[Unit]
Description=imget daily topup (random N new originals per category)
After=imget.service network-online.target

[Service]
Type=oneshot
WorkingDirectory=/opt/imget
ExecStart=/opt/imget/imget topup-types
```

Or as a plain `crontab`:
```cron
5 4 * * *   root  cd /opt/imget && ./imget topup-types  >> /var/log/imget-topup.log 2>&1
0 3 * * 0   root  cd /opt/imget && ./imget cleanup-cache --hours 168
0 5 1 * *   root  cd /opt/imget && ./imget r2-prune-orphans --yes
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

    # /metrics is public to localhost only — keep it that way.
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
imget/
├── cmd/imget/                 CLI entry point + subcommand wiring
├── internal/
│   ├── config/                .env -> Config struct (godotenv)
│   ├── db/                    SQLite + WAL pragmas + 5 tables
│   │   └── migrations/        embedded SQL
│   ├── encoder/               libvips wrapper (decode + resize + encode)
│   ├── imgpipe/               core pipeline: select → fetch → render → upload
│   ├── jobs/                  CLI subcommand bodies (topup-types, prune-orphans, …)
│   ├── r2/                    AWS SDK v2 wrapper (S3-compatible)
│   ├── server/                HTTP routes + handlers + SiteContext
│   │   ├── static/            embedded CSS/JS/logo/favicon
│   │   └── templates/         6 embedded *.html.tmpl files
│   │                          (home, detail, wallpaper, preparing, public-image, error)
│   └── source/                category catalog + Pexels + Pixabay + Picsum providers
└── Dockerfile / docker-compose.yml
```

### Data tables (SQLite)

| Table | Purpose |
|---|---|
| `request_profiles` | Each unique (w,h,type,keyword) combination — request/view/download counters, refresh state. |
| `url_cache` | Variant cache — `?r=42` always resolves to the same source. |
| `refresh_logs` | Audit trail for `topup-types` / `daily-topup` / manual refreshes. |
| `r2_uploads` | Source of truth for what's in R2 — `file_path`, `r2_key`, `cdn_url`, `file_size`. Footer stats query this table. |
| `source_images` | Original "frozen" pool seeded by `imget import-r2`; remote originals downloaded on demand. |

A few million rows fit comfortably; SQLite handles it with `WAL` + sane pragmas (set automatically on startup).

## Development

```bash
go test ./...
go vet ./...

# Override built-in templates by setting TEMPLATES_DIR=./templates
# (the binary will read *.html.tmpl from there instead of the embed.FS).
TEMPLATES_DIR=./internal/server/templates ./imget serve
```

When changing CSS/JS/templates: rebuild, restart, **and bump the `?v=...` cache-buster in the `<link rel="stylesheet">` tags** in `internal/server/templates/*.html.tmpl` so clients refetch.

## License

MIT.
