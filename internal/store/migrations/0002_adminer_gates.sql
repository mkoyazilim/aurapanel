-- AuraPanel şema v2: Adminer gate (W8)
-- Geçici, scope kısıtlı Adminer oturumları. TOKEN HASH saklanır — ham token
-- yalnızca açılış anında panel tarafından gösterilir.

CREATE TABLE adminer_gates (
    id          INTEGER PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    database_id INTEGER REFERENCES databases(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_gates_expiry ON adminer_gates(expires_at);
