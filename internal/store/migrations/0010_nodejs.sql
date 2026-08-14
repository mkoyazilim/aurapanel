CREATE TABLE node_apps (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    app_name TEXT NOT NULL,
    app_path TEXT NOT NULL DEFAULT '/',
    startup_script TEXT NOT NULL DEFAULT 'npm start',
    port INTEGER NOT NULL,
    node_version TEXT NOT NULL DEFAULT '20',
    env_vars TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'stopped',
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_node_apps_site_port ON node_apps(site_id, port);
