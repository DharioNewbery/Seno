-- 0005_correcao.sql — Resultado da correção automática do Monitor
-- (desvio documentado da spec §4: o estado exposto em §6 pede nota_auto e
-- correcao_status por aluno; este é o lar desses valores).

BEGIN;

CREATE TABLE correcao (
  id            BIGSERIAL PRIMARY KEY,
  submissao_id  BIGINT NOT NULL UNIQUE REFERENCES submissao (id) ON DELETE CASCADE,
  monitor_id    TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id  INTEGER NOT NULL,
  status        TEXT NOT NULL DEFAULT 'pendente'
                CHECK (status IN ('pendente', 'executando', 'concluida', 'falhou')),
  nota_auto     NUMERIC,
  resultados    JSONB,
  criada_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
  concluida_em  TIMESTAMPTZ
);

CREATE INDEX correcao_monitor_idx ON correcao (monitor_id);

COMMIT;
