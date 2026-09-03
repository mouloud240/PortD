-- name: CreateIntern :one
INSERT INTO interns (id, full_name, email, identifier, password_hash, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetIntern :one
SELECT * FROM interns WHERE id = ?;

-- name: GetActiveInternByIdentifier :one
SELECT * FROM interns WHERE identifier = ? AND active = 1;

-- name: ListActiveInterns :many
SELECT * FROM interns WHERE active = 1 ORDER BY full_name, id;
