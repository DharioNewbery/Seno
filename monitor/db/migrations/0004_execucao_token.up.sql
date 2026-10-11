-- 0004_execucao_token.sql — token do Judge0 por ensaio (fallback de polling
-- quando o callback é descartado — ver deploy/judge0/reference/PROBES.md).

BEGIN;

ALTER TABLE execucao ADD COLUMN token TEXT;

COMMIT;
