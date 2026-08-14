-- Migration 0008: cron_jobs tablosunu standart şemaya migrate et
CREATE TABLE IF NOT EXISTS _cron_jobs_new (
    id          INTEGER PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    schedule    TEXT NOT NULL,
    command     TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    last_status TEXT,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    last_run_at TEXT
);

INSERT INTO _cron_jobs_new (id, site_id, schedule, command, created_at, last_run_at)
SELECT id, site_id, schedule, command, created_at, last_run_at FROM cron_jobs;

DROP TABLE cron_jobs;
ALTER TABLE _cron_jobs_new RENAME TO cron_jobs;
CREATE INDEX IF NOT EXISTS idx_cron_site ON cron_jobs(site_id);
