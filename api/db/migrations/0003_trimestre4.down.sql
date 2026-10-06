-- 0003_trimestre4.down.sql — volta ao range original (1–3).

BEGIN;

ALTER TABLE periodos_letivos
    DROP CONSTRAINT IF EXISTS periodos_letivos_trimestre_check;

ALTER TABLE periodos_letivos
    ADD CONSTRAINT periodos_letivos_trimestre_check
    CHECK (trimestre BETWEEN 1 AND 3);

COMMIT;
