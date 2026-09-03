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

-- name: ListInterns :many
SELECT * FROM interns WHERE (? = '' OR full_name LIKE '%' || ? || '%' OR email LIKE '%' || ? || '%' OR identifier LIKE '%' || ? || '%') AND (? < 0 OR active = ?) ORDER BY full_name, id;

-- name: UpdateIntern :one
UPDATE interns SET full_name = ?, email = ?, identifier = ?, active = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: UpdateInternPassword :exec
UPDATE interns SET password_hash = ?, updated_at = ? WHERE id = ?;

-- name: DeactivateIntern :exec
UPDATE interns SET active = 0, updated_at = ? WHERE id = ?;

-- name: DeleteIntern :exec
DELETE FROM interns WHERE id = ?;

-- name: CountInternAssignments :one
SELECT COUNT(*) FROM project_interns WHERE intern_id = ?;
