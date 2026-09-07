-- name: UpsertPortObservation :one
INSERT INTO port_observations (id, port, process_name, process_id, project_id, observed_at)
VALUES ('port-' || CAST(? AS TEXT), ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    process_name = excluded.process_name,
    process_id = excluded.process_id,
    project_id = excluded.project_id,
    observed_at = excluded.observed_at
RETURNING *;

-- name: ListPortObservations :many
SELECT * FROM port_observations ORDER BY port;

-- name: DeleteStalePortObservations :exec
DELETE FROM port_observations WHERE observed_at < ?;

-- name: GetPortProjectID :one
SELECT project_id FROM ports WHERE port = ?;
