# Server Version Snapshot

This records the production state observed on `144.76.120.45`
(hostname `image-server`) on 2026-07-11.

> Note: this server replaced the earlier host `91.99.237.99` (itself migrated
> from `8.217.*`). It is a **shared box** — it also runs `caddy`, `bujing-api`,
> `tujing-server`, and `picbi-server`. Keep that in mind when tuning resource
> limits. RAM: 62 GiB.

## Git state (local / GitHub)

- Local branch: `main`
- GitHub default branch: `main`
- Local/GitHub commit: `c670760cb34ee8634734e4ab8fa6d8872c079db3` ("Localize home brand name")
- Local working tree is clean and in sync with `origin/main`.

## Production services

- `imget.service`: `/opt/imget/imget serve` — active, listens on `127.0.0.1:18081`
- `imget-format-proxy.service`: `/usr/bin/python3 /opt/imget-format-proxy/imget-format-proxy.py`
  — active, listens on `127.0.0.1:18080`
- Public traffic is fronted by `caddy` (`:80`).
- `imget-topup.timer` is installed (daily Pixabay/Pexels top-up).
- `imget-rsync.service` exists but is `failed` (one-off image migration from the
  old host; leftover, no longer needed).

The deployed service files are tracked in `deploy/systemd/`.
The deployed proxy script is tracked in `deploy/imget-format-proxy.py`.

## Consistency vs. tracked files

- **Proxy script**: deployed `/opt/imget-format-proxy/imget-format-proxy.py`
  matches `deploy/imget-format-proxy.py` exactly (md5 `24ab1596ad3087dcf4e49be34fd39743`). ✅
- **systemd units**: semantically match `deploy/systemd/` (only blank-line
  formatting differs). `GOMEMLIMIT` was raised from `2GiB` to `10GiB` on
  2026-07-11 so the server now matches the tracked file (62 GiB RAM, low load,
  other co-hosted services use very little). ✅

## Storage layout

- Image store lives on the 2 TB RAID array: `/data/imget/images` (~12 GiB,
  5323 files). `/opt/imget/images` is a **symlink** to it, so `IMAGES_DIR`
  in `.env` stays `/opt/imget/images` (no config drift). Moved off the NVMe
  root on 2026-07-11.
- SQLite DB and the binary remain on the NVMe root (`/opt/imget/`).

## Production binary

Observed binary:

```text
/opt/imget/imget
size: 37549728 bytes
sha256: 2672a4e6934448d25832c4d43fa92bad8a96944ba7b5d6eb4dd6690acb41e35f
mtime: 2026-06-20 07:33:48 UTC
Go: go1.26.2
module: imget (devel)   # built without VCS stamping — no vcs.revision embedded
```

### Which commit is deployed?

The binary has no embedded `vcs.revision`, so it cannot be cryptographically
tied to a commit. By build time it lines up with commit **`69e14ab`**
("Keep language flag colors on hover", 2026-06-20 07:30:00 UTC) and predates the
current head **`c670760`** ("Localize home brand name", 2026-06-20 07:34:16 UTC)
by ~28 seconds.

**=> Production is one commit behind `main`.** The deployed binary does not
include `c670760`, which touches `internal/server/copyright.go`,
`internal/server/locale.go`, and the home i18n template. To bring the server to
head, rebuild `c670760` for `linux/amd64` (CGO enabled — sqlite) and redeploy
`/opt/imget/imget`.

## Operations performed 2026-07-11

- Removed redundant backup/junk files from the deploy dirs (kept only live files):
  - `/opt/imget/database/`: 3 stale `imget.sqlite.bak.*` snapshots (~38 MB) and
    an empty `imget.db` (the live DB is `imget.sqlite`, per `.env` `DB_PATH`).
  - `/opt/imget/.env.bak.pre-img-et-local`.
  - `/opt/imget-format-proxy/`: 6 `imget-format-proxy.py.bak*` copies and the
    `__pycache__/` directory (stale 3.12 + 3.13 caches).
- Raised `GOMEMLIMIT` 2GiB → 10GiB in `imget.service` (daemon-reload + restart).
- Moved the image store from the NVMe root to the 2 TB RAID array
  (`/data/imget/images`) via two-pass rsync; `/opt/imget/images` is now a
  symlink. Freed ~12 GiB on `/`. Verified end-to-end: `/files/…` serves an
  image `HTTP 200` from the new location.
