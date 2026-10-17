-- Queries das tentativas (PROJETO §Tentativa): rascunho do aluno por
-- (atribuição, aluno), com revisão anti duas-abas.

-- name: GetTentativa :one
SELECT id, atribuicao_id, aluno_id, comecou_em, revisao, gravado_em, snapshot
FROM tentativas
WHERE atribuicao_id = $1 AND aluno_id = $2;

-- name: InsertTentativa :one
INSERT INTO tentativas (atribuicao_id, aluno_id, snapshot, created_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atribuicao_id, aluno_id) DO NOTHING
RETURNING id, atribuicao_id, aluno_id, comecou_em, revisao, gravado_em, snapshot;

-- name: GravarTentativa :many
UPDATE tentativas
SET snapshot   = $3,
    revisao    = revisao + 1,
    gravado_em = now()
WHERE atribuicao_id = $1
  AND aluno_id = $2
  AND revisao = $4
RETURNING id, atribuicao_id, aluno_id, comecou_em, revisao, gravado_em, snapshot;

-- Matrícula ativa do aluno na turma (chave para abrir tentativa).
-- name: MatriculaAtiva :one
SELECT count(*) > 0 AS matriculado
FROM matriculas
WHERE turma_id = $1
  AND aluno_id = $2
  AND saida_em IS NULL;

-- name: RestaurarTentativa :exec
INSERT INTO tentativas (atribuicao_id, aluno_id, comecou_em, revisao, gravado_em, snapshot, created_by)
VALUES ($1, $2, $3, 1, now(), $4, $5)
ON CONFLICT (atribuicao_id, aluno_id) DO UPDATE
SET comecou_em = EXCLUDED.comecou_em,
    revisao = 1,
    gravado_em = now(),
    snapshot = EXCLUDED.snapshot,
    created_by = EXCLUDED.created_by;

-- name: ListTentativasSemSubmissao :many
SELECT t.id,
       t.atribuicao_id,
       t.aluno_id,
       t.comecou_em,
       t.revisao,
       t.gravado_em,
       p.first_name AS aluno_first_name,
       p.last_name  AS aluno_last_name,
       u.email       AS aluno_email
FROM tentativas t
JOIN users u ON u.id = t.aluno_id
JOIN persons p ON p.id = u.person_id
LEFT JOIN submissoes s ON s.atribuicao_id = t.atribuicao_id AND s.aluno_id = t.aluno_id
WHERE t.atribuicao_id = $1
  AND s.id IS NULL
ORDER BY t.comecou_em, t.id;

