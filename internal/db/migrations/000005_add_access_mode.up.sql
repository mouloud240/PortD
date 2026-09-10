ALTER TABLE projects ADD COLUMN access_mode TEXT NOT NULL DEFAULT 'direct'
    CHECK (access_mode IN ('direct', 'proxied'));
