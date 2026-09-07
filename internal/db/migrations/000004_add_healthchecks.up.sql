CREATE TABLE project_healthchecks (
    id TEXT PRIMARY KEY NOT NULL,
    project_id TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    expected_status INTEGER NOT NULL DEFAULT 200 CHECK (expected_status BETWEEN 200 AND 599),
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);
CREATE INDEX project_healthchecks_project_idx ON project_healthchecks (project_id);
