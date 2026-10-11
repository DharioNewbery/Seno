-- deploy/limpar-dev.sql — limpa os dados de teste do banco da API (seno).
-- Mantém o super admin (super@seno.dev); tudo o mais some. Re-executável.
-- Uso: Get-Content deploy\limpar-dev.sql -Raw | docker exec -i seno-dev-postgres-1 psql -U seno -d seno -f -
-- (O banco do Monitor: deploy\limpar-dev-monitor.sql)

TRUNCATE monitor_estado_aluno, execucoes_teste, correcao_tarefas, correcoes,
         resultados, submissoes, tentativas, atribuicoes, atividade_tarefas,
         atividades, testes_tarefa, tarefas, matriculas, turmas, materias,
         periodos_letivos, log_entries, jobs, sessions, one_time_tokens
  RESTART IDENTITY CASCADE;

DELETE FROM user_roles
WHERE user_id <> (SELECT id FROM users WHERE email = 'super@seno.dev');
DELETE FROM users WHERE email <> 'super@seno.dev';
DELETE FROM persons p
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.person_id = p.id);

-- Sessões do navegador (BFF) ficam inválidas: logue de novo.
