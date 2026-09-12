-- name: ListProjectHealthchecks :many
SELECT * FROM project_healthchecks WHERE project_id = ? ORDER BY created_at, id;

-- name: ListAllProjectHealthchecks :many
SELECT * FROM project_healthchecks ORDER BY project_id, created_at, id;

-- name: AddProjectHealthcheck :one
INSERT INTO project_healthchecks (id, project_id, endpoint, expected_status, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteProjectHealthcheck :exec
DELETE FROM project_healthchecks WHERE id = ? AND project_id = ?;
