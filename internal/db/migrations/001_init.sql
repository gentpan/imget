-- img.et schema (Go rewrite)
-- Single-tenant SQLite. All timestamps are unix seconds (INTEGER).

CREATE TABLE IF NOT EXISTS request_profiles (
    profile_key              TEXT PRIMARY KEY,
    width                    INTEGER NOT NULL,
    height                   INTEGER NOT NULL,
    type                     TEXT NOT NULL,
    keyword                  TEXT NOT NULL DEFAULT '',
    request_count            INTEGER NOT NULL DEFAULT 0,
    view_count               INTEGER NOT NULL DEFAULT 0,
    download_count           INTEGER NOT NULL DEFAULT 0,
    first_requested_at       INTEGER NOT NULL,
    last_requested_at        INTEGER NOT NULL,
    last_daily_topup_at      INTEGER,
    last_daily_topup_saved   INTEGER NOT NULL DEFAULT 0,
    initial_prefetch_done_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_profiles_last_requested
    ON request_profiles(last_requested_at DESC);

CREATE TABLE IF NOT EXISTS url_cache (
    cache_key     TEXT PRIMARY KEY,
    selected_path TEXT NOT NULL,
    updated_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_url_cache_path
    ON url_cache(selected_path);

CREATE TABLE IF NOT EXISTS refresh_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_key     TEXT NOT NULL,
    width           INTEGER NOT NULL,
    height          INTEGER NOT NULL,
    type            TEXT NOT NULL,
    keyword         TEXT NOT NULL DEFAULT '',
    mode            TEXT NOT NULL,
    requested_count INTEGER NOT NULL,
    saved_count     INTEGER NOT NULL,
    error_text      TEXT,
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_refresh_logs_created
    ON refresh_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_refresh_logs_profile
    ON refresh_logs(profile_key);

CREATE TABLE IF NOT EXISTS r2_uploads (
    file_path   TEXT PRIMARY KEY,
    r2_key      TEXT NOT NULL,
    cdn_url     TEXT NOT NULL,
    file_size   INTEGER NOT NULL DEFAULT 0,
    uploaded_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS source_images (
    r2_key      TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    sha1        TEXT NOT NULL,
    ext         TEXT NOT NULL,
    file_size   INTEGER NOT NULL DEFAULT 0,
    cdn_url     TEXT,
    imported_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_source_images_type
    ON source_images(type);
CREATE INDEX IF NOT EXISTS idx_source_images_sha1
    ON source_images(sha1);
