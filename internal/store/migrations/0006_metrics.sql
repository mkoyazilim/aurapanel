CREATE TABLE IF NOT EXISTS metrics (
    id         INTEGER PRIMARY KEY,
    site_id    TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    ts         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    cpu_pct    REAL NOT NULL DEFAULT 0,   -- 0-100
    mem_mb     REAL NOT NULL DEFAULT 0,
    disk_mb    REAL NOT NULL DEFAULT 0,
    disk_inodes INTEGER NOT NULL DEFAULT 0,
    pids       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_metrics_site_ts ON metrics(site_id, ts);

-- Eski metrik satırlarını (>25 saat) otomatik temizle.
-- Panel periyodik olarak bu trigger yerine bir cleanup goroutine çalıştırır.
