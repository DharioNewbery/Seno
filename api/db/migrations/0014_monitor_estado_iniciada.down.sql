BEGIN;

ALTER TABLE monitor_estado_aluno
    DROP COLUMN IF EXISTS iniciada_em;

COMMIT;
