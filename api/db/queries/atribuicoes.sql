-- Queries das atribuições (PROJETO §Atribuição): turma ← atividade com
-- autocomplete/início/prazo/duração/atraso. O dono da turma cria.

-- name: InsertAtribuicao :one
INSERT INTO atribuicoes (turma_id, atividade_id, autocomplete, inicio, prazo, duracao_seg, pode_atrasado, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, turma_id, atividade_id, autocomplete, inicio, prazo, duracao_seg, pode_atrasado, created_at;

-- name: GetAtribuicao :one
SELECT a.id,
       a.turma_id,
       a.atividade_id,
       a.autocomplete,
       a.inicio,
       a.prazo,
       a.duracao_seg,
       a.pode_atrasado,
       a.created_at,
       t.professor_id   AS turma_professor_id,
       t.encerrado_em   AS turma_encerrado_em,
       atv.professor_id AS atividade_professor_id,
       atv.nome         AS atividade_nome,
       t.titulo         AS turma_titulo
FROM atribuicoes a
JOIN turmas t ON t.id = a.turma_id
JOIN atividades atv ON atv.id = a.atividade_id
WHERE a.id = $1;

-- name: ListAtribuicoes :many
SELECT a.id,
       a.turma_id,
       a.atividade_id,
       a.autocomplete,
       a.inicio,
       a.prazo,
       a.duracao_seg,
       a.pode_atrasado,
       a.created_at,
       t.professor_id AS turma_professor_id,
       atv.nome       AS atividade_nome,
       t.titulo       AS turma_titulo
FROM atribuicoes a
JOIN turmas t ON t.id = a.turma_id
JOIN atividades atv ON atv.id = a.atividade_id
WHERE (sqlc.narg('turma')::bigint IS NULL OR a.turma_id = sqlc.narg('turma')::bigint)
  AND (sqlc.narg('atividade')::bigint IS NULL OR a.atividade_id = sqlc.narg('atividade')::bigint)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAtribuicoes :one
SELECT count(*)
FROM atribuicoes a
WHERE (sqlc.narg('turma')::bigint IS NULL OR a.turma_id = sqlc.narg('turma')::bigint)
  AND (sqlc.narg('atividade')::bigint IS NULL OR a.atividade_id = sqlc.narg('atividade')::bigint);

-- name: UpdateAtribuicao :one
UPDATE atribuicoes
SET autocomplete  = $2,
    inicio        = $3,
    prazo         = $4,
    duracao_seg   = $5,
    pode_atrasado = $6
WHERE id = $1
RETURNING id, turma_id, atividade_id, autocomplete, inicio, prazo, duracao_seg, pode_atrasado, created_at;

-- name: DeleteAtribuicao :execrows
DELETE FROM atribuicoes
WHERE id = $1;
