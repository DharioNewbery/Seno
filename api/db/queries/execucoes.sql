-- Queries do Execução de teste (PROJETO §Execução de teste: retenção
-- de 7 dias, limite de 5 execuções/minuto por aluno — somadas).

-- name: ContarExecucoesRecentes :one
SELECT count(*)::int AS n
FROM execucoes_teste
WHERE aluno_id = $1
  AND criado_em > now() - interval '1 minute';

-- name: MaisAntiga1Minuto :one
SELECT criado_em
FROM execucoes_teste
WHERE aluno_id = $1
  AND criado_em > now() - interval '1 minute'
ORDER BY criado_em
LIMIT 1;

-- name: InsertExecucao :one
INSERT INTO execucoes_teste (aluno_id, atribuicao_id, tarefa_id, linguagem, codigo, resultados)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, aluno_id, atribuicao_id, tarefa_id, linguagem, criado_em;

-- name: ApagarExecucoesVencidas :execrows
DELETE FROM execucoes_teste
WHERE criado_em < now() - interval '7 days';

-- name: ListExecucoesAluno :many
SELECT e.id, e.atribuicao_id, e.tarefa_id, e.linguagem, e.criado_em,
       a.atividade_id,
       (
         SELECT atv.nome FROM atribuicoes aa
         JOIN atividades atv ON atv.id = aa.atividade_id
         WHERE aa.id = e.atribuicao_id
       ) AS atividade_nome
FROM execucoes_teste e
JOIN atribuicoes a ON a.id = e.atribuicao_id
WHERE e.aluno_id = $1
  AND e.criado_em > now() - interval '7 days'
ORDER BY e.criado_em DESC
LIMIT 50;

-- name: GetExecucaoAluno :one
SELECT id, aluno_id, atribuicao_id, tarefa_id, linguagem, codigo, resultados, criado_em
FROM execucoes_teste
WHERE id = $1;

-- name: CountTestesPublicos :one
SELECT count(*)::int > 0 AS tem_publico
FROM testes_tarefa
WHERE tarefa_id = $1
  AND publico;

-- name: CountTestesPublicosDaAtribuicao :many
SELECT at.tarefa_id,
       EXISTS (
         SELECT 1 FROM testes_tarefa te
         WHERE te.tarefa_id = at.tarefa_id AND te.publico
       ) AS tem_publico
FROM atividade_tarefas at
JOIN atribuicoes a ON a.atividade_id = at.atividade_id
WHERE a.id = $1;
