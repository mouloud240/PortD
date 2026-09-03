CREATE TABLE routes (
    id TEXT PRIMARY KEY NOT NULL,
    project_id TEXT NOT NULL UNIQUE,
    provider_route_id TEXT NOT NULL UNIQUE,
    public_path TEXT NOT NULL UNIQUE,
    upstream_host TEXT NOT NULL DEFAULT 'localhost',
    upstream_port INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    sync_status TEXT NOT NULL DEFAULT 'pending' CHECK (sync_status IN ('pending', 'synced', 'failed', 'disabled')),
    last_error TEXT,
    synced_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    FOREIGN KEY (upstream_port) REFERENCES ports (port) ON DELETE RESTRICT
);

CREATE INDEX routes_sync_status_idx ON routes (sync_status);
