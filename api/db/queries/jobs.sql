-- Queries da fila de jobs no PostgreSQL (ARQUITETURA §6). O worker embutido
-- entra quando o Judge0 for integrado; a fila já nasce funcional.

-- name: InsertJob :one
INSERT INTO jobs (kind, payload, priority, run_after)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: DequeueJob :one
UPDATE jobs
SET status = 'running', updated_at = now()
WHERE id = (
    SELECT id
    FROM jobs
    WHERE status = 'scheduled'
      AND run_after <= now()
    ORDER BY priority DESC, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, kind, payload, attempts;

-- name: MarkJobDone :exec
UPDATE jobs
SET status = 'done', updated_at = now()
WHERE id = $1;

-- name: ScheduleJobRetry :exec
UPDATE jobs
SET attempts    = attempts + 1,
    status      = 'scheduled',
    run_after   = $2,
    last_error  = $3,
    updated_at  = now()
WHERE id = $1;

-- name: MarkJobFailed :exec
UPDATE jobs
SET status = 'failed', last_error = $2, updated_at = now()
WHERE id = $1;

-- name: CountScheduledJobs :one
SELECT count(*)
FROM jobs
WHERE status = 'scheduled';
