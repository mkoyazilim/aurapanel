CREATE TABLE IF NOT EXISTS site_security (
    site_id TEXT PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
    profile TEXT NOT NULL DEFAULT 'minimal'  -- 'minimal' | 'balanced' | 'hardened'
);
