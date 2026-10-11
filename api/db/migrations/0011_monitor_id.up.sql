-- 0011_monitor_id — mapa API↔Monitor (Fase 9 do PLANO_MONITOR): a API
-- guarda o monitor_id devolvido pela transferência da atribuição publicada.
-- NULL = ainda não transferida (ou integração desligada).

BEGIN;

ALTER TABLE atribuicoes ADD COLUMN monitor_id TEXT;

ALTER TABLE atribuicoes
  ADD CONSTRAINT atribuicoes_monitor_id_key UNIQUE (monitor_id);

COMMIT;
