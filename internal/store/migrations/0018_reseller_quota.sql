CREATE TABLE IF NOT EXISTS reseller_quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reseller_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_sites INTEGER NOT NULL DEFAULT 10,
    max_databases INTEGER NOT NULL DEFAULT 20,
    max_disk_gb INTEGER NOT NULL DEFAULT 100,
    max_bandwidth_gb INTEGER NOT NULL DEFAULT 500,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT,
    UNIQUE(reseller_id)
);
