-- Cluster event log: health check, key rotation, site deploy olayları
CREATE TABLE IF NOT EXISTS cluster_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id  TEXT    NOT NULL,
    event_type TEXT    NOT NULL, -- health_ok | health_fail | key_rotated | site_created
    detail     TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_cluster_events_server ON cluster_events(server_id, id DESC);
