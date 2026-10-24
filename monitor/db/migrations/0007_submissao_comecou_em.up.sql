-- 0007_submissao_comecou_em.up.sql — Registra o instante de abertura da tentativa na submissão.
BEGIN;

ALTER TABLE submissao ADD COLUMN IF NOT EXISTS comecou_em TIMESTAMPTZ;
UPDATE submissao SET comecou_em = entregue_em WHERE comecou_em IS NULL;

COMMIT;
