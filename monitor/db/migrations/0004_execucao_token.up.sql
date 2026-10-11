-- 0004_execucao_token.sql — token do Judge0 por ensaio (fallback de polling
-- quando o callback é descartado — ver deploy/judge0/reference/PROBES.md).
--
-- HISTÓRICO: em bancos vivos já catalogados, a 0003 original não criava
-- `token`; a edição posterior dela (com fusão do run) passou a criar. Assim,
-- em BANCO NOVO a 0004 sempre colidia (42701). Idempotente para cobrir os
-- dois mundos.

BEGIN;

ALTER TABLE execucao ADD COLUMN IF NOT EXISTS token TEXT;

COMMIT;
