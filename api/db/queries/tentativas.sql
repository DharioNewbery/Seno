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
