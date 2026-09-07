-- name: CreateProject :one
INSERT INTO projects (
    id, name, slug, description, directory, startup_command,
    should_run, is_live, lifecycle_status, route_sync_status, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProjectByID :one
SELECT * FROM projects WHERE id = ?;

-- name: GetProjectBySlug :one
SELECT * FROM projects WHERE slug = ?;

-- name: ListProjects :many
SELECT * FROM projects
WHERE (? = '' OR name LIKE '%' || ? || '%' OR slug LIKE '%' || ? || '%' OR description LIKE '%' || ? || '%')
  AND (? = '' OR lifecycle_status = ?)
  AND (? < 0 OR is_live = ?)
ORDER BY updated_at DESC, name, id;

-- name: UpdateProject :one
UPDATE projects
SET name = ?,
    description = ?,
    should_run = ?,
    lifecycle_status = ?,
    updated_at = ?
WHERE id = ?
RETURNING *;

-- name: InsertProjectIntern :exec
INSERT INTO project_interns (project_id, intern_id, created_at)
VALUES (?, ?, ?);

-- name: DeleteProjectInterns :exec
DELETE FROM project_interns WHERE project_id = ?;

-- name: ListProjectInternIDs :many
SELECT intern_id FROM project_interns WHERE project_id = ? ORDER BY intern_id;

-- name: ListProjectInterns :many
SELECT i.*
FROM interns i
INNER JOIN project_interns pi ON pi.intern_id = i.id
WHERE pi.project_id = ?
ORDER BY i.full_name, i.id;

-- name: IsProjectMember :one
SELECT COUNT(*) FROM project_interns WHERE project_id = ? AND intern_id = ?;

-- name: ListInternProjects :many
SELECT p.*
FROM projects p
INNER JOIN project_interns pi ON pi.project_id = p.id
WHERE pi.intern_id = ?
  AND (? = '' OR p.name LIKE '%' || ? || '%' OR p.slug LIKE '%' || ? || '%' OR p.description LIKE '%' || ? || '%')
  AND (? = '' OR p.lifecycle_status = ?)
  AND (? < 0 OR p.is_live = ?)
ORDER BY p.updated_at DESC, p.name, p.id;
