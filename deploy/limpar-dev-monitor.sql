-- deploy/limpar-dev-monitor.sql — limpa o banco do Monitor (seno_monitor).
-- Uso: Get-Content deploy\limpar-dev-monitor.sql -Raw | docker exec -i seno-dev-postgres-1 psql -U seno -d seno_monitor -f -
-- (Atividade leva tentativa/rascunho/execução/submissão/sessão juntos:
--  todas com ON DELETE CASCADE.)

TRUNCATE atividade RESTART IDENTITY CASCADE;
TRUNCATE log RESTART IDENTITY;
TRUNCATE jobs RESTART IDENTITY;
