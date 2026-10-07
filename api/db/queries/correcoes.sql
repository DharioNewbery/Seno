-- Queries da correção (PROJETO §Correção/§Correção de tarefa/§Resultado)
-- e apoio para o lote do Judge0 (callback por submissão do lote).

-- name: InsertCorrecao :one
INSERT INTO correcoes (submissao_id, status, created_by)
VALUES ($1, $2, $3)
RETURNING id, submissao_id, status, confirmada_em, publicada_em, created_at;

-- name: GetCorrecaoPorSubmissao :one
SELECT c.id, c.submissao_id, c.status, c.feedback,
       c.confirmada_em, c.publicada_em, c.created_at
FROM correcoes c
WHERE c.submissao_id = $1;

-- name: ListCorrecaoTarefas :many
SELECT ct.id, ct.correcao_id, ct.tarefa_id, ct.valor_pts,
       ct.nota_auto, ct.nota_final, ct.feedback,
       t.nome AS tarefa_nome
FROM correcao_tarefas ct
JOIN tarefas t ON t.id = ct.tarefa_id
WHERE ct.correcao_id = $1
ORDER BY ct.id;

-- name: InsertCorrecaoTarefa :exec
INSERT INTO correcao_tarefas (correcao_id, tarefa_id, valor_pts)
VALUES ($1, $2, $3);

-- name: AjustarCorrecaoTarefa :exec
UPDATE correcao_tarefas
SET nota_final = $2,
    feedback   = $3
WHERE id = $1;

-- name: UpdateStatusCorrecao :exec
UPDATE correcoes
SET status = $2
WHERE id = $1;

-- name: UpdateFeedbackCorrecao :exec
UPDATE correcoes
SET feedback = $2
WHERE id = $1;

-- name: ConfirmarCorrecao :execrows
UPDATE correcoes
SET confirmada_por = $2,
    confirmada_em  = now()
WHERE id = $1
  AND confirmada_em IS NULL;

-- name: PublicarCorrecao :execrows
UPDATE correcoes
SET publicada_em = now()
WHERE id = $1
  AND confirmada_em IS NOT NULL
  AND publicada_em IS NULL;

-- name: AplicarResultado :exec
INSERT INTO resultados (correcao_tarefa_id, teste_id, status, stdout, stderr, compile_output, tempo, memoria, cpu)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (correcao_tarefa_id, teste_id) DO UPDATE
SET status         = excluded.status,
    stdout         = excluded.stdout,
    stderr         = excluded.stderr,
    compile_output = excluded.compile_output,
    tempo          = excluded.tempo,
    memoria        = excluded.memoria,
    cpu            = excluded.cpu;

-- name: ListResultados :many
SELECT r.id, r.correcao_tarefa_id, r.teste_id, r.status, r.stdout, r.stderr,
       r.compile_output, r.tempo, r.memoria, r.cpu,
       te.stdin, te.stdout_esperado, te.publico
FROM resultados r
JOIN testes_tarefa te ON te.id = r.teste_id
WHERE r.correcao_tarefa_id = ANY($1::bigint[])
ORDER BY r.correcao_tarefa_id, r.teste_id;

-- name: ContarResultadosFaltando :one
SELECT count(*)::int AS faltando
FROM correcao_tarefas ct
WHERE ct.correcao_id = $1
  AND (SELECT count(*) FROM testes_tarefa te WHERE te.tarefa_id = ct.tarefa_id) >
      (SELECT count(*) FROM resultados rs WHERE rs.correcao_tarefa_id = ct.id);

-- Recompute da nota automática da tarefa (value_pts se todos os testes
-- passaram; 0 quando reprovação; NULL em tarefas abertas — sem testes).
-- name: NotaAutoCorrecaoTarefa :one
SELECT ct.id, ct.valor_pts,
       (SELECT count(*) FROM testes_tarefa te WHERE te.tarefa_id = ct.tarefa_id) AS num_testes,
       (SELECT count(*) FROM resultados rs
        WHERE rs.correcao_tarefa_id = ct.id
          AND rs.status IS NOT NULL) AS aplicados,
       (SELECT count(*) FROM resultados rs
        WHERE rs.correcao_tarefa_id = ct.id
          AND rs.status <> 'accepted') AS reprovados
FROM correcao_tarefas ct
WHERE ct.id = $1;

-- name: SetNotaAuto :exec
UPDATE correcao_tarefas
SET nota_auto = $2
WHERE id = $1;

-- Apoio do fluxo de correção: dados por submissão para montar o batch
-- do Judge0 e mapeamento de tarefas/linguagens da atividade.

-- name: GetSubmissaoCorrecao :one
SELECT id, atribuicao_id, aluno_id, snapshot
FROM submissoes
WHERE id = $1;

-- name: GetSubmissaoEscopo :one
SELECT s.id,
       s.atribuicao_id,
       s.aluno_id,
       t.professor_id AS turma_professor_id,
       atv.nome AS atividade_nome,
       u.email AS aluno_email,
       p.first_name AS aluno_first_name,
       p.last_name  AS aluno_last_name
FROM submissoes s
JOIN atribuicoes a ON a.id = s.atribuicao_id
JOIN turmas t ON t.id = a.turma_id
JOIN atividades atv ON atv.id = a.atividade_id
JOIN users u ON u.id = s.aluno_id
JOIN persons p ON p.id = u.person_id
WHERE s.id = $1;

-- name: TarefasDaAtribuicao :many
SELECT at.tarefa_id, at.linguagem, at.valor_pts, at.ordem
FROM atividade_tarefas at
JOIN atribuicoes a ON a.atividade_id = at.atividade_id
WHERE a.id = $1
ORDER BY at.ordem, at.tarefa_id;

-- name: GetCorrecaoTarefa :one
SELECT id, correcao_id, tarefa_id, valor_pts
FROM correcao_tarefas
WHERE id = $1;
