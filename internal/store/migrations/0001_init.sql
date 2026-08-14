-- AuraPanel şema v1 (ARCHITECTURE §4.1)
-- Tarih: 2026-08-14
-- Tüm zaman damgaları ISO 8601 UTC metindir.

CREATE TABLE roles (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    permissions TEXT NOT NULL DEFAULT '[]',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE users (
    id                   INTEGER PRIMARY KEY,
    username             TEXT NOT NULL UNIQUE,
    password_hash        TEXT NOT NULL,
    role_id              INTEGER NOT NULL REFERENCES roles(id),
    totp_secret_enc      TEXT,
    webauthn_creds       TEXT,
    must_change_password INTEGER NOT NULL DEFAULT 1,
    status               TEXT NOT NULL DEFAULT 'active',
    last_login_at        TEXT,
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip         TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    csrf_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE sites (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL UNIQUE,
    linux_user          TEXT NOT NULL UNIQUE,
    home_dir            TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'creating',
    php_version_id      INTEGER REFERENCES php_versions(id),
    security_profile_id INTEGER REFERENCES security_profiles(id),
    feature_flags       TEXT NOT NULL DEFAULT '{}',
    limits              TEXT NOT NULL DEFAULT '{}',
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE domains (
    id          INTEGER PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    domain      TEXT NOT NULL UNIQUE,
    kind        TEXT NOT NULL DEFAULT 'main',
    ssl_enabled INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_domains_site ON domains(site_id);

CREATE TABLE ssl_certificates (
    id              INTEGER PRIMARY KEY,
    site_id         TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    domain_id       INTEGER REFERENCES domains(id) ON DELETE CASCADE,
    issuer          TEXT NOT NULL DEFAULT 'letsencrypt',
    cert_path       TEXT,
    key_path        TEXT,
    not_before      TEXT,
    not_after       TEXT,
    auto_renew      INTEGER NOT NULL DEFAULT 1,
    last_renewed_at TEXT,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_ssl_site ON ssl_certificates(site_id);

CREATE TABLE php_versions (
    id          INTEGER PRIMARY KEY,
    version     TEXT NOT NULL UNIQUE,
    binary_path TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'available',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE php_pools (
    id             INTEGER PRIMARY KEY,
    site_id        TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    php_version_id INTEGER NOT NULL REFERENCES php_versions(id),
    uid            TEXT NOT NULL,
    gid            TEXT NOT NULL,
    cgroup         TEXT NOT NULL,
    settings       TEXT NOT NULL DEFAULT '{}',
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE databases (
    id         INTEGER PRIMARY KEY,
    site_id    TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name       TEXT NOT NULL UNIQUE,
    charset    TEXT NOT NULL DEFAULT 'utf8mb4',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE database_users (
    id           INTEGER PRIMARY KEY,
    site_id      TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    username     TEXT NOT NULL UNIQUE,
    password_enc TEXT NOT NULL,
    host         TEXT NOT NULL DEFAULT 'localhost',
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE sftp_accounts (
    id         INTEGER PRIMARY KEY,
    site_id    TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    username   TEXT NOT NULL UNIQUE,
    jail_path  TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE cron_jobs (
    id          INTEGER PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    schedule    TEXT NOT NULL,
    command     TEXT NOT NULL,
    args        TEXT NOT NULL DEFAULT '[]',
    status      TEXT NOT NULL DEFAULT 'enabled',
    last_run_at TEXT,
    next_run_at TEXT,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_cron_site ON cron_jobs(site_id);

CREATE TABLE backups (
    id          INTEGER PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL DEFAULT 'full',
    storage     TEXT NOT NULL DEFAULT 'local',
    location    TEXT,
    encrypted   INTEGER NOT NULL DEFAULT 1,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    finished_at TEXT
);
CREATE INDEX idx_backups_site ON backups(site_id);

-- Append-only: UPDATE ve DELETE trigger'larla reddedilir (İlke 13).
CREATE TABLE audit_logs (
    id         INTEGER PRIMARY KEY,
    timestamp  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    user       TEXT NOT NULL DEFAULT 'system',
    ip         TEXT NOT NULL DEFAULT '',
    action     TEXT NOT NULL,
    target     TEXT NOT NULL DEFAULT '',
    result     TEXT NOT NULL DEFAULT 'success',
    request_id TEXT NOT NULL,
    extra      TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_audit_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_action ON audit_logs(action);

CREATE TRIGGER trg_audit_no_update BEFORE UPDATE ON audit_logs
BEGIN
    SELECT RAISE(ABORT, 'audit_logs append-only: UPDATE yasak');
END;

CREATE TRIGGER trg_audit_no_delete BEFORE DELETE ON audit_logs
BEGIN
    SELECT RAISE(ABORT, 'audit_logs append-only: DELETE yasak');
END;

CREATE TABLE security_profiles (
    id       INTEGER PRIMARY KEY,
    name     TEXT NOT NULL UNIQUE,
    settings TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE system_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE drift_events (
    id          INTEGER PRIMARY KEY,
    site_id     TEXT REFERENCES sites(id) ON DELETE CASCADE,
    resource    TEXT NOT NULL,
    expected    TEXT NOT NULL,
    actual      TEXT NOT NULL,
    severity    TEXT NOT NULL DEFAULT 'warning',
    status      TEXT NOT NULL DEFAULT 'open',
    detected_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    resolved_at TEXT
);
CREATE INDEX idx_drift_status ON drift_events(status);

CREATE TABLE metrics (
    id      INTEGER PRIMARY KEY,
    site_id TEXT REFERENCES sites(id) ON DELETE CASCADE,
    metric  TEXT NOT NULL,
    value   REAL NOT NULL,
    ts      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_metrics_site_ts ON metrics(site_id, ts);

-- Seed veriler: roller ve güvenlik profilleri (ARCHITECTURE §10).
INSERT INTO roles (name, permissions) VALUES
    ('admin', '["*"]'),
    ('user',  '[]');

INSERT INTO security_profiles (name, settings) VALUES
    ('compatibility', '{}'),
    ('balanced',      '{"open_basedir":true,"disable_functions":"common","private_tmp":true,"dir_listing":false}'),
    ('hardened',      '{"open_basedir":true,"disable_functions":"strict","private_tmp":true,"dir_listing":false,"waf":"basic","session_hardening":true}');
