BEGIN;

ALTER TABLE monitor_estado_aluno
    ADD COLUMN IF NOT EXISTS iniciada_em TIMESTAMPTZ;

COMMIT;
