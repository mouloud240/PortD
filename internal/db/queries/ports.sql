-- name: ListAssignedPorts :many
SELECT port FROM ports ORDER BY port;

-- name: ListProjectPorts :many
SELECT * FROM ports WHERE project_id = ? ORDER BY port;

-- name: GetProjectMainPort :one
SELECT * FROM ports WHERE project_id = ? AND role = 'main';

-- name: InsertPort :one
INSERT INTO ports (port, project_id, role, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: DeletePort :exec
DELETE FROM ports WHERE port = ? AND project_id = ?;

-- name: SetPortRole :one
UPDATE ports SET role = ? WHERE port = ? AND project_id = ?
RETURNING *;
