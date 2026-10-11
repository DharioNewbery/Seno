-- deploy/seed-dev.sql — dados de teste do fluxo do Monitor em dev.
-- Login: super@seno.dev / 102938 (super, professor da turma)
--        alice@seno.dev / 102938 (aluno da turma)
-- Re-executável: limpa os dados de teste antes de inserir.

BEGIN;

-- Limpeza idempotente (ordem reversa de FK)
DELETE FROM atribuicoes;
DELETE FROM monitor_estado_aluno;
DELETE FROM atividade_tarefas;
DELETE FROM atividades;
DELETE FROM testes_tarefa;
DELETE FROM tarefas;
DELETE FROM matriculas;
DELETE FROM turmas;
DELETE FROM materias;
DELETE FROM periodos_letivos;
DELETE FROM users WHERE email IN ('alice@seno.dev');
DELETE FROM persons WHERE first_name = 'Alice' AND last_name = 'Souza';

-- Matéria, período e turma do super
INSERT INTO materias (codigo, nome) VALUES ('PROG1', 'Programação I');
INSERT INTO periodos_letivos (trimestre, ano) VALUES (4, 2026);

INSERT INTO turmas (materia_id, periodo_id, professor_id, titulo)
SELECT m.id, p.id, u.id, 'T1'
FROM materias m, periodos_letivos p, users u
WHERE m.codigo = 'PROG1' AND (p.trimestre, p.ano) = (4, 2026)
  AND u.email = 'super@seno.dev';

-- Aluna de teste (senha cópia do super: 102938)
INSERT INTO persons (first_name, last_name) VALUES ('Alice', 'Souza');
INSERT INTO users (person_id, email, password_hash, status)
SELECT p.id, 'alice@seno.dev',
       (SELECT password_hash FROM users WHERE email = 'super@seno.dev'),
       'active'
FROM persons p WHERE p.first_name = 'Alice' AND p.last_name = 'Souza';
INSERT INTO user_roles (user_id, role)
SELECT u.id, 'student' FROM users u WHERE u.email = 'alice@seno.dev';

INSERT INTO matriculas (turma_id, aluno_id, created_by)
SELECT t.id, u2.id, (SELECT id FROM users WHERE email = 'super@seno.dev')
FROM turmas t, users u2
WHERE t.titulo = 'T1' AND u2.email = 'alice@seno.dev';

-- Tarefa com 1 teste público + 1 privado
INSERT INTO tarefas (professor_id, nome, enunciado, tempo_cpu_ms, tempo_total_ms, memoria_mb, created_by)
SELECT u.id, 'Dobro', 'Leia um inteiro e imprima o dobro.', 2000, 5000, 256, u.id
FROM users u WHERE u.email = 'super@seno.dev';

INSERT INTO testes_tarefa (tarefa_id, stdin, stdout_esperado, publico, created_by)
SELECT t.id, '3', '6', true, t.professor_id FROM tarefas t WHERE t.nome = 'Dobro';
INSERT INTO testes_tarefa (tarefa_id, stdin, stdout_esperado, publico, created_by)
SELECT t.id, '-4', '-8', false, t.professor_id FROM tarefas t WHERE t.nome = 'Dobro';

-- Atividade com a tarefa
INSERT INTO atividades (professor_id, nome, conteudo, created_by)
SELECT u.id, 'Prova 1 — Monitor', '{}', u.id
FROM users u WHERE u.email = 'super@seno.dev';

INSERT INTO atividade_tarefas (atividade_id, tarefa_id, ordem, valor_pts, linguagem)
SELECT a.id, t.id, 1, 10, 'python'
FROM atividades a, tarefas t
WHERE a.nome = 'Prova 1 — Monitor' AND t.nome = 'Dobro';

-- Atribuição (publicação dispara monitor_transferir na fila)
INSERT INTO atribuicoes (turma_id, atividade_id, autocomplete, inicio, prazo, duracao_seg, pode_atrasado, created_by)
SELECT t.id, a.id, false, now() - interval '5 minutes', now() + interval '4 hours', 7200, true,
       (SELECT id FROM users WHERE email = 'super@seno.dev')
FROM turmas t, atividades a
WHERE t.titulo = 'T1' AND a.nome = 'Prova 1 — Monitor';
-- Direct INSERT na tabela pula o endpoint POST /v1/atribuicoes (que é quem
-- enfileira o job de transferência). Injete o job igual ao handler:
INSERT INTO jobs (kind, payload, priority)
SELECT 'monitor_transferir', jsonb_build_object('atribuicao_id', a.id), 5
FROM atribuicoes a WHERE a.monitor_id IS NULL AND a.id = (SELECT max(id) FROM atribuicoes);

COMMIT;
