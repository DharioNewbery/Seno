-- Queries do banco de tarefas e testes (PROJETO §Tarefas/§Teste).
-- A linguagem não pertence à Tarefa: é definida na Atividade, por tarefa.

-- name: InsertTarefa :one
INSERT INTO tarefas (professor_id, nome, enunciado, tempo_cpu_ms, tempo_total_ms, memoria_mb, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, professor_id, nome, enunciado, tempo_cpu_ms, tempo_total_ms, memoria_mb, created_at, deleted_at;

-- name: InsertTesteTarefa :one
INSERT INTO testes_tarefa (tarefa_id, stdin, stdout_esperado, publico, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, tarefa_id, stdin, stdout_esperado, publico, created_at;

-- name: UpdateTarefa :one
UPDATE tarefas
SET nome           = $2,
    enunciado      = $3,
    tempo_cpu_ms   = $4,
    tempo_total_ms = $5,
    memoria_mb     = $6
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, professor_id, nome, enunciado, tempo_cpu_ms, tempo_total_ms, memoria_mb, created_at, deleted_at;

-- name: ReplaceTestesTarefa :execrows
DELETE FROM testes_tarefa
WHERE tarefa_id = $1;

-- name: DeleteTarefa :execrows
DELETE FROM tarefas
WHERE id = $1
  AND deleted_at IS NULL;

-- name: SoftDeleteTarefa :execrows
UPDATE tarefas
SET deleted_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetTarefa :one
SELECT t.id,
       t.professor_id,
       t.nome,
       t.enunciado,
       t.tempo_cpu_ms,
       t.tempo_total_ms,
       t.memoria_mb,
       t.created_at,
       t.deleted_at,
       p.first_name,
       p.last_name,
       u.email AS professor_email
FROM tarefas t
JOIN users u ON u.id = t.professor_id
JOIN persons p ON p.id = u.person_id
WHERE t.id = $1;

-- name: ListTarefas :many
SELECT t.id,
       t.professor_id,
       t.nome,
       t.tempo_cpu_ms,
       t.tempo_total_ms,
       t.memoria_mb,
       t.deleted_at,
       t.created_at,
       p.first_name,
       p.last_name,
       u.email AS professor_email
FROM tarefas t
JOIN users u ON u.id = t.professor_id
JOIN persons p ON p.id = u.person_id
WHERE (sqlc.narg('professor')::bigint IS NULL OR t.professor_id = sqlc.narg('professor')::bigint)
  AND t.deleted_at IS NULL
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountTarefas :one
SELECT count(*)
FROM tarefas
WHERE (sqlc.narg('professor')::bigint IS NULL OR professor_id = sqlc.narg('professor')::bigint)
  AND deleted_at IS NULL;

-- name: ListTestesTarefa :many
SELECT id, tarefa_id, stdin, stdout_esperado, publico, created_at
FROM testes_tarefa
WHERE tarefa_id = $1
ORDER BY id;

-- name: CopyTarefa :one
INSERT INTO tarefas (professor_id, nome, enunciado, tempo_cpu_ms, tempo_total_ms, memoria_mb, created_by)
SELECT src.professor_id, $2, src.enunciado, src.tempo_cpu_ms, src.tempo_total_ms, src.memoria_mb, $3
FROM tarefas src
WHERE src.id = $1
RETURNING tarefas.id, tarefas.professor_id, tarefas.nome, tarefas.enunciado, tarefas.tempo_cpu_ms, tarefas.tempo_total_ms, tarefas.memoria_mb, tarefas.created_at, tarefas.deleted_at;

-- name: CopyTestesTarefa :many
INSERT INTO testes_tarefa (tarefa_id, stdin, stdout_esperado, publico, created_by)
SELECT $2, src.stdin, src.stdout_esperado, src.publico, $3
FROM testes_tarefa src
WHERE src.tarefa_id = $1
RETURNING testes_tarefa.id, testes_tarefa.tarefa_id, testes_tarefa.stdin, testes_tarefa.stdout_esperado, testes_tarefa.publico, testes_tarefa.created_at;
