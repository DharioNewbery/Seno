-- 0003_fila.sql — fila PostgreSQL do Monitor (mesmo desenho de
-- ARQUITETURA §6: worker embutido, FOR UPDATE SKIP LOCKED, backoff)
-- e vínculo execucao -> run (job) para o GET de ensaio.

BEGIN;

CREATE TABLE jobs (
  id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  kind        TEXT        NOT NULL,
  payload     JSONB       NOT NULL DEFAULT '{}'::jsonb,
  status      TEXT        NOT NULL DEFAULT 'agendado'
              CHECK (status IN ('agendado', 'executando', 'concluido', 'falhou')),
  priority    SMALLINT    NOT NULL DEFAULT 0,
  attempts    SMALLINT    NOT NULL DEFAULT 0,
  run_after   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error  TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Dequeue seguro com múltiplos workers (ARQUITETURA §6).
CREATE INDEX jobs_dequeue_idx
  ON jobs (priority DESC, id)
  WHERE status = 'agendado';

-- ALTER TABLE execucao ADD COLUMN job_id BIGINT;
-- CREATE UNIQUE INDEX execucao_job_teste_idx
--   ON execucao (job_id, teste_ordem)
--   WHERE job_id IS NOT NULL;

-- run: token opaco do ensaio (o job em si fica na fila; o vínculo é o run).
ALTER TABLE execucao ADD COLUMN run TEXT;
ALTER TABLE execucao ADD COLUMN token TEXT; -- token do Judge0 (fallback de polling)
CREATE UNIQUE INDEX execucao_run_teste_idx ON execucao (run, teste_ordem);

COMMIT;
