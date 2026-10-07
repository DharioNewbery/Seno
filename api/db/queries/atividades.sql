-- Queries das atividades (PROJETO §Atividades): conjunto de tarefas com
-- ordem, pontuação e linguagem por tarefa; conteúdo JSON pré-hidratação.

-- name: InsertAtividade :one
INSERT INTO atividades (professor_id, nome, conteudo, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id, professor_id, nome, conteudo, created_at, deleted_at;

-- name: InsertAtividadeTarefa :exec
INSERT INTO atividade_tarefas (atividade_id, tarefa_id, ordem, valor_pts, linguagem)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteAtividadeTarefas :execrows
DELETE FROM atividade_tarefas
WHERE atividade_id = $1;

-- name: GetAtividade :one
SELECT a.id,
       a.professor_id,
       a.nome,
       a.conteudo,
       a.created_at,
       a.deleted_at,
       p.first_name,
       p.last_name,
       u.email AS professor_email
FROM atividades a
JOIN users u ON u.id = a.professor_id
JOIN persons p ON p.id = u.person_id
WHERE a.id = $1;

-- name: ListAtividadeTarefas :many
SELECT at.tarefa_id,
       at.ordem,
       at.valor_pts,
       at.linguagem,
       t.nome           AS tarefa_nome,
       t.enunciado      AS tarefa_enunciado,
       t.deleted_at     AS tarefa_deleted_at
FROM atividade_tarefas at
JOIN tarefas t ON t.id = at.tarefa_id
WHERE at.atividade_id = $1
ORDER BY at.ordem, at.tarefa_id;

-- name: ListAtividades :many
SELECT a.id,
       a.professor_id,
       a.nome,
       a.deleted_at,
       a.created_at,
       p.first_name,
       p.last_name,
       u.email AS professor_email
FROM atividades a
JOIN users u ON u.id = a.professor_id
JOIN persons p ON p.id = u.person_id
WHERE (sqlc.narg('professor')::bigint IS NULL OR a.professor_id = sqlc.narg('professor')::bigint)
  AND a.deleted_at IS NULL
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAtividades :one
SELECT count(*)
FROM atividades a
WHERE (sqlc.narg('professor')::bigint IS NULL OR a.professor_id = sqlc.narg('professor')::bigint)
  AND a.deleted_at IS NULL;

-- name: UpdateAtividade :one
UPDATE atividades
SET nome = $2,
    conteudo = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, professor_id, nome, conteudo, created_at, deleted_at;

-- name: DeleteAtividade :execrows
DELETE FROM atividades
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CopyAtividade :one
INSERT INTO atividades (professor_id, nome, conteudo, created_by)
SELECT src.professor_id, $2, src.conteudo, $3
FROM atividades src
WHERE src.id = $1
RETURNING id, professor_id, nome, conteudo, created_at, deleted_at;

-- name: CopyAtividadeTarefas :many
INSERT INTO atividade_tarefas (atividade_id, tarefa_id, ordem, valor_pts, linguagem)
SELECT $2, src.tarefa_id, src.ordem, src.valor_pts, src.linguagem
FROM atividade_tarefas src
WHERE src.atividade_id = $1
RETURNING atividade_tarefas.atividade_id, atividade_tarefas.tarefa_id, atividade_tarefas.ordem, atividade_tarefas.valor_pts, atividade_tarefas.linguagem;

-- Tarefa em uso por qualquer atividade não apagada (regra §Ciclo de
-- Tarefa e Atividade): usada → não edita; exclusão vira soft delete.
-- name: CountAtividadesUsandoTarefa :one
SELECT count(*)
FROM atividade_tarefas at
JOIN atividades a ON a.id = at.atividade_id
WHERE at.tarefa_id = $1
  AND a.deleted_at IS NULL;
