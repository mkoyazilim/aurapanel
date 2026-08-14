CREATE TABLE IF NOT EXISTS cron_jobs (
    id          INTEGER PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    schedule    TEXT NOT NULL,  -- cron expression: "* * * * *"
    command     TEXT NOT NULL,  -- çalıştırılacak komut (shell opsiz, argüman listesi)
    label       TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    last_run_at TEXT,
    last_status TEXT            -- 'ok' | 'error' | null
);
CREATE INDEX IF NOT EXISTS idx_cron_site ON cron_jobs(site_id);
