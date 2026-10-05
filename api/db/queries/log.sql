-- Queries do log de auditoria (eventos de escrita e login).

-- name: InsertLogEntry :exec
INSERT INTO log_entries (actor_id, actor, kind, detail)
VALUES ($1, $2, $3, $4);
