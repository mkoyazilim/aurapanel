CREATE TABLE staging_environments (
    id TEXT PRIMARY KEY,
    production_site_id TEXT NOT NULL,
    staging_site_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(production_site_id) REFERENCES sites(id) ON DELETE CASCADE,
    FOREIGN KEY(staging_site_id) REFERENCES sites(id) ON DELETE CASCADE
);
