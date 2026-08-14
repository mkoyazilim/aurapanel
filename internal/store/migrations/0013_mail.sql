CREATE TABLE mail_domains (
    domain TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE mail_accounts (
    email TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    quota_mb INTEGER NOT NULL DEFAULT 512,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    FOREIGN KEY(domain) REFERENCES mail_domains(domain) ON DELETE CASCADE
);
