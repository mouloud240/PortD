PRAGMA foreign_keys = ON;

CREATE TABLE interns (
    id TEXT PRIMARY KEY NOT NULL,
    full_name TEXT NOT NULL,
    email TEXT UNIQUE,
    identifier TEXT UNIQUE,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX interns_active_idx ON interns (active);

CREATE TABLE projects (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    directory TEXT NOT NULL,
    startup_command TEXT NOT NULL,
    should_run INTEGER NOT NULL DEFAULT 0 CHECK (should_run IN (0, 1)),
    is_live INTEGER NOT NULL DEFAULT 0 CHECK (is_live IN (0, 1)),
    lifecycle_status TEXT NOT NULL DEFAULT 'draft' CHECK (lifecycle_status IN ('draft', 'ready', 'running', 'stopped', 'failed', 'archived')),
    route_sync_status TEXT NOT NULL DEFAULT 'pending' CHECK (route_sync_status IN ('pending', 'synced', 'failed', 'disabled')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX projects_lifecycle_idx ON projects (lifecycle_status);
CREATE INDEX projects_should_run_idx ON projects (should_run, is_live);

CREATE TABLE project_interns (
    project_id TEXT NOT NULL,
    intern_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (project_id, intern_id),
    FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    FOREIGN KEY (intern_id) REFERENCES interns (id) ON DELETE RESTRICT
);
CREATE INDEX project_interns_intern_idx ON project_interns (intern_id);

CREATE TABLE ports (
    port INTEGER PRIMARY KEY NOT NULL CHECK (port BETWEEN 1 AND 65535),
    project_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('main', 'internal')),
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);
CREATE INDEX ports_project_idx ON ports (project_id);
CREATE UNIQUE INDEX ports_one_main_per_project_idx ON ports (project_id) WHERE role = 'main';

CREATE TABLE port_observations (
    id TEXT PRIMARY KEY NOT NULL,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    process_name TEXT,
    process_id INTEGER,
    project_id TEXT,
    observed_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE SET NULL
);
CREATE INDEX port_observations_port_idx ON port_observations (port, observed_at);
CREATE INDEX port_observations_project_idx ON port_observations (project_id, observed_at);

CREATE TABLE activity_logs (
    id TEXT PRIMARY KEY NOT NULL,
    actor_intern_id TEXT,
    event_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT,
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure')),
    detail TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY (actor_intern_id) REFERENCES interns (id) ON DELETE SET NULL
);
CREATE INDEX activity_logs_created_idx ON activity_logs (created_at DESC);
CREATE INDEX activity_logs_entity_idx ON activity_logs (entity_type, entity_id, created_at DESC);
