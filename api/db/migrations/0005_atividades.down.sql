-- 0005_atividades.down.sql — remove atividades e o vínculo com tarefas.

BEGIN;

DROP TABLE IF EXISTS atividade_tarefas;
DROP TABLE IF EXISTS atividades;

COMMIT;
