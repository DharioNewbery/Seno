-- Queries do aluno autenticado (portal aluno): turmas com matrícula
-- ativa e atribuições visíveis (PROJETO §Portal Aluno §hub).

-- name: ListTurmasAluno :many
SELECT t.id,
       t.titulo,
       t.encerrado_em,
       t.created_at,
       m.codigo    AS materia_codigo,
       m.nome      AS materia_nome,
       p.trimestre,
       p.ano,
       up.first_name AS prof_first_name,
       up.last_name  AS prof_last_name,
       uu.email      AS prof_email
FROM matriculas mt
JOIN turmas t ON t.id = mt.turma_id
JOIN materias m ON m.id = t.materia_id
JOIN periodos_letivos p ON p.id = t.periodo_id
JOIN users uu ON uu.id = t.professor_id
JOIN persons up ON up.id = uu.person_id
WHERE mt.aluno_id = $1
  AND mt.saida_em IS NULL
ORDER BY t.encerrado_em IS NULL DESC, t.created_at DESC, t.id DESC;

-- Atribuições da escolha do aluno (turmas com matrícula ativa), com a
-- tentativa própria quando existir (em andamento). Sem submissões não
-- há separação entre pendentes/entregues — o rascunho sinaliza o estado.
-- name: ListAtribuicoesAluno :many
SELECT a.id,
       a.turma_id,
       a.autocomplete,
       a.inicio,
       a.prazo,
       a.duracao_seg,
       a.pode_atrasado,
       m.codigo        AS materia_codigo,
       t.titulo        AS turma_titulo,
       t.encerrado_em  AS turma_encerrado_em,
       atv.id          AS atividade_id,
       atv.nome        AS atividade_nome,
       tt.revisao      AS minha_revisao,
       tt.gravado_em   AS meu_gravado_em,
       ss.entregue_em  AS minha_entrega,
       ss.atrasada     AS minha_atrasada
FROM atribuicoes a
JOIN turmas t ON t.id = a.turma_id
JOIN materias m ON m.id = t.materia_id
JOIN atividades atv ON atv.id = a.atividade_id
  AND atv.deleted_at IS NULL
JOIN matriculas mt ON mt.turma_id = t.id
  AND mt.aluno_id = $1
  AND mt.saida_em IS NULL
LEFT JOIN tentativas tt ON tt.atribuicao_id = a.id
  AND tt.aluno_id = $1
LEFT JOIN submissoes ss ON ss.atribuicao_id = a.id
  AND ss.aluno_id = $1
WHERE (sqlc.narg('turma')::bigint IS NULL OR a.turma_id = sqlc.narg('turma')::bigint)
ORDER BY a.prazo NULLS FIRST, tt.gravado_em NULLS FIRST, a.created_at DESC, a.id DESC;
