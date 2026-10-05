-- Queries de tokens de uso único (convite e recuperação de senha, §5.3).

-- name: InsertOneTimeToken :exec
INSERT INTO one_time_tokens (user_id, kind, token_hash, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5);

-- name: InvalidateTokens :execrows
UPDATE one_time_tokens
SET used_at = now()
WHERE user_id = $1
  AND kind = $2
  AND used_at IS NULL;

-- name: ConsumeToken :one
UPDATE one_time_tokens
SET used_at = now()
WHERE token_hash = $1
  AND kind = $2
  AND used_at IS NULL
  AND expires_at > now()
RETURNING user_id;

-- name: FindTokenUser :one
SELECT user_id
FROM one_time_tokens
WHERE token_hash = $1
  AND kind = $2
  AND used_at IS NULL
  AND expires_at > now();
