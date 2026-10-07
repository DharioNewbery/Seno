-- Queries das submissões (PROJETO §Submissão): entrega final por
-- (atribuição, aluno); a Tentativa é apagada na submissão.

-- name: InsertSubmissao :one
INSERT INTO submissoes (atribuicao_id, aluno_id, comecou_em, atrasada, observacao, snapshot, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, atribuicao_id, aluno_id, comecou_em, entregue_em, atrasada, snapshot;

-- name: ListSubmissoesAtribuicao :many
SELECT s.id,
       s.atribuicao_id,
       s.aluno_id,
       s.comecou_em,
       s.entregue_em,
       s.atrasada,
       p.first_name   AS aluno_first_name,
       p.last_name    AS aluno_last_name,
       u.email        AS aluno_email
FROM submissoes s
JOIN users u ON u.id = s.aluno_id
JOIN persons p ON p.id = u.person_id
WHERE s.atribuicao_id = $1
ORDER BY s.entregue_em, s.id;

-- Já entregou? A Tentativa não pode voltar após a submissão.
-- name: GetSubmissaoAluno :one
SELECT id, atribuicao_id, aluno_id, comecou_em, entregue_em, atrasada, snapshot
FROM submissoes
WHERE atribuicao_id = $1
  AND aluno_id = $2;

-- Apaga a Tentativa ao submeter (PROJETO §Tentativa: "é excluída assim
-- que uma submissão é feita").
-- name: ApagarTentativa :execrows
DELETE FROM tentativas
WHERE atribuicao_id = $1
  AND aluno_id = $2;
