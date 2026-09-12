-- name: RecordActivity :exec
INSERT INTO activity_logs (
    id, actor_intern_id, event_type, entity_type, entity_id, outcome, detail, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListActivity :many
SELECT * FROM activity_logs
WHERE (sqlc.arg(event_type) = '' OR event_type = sqlc.arg(event_type))
  AND (sqlc.arg(category) = '' OR event_type LIKE sqlc.arg(category) || '.%')
  AND (sqlc.arg(outcome) = '' OR outcome = sqlc.arg(outcome))
  AND (sqlc.arg(search) = '' OR entity_id LIKE '%' || sqlc.arg(search) || '%' OR detail LIKE '%' || sqlc.arg(search) || '%')
ORDER BY created_at DESC, id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
