-- name: CreateIntern :one
INSERT INTO interns (id, full_name, email, identifier, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetIntern :one
SELECT * FROM interns WHERE id = ?;

-- name: ListActiveInterns :many
SELECT * FROM interns WHERE active = 1 ORDER BY full_name, id;
