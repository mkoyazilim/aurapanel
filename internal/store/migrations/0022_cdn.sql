-- CDN cache istatistikleri (site başına hit/miss)
CREATE TABLE IF NOT EXISTS cdn_stats (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id    TEXT    NOT NULL,
    source     TEXT    NOT NULL DEFAULT 'ols', -- "ols" | "cloudflare"
    hits       INTEGER NOT NULL DEFAULT 0,
    misses     INTEGER NOT NULL DEFAULT 0,
    purges     INTEGER NOT NULL DEFAULT 0,
    recorded_at TEXT   NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_cdn_stats ON cdn_stats(site_id, id DESC);

-- CF Cache kuralları (page rules alternatifi)
CREATE TABLE IF NOT EXISTS cdn_cache_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id     TEXT    NOT NULL,
    pattern     TEXT    NOT NULL, -- URL pattern "example.com/static/*"
    cache_level TEXT    NOT NULL DEFAULT 'standard', -- "bypass"|"standard"|"aggressive"
    ttl         INTEGER NOT NULL DEFAULT 0, -- saniye; 0=CF default
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
