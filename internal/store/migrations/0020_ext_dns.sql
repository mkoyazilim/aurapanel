-- External DNS provider credentials (şifreli saklama)
CREATE TABLE IF NOT EXISTS ext_dns_providers (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,                  -- "cloudflare-main", "route53-prod" vb.
    provider    TEXT    NOT NULL,                  -- "cloudflare" | "route53"
    credentials TEXT    NOT NULL DEFAULT '{}',     -- AES-GCM şifreli JSON blob
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT
);

-- External DNS sync log
CREATE TABLE IF NOT EXISTS ext_dns_sync_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER NOT NULL REFERENCES ext_dns_providers(id) ON DELETE CASCADE,
    direction   TEXT    NOT NULL, -- "push" | "pull"
    action      TEXT    NOT NULL, -- "sync" | "conflict"
    detail      TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_ext_dns_sync_log ON ext_dns_sync_log(provider_id, id DESC);
