-- 0003_trimestre4.up.sql — Períodos letivos passam a aceitar trimestres
-- de 1 a 4 (Problema relatado: range anterior 1–3).

BEGIN;

ALTER TABLE periodos_letivos
    DROP CONSTRAINT IF EXISTS periodos_letivos_trimestre_check;

ALTER TABLE periodos_letivos
    ADD CONSTRAINT periodos_letivos_trimestre_check
    CHECK (trimestre BETWEEN 1 AND 4);

COMMIT;
