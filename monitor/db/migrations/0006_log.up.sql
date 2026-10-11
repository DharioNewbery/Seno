-- 0006_log.sql — Log de auditoria do Monitor (Fase 8.2; ARQUITETURA §9.3,
-- PROJETO §Log): data-hora, agente, tipo, descrição. Registra escritas e
-- entradas (login); ensaios têm histórico próprio (execucao) e autosave tem
-- rascunho_ponto — nenhum dos dois entra aqui.

BEGIN;

CREATE TABLE log (
  id          BIGSERIAL PRIMARY KEY,
  ocorrida_em TIMESTAMPTZ NOT NULL DEFAULT now(),
  agente      TEXT NOT NULL,   -- ex.: "api", "aluno:12", "ip:203.0.113.9"
  tipo        TEXT NOT NULL,   -- ex.: transferencia, entrada, abrir, cancelar, confirmar
  descricao   TEXT NOT NULL
);

CREATE INDEX log_ocorrida_em_idx ON log (ocorrida_em);

COMMIT;
