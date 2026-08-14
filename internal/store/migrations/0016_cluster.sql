-- Add servers table for cluster logic
CREATE TABLE IF NOT EXISTS servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    api_key TEXT NOT NULL,
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Add server_id to sites for cluster logic
ALTER TABLE sites ADD COLUMN server_id TEXT REFERENCES servers(id) ON DELETE SET NULL;
