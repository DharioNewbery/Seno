BEGIN;

ALTER TABLE atribuicoes
  DROP CONSTRAINT atribuicoes_monitor_id_key;

ALTER TABLE atribuicoes DROP COLUMN monitor_id;

COMMIT;
