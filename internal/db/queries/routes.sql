-- name: CreateRoute :one
INSERT INTO routes (id, project_id, provider_route_id, public_path, upstream_host, upstream_port, enabled, sync_status, last_error, synced_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetRoute :one
SELECT * FROM routes WHERE id = ?;

-- name: GetRouteByProject :one
SELECT * FROM routes WHERE project_id = ?;

-- name: ListRoutesBySyncStatus :many
SELECT * FROM routes WHERE sync_status = ? ORDER BY updated_at, id;

-- name: MarkRouteSynced :exec
UPDATE routes SET sync_status = 'synced', last_error = NULL, synced_at = ?, updated_at = ? WHERE id = ?;

-- name: MarkRouteFailed :exec
UPDATE routes SET sync_status = 'failed', last_error = ?, updated_at = ? WHERE id = ?;
