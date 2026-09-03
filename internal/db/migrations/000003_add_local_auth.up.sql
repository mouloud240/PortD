ALTER TABLE interns ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE sessions (
    id TEXT PRIMARY KEY NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    intern_id TEXT,
    role TEXT NOT NULL CHECK (role IN ('admin', 'intern')),
    csrf_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (intern_id) REFERENCES interns (id) ON DELETE CASCADE,
    CHECK (
        (role = 'admin' AND intern_id IS NULL) OR
        (role = 'intern' AND intern_id IS NOT NULL)
    )
);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
