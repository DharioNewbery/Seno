-- 0004_tarefas.down.sql — remove o banco de tarefas.

BEGIN;

DROP TABLE IF EXISTS testes_tarefa;
DROP TABLE IF EXISTS tarefas;

COMMIT;
