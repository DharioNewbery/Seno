-- Queries de sessões opacas (SHA-256; sliding window com teto).

-- name: InsertSession :one
INSERT INTO sessions (user_id, token_hash, expires_at, max_expires_at, created_ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at;

-- name: FindSessionByTokenHash :one
SELECT id,
       user_id,
       expires_at,
       max_expires_at
FROM sessions
WHERE token_hash = $1
  AND expires_at > now()
  AND max_expires_at > now();

-- name: UpdateSessionActivity :exec
UPDATE sessions
SET last_seen_at = now(),
    expires_at   = $2
WHERE id = $1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = $1;

-- name: DeleteAllUserSessions :execrows
DELETE FROM sessions
WHERE user_id = $1;
