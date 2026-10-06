-- 0009_correcoes.down.sql — remove correções e resultados.

BEGIN;

DROP TABLE IF EXISTS resultados;
DROP TABLE IF EXISTS correcao_tarefas;
DROP TABLE IF EXISTS correcoes;

COMMIT;
