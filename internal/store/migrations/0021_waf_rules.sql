-- WAF kural setleri (site başına özel kurallar + OWASP CRS profil seçimi)
CREATE TABLE IF NOT EXISTS waf_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id     TEXT    NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    rule_id     TEXT    NOT NULL,                -- kullanıcı tanımlı "rule-001" vb.
    phase       INTEGER NOT NULL DEFAULT 2,       -- ModSecurity phase 1-5
    action      TEXT    NOT NULL DEFAULT 'deny',  -- "deny" | "allow" | "log"
    pattern     TEXT    NOT NULL,                -- regex / değer
    description TEXT    NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(site_id, rule_id)
);

-- WAF istek log (kural eşleşmeleri — ring buffer, son 1000)
CREATE TABLE IF NOT EXISTS waf_request_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id    TEXT    NOT NULL,
    rule_id    TEXT    NOT NULL DEFAULT '',
    action     TEXT    NOT NULL DEFAULT 'deny',
    client_ip  TEXT    NOT NULL DEFAULT '',
    uri        TEXT    NOT NULL DEFAULT '',
    method     TEXT    NOT NULL DEFAULT '',
    dry_run    INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_waf_request_log ON waf_request_log(site_id, id DESC);

-- OWASP CRS sürüm ve aktif profil (global)
CREATE TABLE IF NOT EXISTS waf_crs_config (
    id          INTEGER PRIMARY KEY CHECK (id=1),
    crs_version TEXT    NOT NULL DEFAULT '3.3.5',
    paranoia    INTEGER NOT NULL DEFAULT 1,   -- 1-4
    dry_run     INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT
);
INSERT OR IGNORE INTO waf_crs_config (id, crs_version, paranoia) VALUES (1, '3.3.5', 1);
