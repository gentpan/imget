-- Provenance for every locally-downloaded original: which provider it came
-- from and the exact upstream URL. Keyed by the file's SHA1 (the on-disk
-- filename stem), so display code can look it up from the served path.
CREATE TABLE IF NOT EXISTS image_sources (
    sha1        TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    provider    TEXT NOT NULL,
    source_url  TEXT NOT NULL DEFAULT '',
    fetched_at  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_image_sources_provider
    ON image_sources(provider);
