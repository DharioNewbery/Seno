-- 0001_init.sql — Esquema do banco do Monitor (spec §4, MONITOR_FLUXO_ALUNO.md).
-- O Monitor não acessa o Postgres da instituição: tudo abaixo vive no banco
-- dele. `carga` e `dados` carregam o envelope/respostas conforme a spec.

BEGIN;

CREATE TABLE atividade (
  monitor_id   TEXT PRIMARY KEY,
  api_id       INTEGER UNIQUE,
  carga        JSONB NOT NULL,      -- envelope de transferência (integral)
  recebida_em  TIMESTAMPTZ NOT NULL,
  status       TEXT NOT NULL DEFAULT 'ativa'
               CHECK (status IN ('ativa', 'encerrada', 'cancelada'))
);

CREATE INDEX atividade_status_idx ON atividade (status);

CREATE TABLE atividade_aluno (
  monitor_id    TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id  INTEGER NOT NULL,
  UNIQUE (monitor_id, aluno_api_id)
);

CREATE INDEX atividade_aluno_aluno_idx ON atividade_aluno (aluno_api_id);

CREATE TABLE sessao (
  token_hash    TEXT PRIMARY KEY,        -- SHA-256 (base64url) do token `hm_sess`
  aluno_api_id  INTEGER NOT NULL,
  monitor_id    TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  expira_em     TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessao_monitor_idx ON sessao (monitor_id);

CREATE TABLE tentativa (
  monitor_id     TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id   INTEGER NOT NULL,
  UNIQUE (monitor_id, aluno_api_id),
  aberta_em      TIMESTAMPTZ NOT NULL,
  fim_real       TIMESTAMPTZ NOT NULL,
  revisao        INTEGER NOT NULL,
  seq_submissao  INTEGER NOT NULL DEFAULT 0,
  dados          JSONB NOT NULL
);

CREATE INDEX tentativa_monitor_idx ON tentativa (monitor_id);

-- Ponto de snapshot imutável (autosave a cada SENO_SNAPSHOT_INTERVALO_S).
CREATE TABLE rascunho_ponto (
  tentativa_seq  BIGSERIAL PRIMARY KEY,
  monitor_id     TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id   INTEGER NOT NULL,
  fixado_em      TIMESTAMPTZ NOT NULL,
  dados          JSONB NOT NULL
);

CREATE INDEX rascunho_ponto_aluno_idx
  ON rascunho_ponto (monitor_id, aluno_api_id, fixado_em DESC);

CREATE TABLE execucao (
  id            BIGSERIAL PRIMARY KEY,
  monitor_id    TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id  INTEGER NOT NULL,
  tipo          TEXT NOT NULL CHECK (tipo IN ('ensaio', 'pre_avaliacao')),
  tarefa_ordem  INTEGER NOT NULL,
  teste_ordem   INTEGER,
  publico       BOOLEAN,
  veredito      TEXT,
  stdout_obtido TEXT,
  tempo_seg     NUMERIC,
  memoria_kb    INTEGER,
  criada_em     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX execucao_monitor_aluno_idx
  ON execucao (monitor_id, aluno_api_id, criada_em DESC);

CREATE TABLE submissao (
  id            BIGSERIAL PRIMARY KEY,
  monitor_id    TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id  INTEGER NOT NULL,
  n_t           INTEGER NOT NULL,   -- número da submissão: 1, 2, 3...
  UNIQUE (monitor_id, aluno_api_id, n_t),
  dados         JSONB NOT NULL,     -- respostas congeladas (snapshot fechado)
  entregue_em   TIMESTAMPTZ NOT NULL,
  atrasada      BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX submissao_monitor_idx ON submissao (monitor_id);

CREATE TABLE pre_avaliacao (
  id           TEXT PRIMARY KEY,
  submissao_id BIGINT REFERENCES submissao (id),
  monitor_id   TEXT NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id INTEGER NOT NULL,
  revisao_ref  INTEGER NOT NULL,    -- revisão da tentativa no momento da avaliação
  relatorio    JSONB,
  criada_em    TIMESTAMPTZ NOT NULL DEFAULT now(),
  expira_em    TIMESTAMPTZ NOT NULL,
  confirmada   BOOLEAN NOT NULL DEFAULT false,
  cancelada    BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX pre_avaliacao_monitor_aluno_idx
  ON pre_avaliacao (monitor_id, aluno_api_id, expira_em DESC);

COMMIT;
