# imget

**English** · [简体中文](README.zh-CN.md)

A free image service. Turn one URL into a random photo, a wallpaper, or a sized placeholder. Self-hosted, MIT-licensed.

Live demo: **https://img.et**

## What it does

- **Random images by category** — `https://img.et/1920/1080?type=landscape` returns a fresh landscape photo each request, resized + format-converted on the fly.
- **Wallpaper mode** — `https://img.et/landscape` returns one original 4K-8K image (no resize), redirecting to a CDN. Perfect for wallpaper apps.
- **20 categories** — banner, landscape, beauty, anime, city, nature, car, game, food, animal, travel, space, tech, business, sports, architecture, wedding, kids, abstract, concert.
- **Custom search** — `?keyword=red+sports+car` overrides the category default and queries Pexels / Pixabay directly.
- **Fixed image** — `?r=42` always returns the same image for that URL (good for article hero images, themed wallpaper sets, caching).
- **Daily auto-refresh** — a built-in cron job pulls fresh originals (configurable per-category count) so the pool stays varied.
- **WebP / AVIF output** — modern formats by default, with `?format=webp|avif` override.

## Install (Docker, 3 commands)

```bash
git clone https://github.com/gentpan/imget.git && cd imget
cp .env.example .env       # then edit: see "Configure" below
docker compose up -d
```

Service is on `http://localhost:8080`. Persistent data lives in `./data/`.

To rebuild after editing `.env` or pulling new code:
```bash
docker compose up -d --build
```

## Configure

Edit `.env`. The minimum to get real photos:

```ini
# Your public domain (used in OG tags, code samples, etc.)
SITE_BASE_URL=https://img.example.com

# Pexels — primary image source (4K-8K originals). Free, register at:
#   https://www.pexels.com/api/
PEXELS_API_KEY=xxxxxxxxxxxxxxxxxxxx

# Pixabay — fallback source. Free, register at:
#   https://pixabay.com/api/docs/
PIXABAY_API_KEY=xxxxxxxx-xxxxxxxxxxxxxxx

# S3-compatible storage for the image library (R2 / MinIO / SeaweedFS / S3).
# Leave blank to run in local-disk-only mode.
R2_ENDPOINT=https://your-s3-host
R2_ACCESS_KEY_ID=xxx
R2_SECRET_ACCESS_KEY=xxx
R2_BUCKET=imget
R2_CDN_BASE_URL=https://cdn.example.com
```

Other useful knobs (all have sensible defaults):

| Key | Default | What it does |
|---|---|---|
| `SITE_NAME` | `imget` | Brand shown in title / footer. |
| `DEFAULT_FORMAT` | `webp` | `webp` or `avif`. |
| `MAX_DIM` | `4000` | Cap on requested width/height. |
| `INITIAL_PREFETCH_COUNT` | `5` | New (size, type) combo prefetches this many on first hit. |
| `TOPUP_TYPES_MIN_PER_TYPE` / `MAX_PER_TYPE` | `5` / `10` | Daily auto-fetch picks a random number in this range, per category. Set `30` / `50` for ~500-800 fresh images/day. |
| `PEXELS_MIN_WIDTH` | `1920` | Reject Pexels images narrower than this. Set `2560` for 2K+, `3840` for strict 4K. |

Full list in `.env.example`. All credentials live in `.env` — there is no separate secrets file or admin UI.

## URL patterns

```
https://img.et/1920/1080                          # random image, 1920×1080, WebP
https://img.et/1920/1080?type=landscape           # specific category
https://img.et/800/600?type=car&format=avif       # custom format
https://img.et/1920/1080?r=42                     # fixed: same URL always returns the same image
https://img.et/1920/1080?r=42&s=2                 # second image of the same article
https://img.et/800/600?keyword=red+ferrari        # custom search keyword

https://img.et/landscape                          # wallpaper mode: original 4K-8K
https://img.et/landscape?r=42                     # fixed wallpaper
https://img.et/type/landscape                     # equivalent namespaced form
```

Each category card on the home page links to its wallpaper URL — open the homepage and click around to see them in action.

## Daily auto-refresh

The package ships with a CLI subcommand that pulls fresh images for every category in one go:

```bash
# Inside the container, or on the host if running natively:
imget topup-types
```

Schedule it with cron or a systemd timer. Example timer running daily at 04:00 UTC:

```ini
# /etc/systemd/system/imget-topup.service
[Service]
Type=oneshot
WorkingDirectory=/opt/imget
ExecStart=/opt/imget/imget topup-types

# /etc/systemd/system/imget-topup.timer
[Timer]
OnCalendar=*-*-* 04:00:00 UTC
Persistent=true

[Install]
WantedBy=timers.target
```

```bash
systemctl enable --now imget-topup.timer
```

## CLI summary

```bash
imget serve              # HTTP server (default; what docker-compose runs)
imget topup-types        # daily auto-refresh — random N images per category
imget cleanup-cache      # delete rendered variants older than TTL (originals safe)
imget r2-sync            # upload disk-resident files missing from S3
imget r2-prune-orphans   # delete S3 objects not referenced by the DB (--yes for real)
imget import-r2          # one-shot: index an existing S3 bucket into the catalog
```

## License

MIT.
