CREATE TABLE git_deployments (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL UNIQUE,
    repo_url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT 'main',
    deploy_path TEXT NOT NULL DEFAULT '/',
    webhook_secret TEXT NOT NULL,
    deploy_script TEXT NOT NULL,
    last_deployed_at TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE INDEX idx_git_deployments_site_id ON git_deployments(site_id);
