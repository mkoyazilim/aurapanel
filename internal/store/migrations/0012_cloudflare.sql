CREATE TABLE cloudflare_settings (
    site_id TEXT PRIMARY KEY,
    api_token TEXT NOT NULL,
    zone_id TEXT NOT NULL,
    proxy_enabled BOOLEAN NOT NULL DEFAULT 1,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);
