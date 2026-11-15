-- 0009_arquivos.up.sql — Armazenamento de arquivos anexados pelo aluno
-- durante a tentativa e congelados na submissão (tipo "arquivo_submissao").

BEGIN;

CREATE TABLE IF NOT EXISTS tentativa_arquivo (
  monitor_id    TEXT        NOT NULL REFERENCES atividade (monitor_id) ON DELETE CASCADE,
  aluno_api_id  BIGINT      NOT NULL,
  tarefa_ordem  INTEGER     NOT NULL,
  tarefa_api_id BIGINT      NOT NULL DEFAULT 0,
  nome_arquivo  TEXT        NOT NULL,
  mime_type     TEXT        NOT NULL,
  tamanho_bytes BIGINT      NOT NULL CHECK (tamanho_bytes > 0),
  sha256        TEXT        NOT NULL,
  conteudo      BYTEA       NOT NULL,
  atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (monitor_id, aluno_api_id, tarefa_ordem)
);

CREATE TABLE IF NOT EXISTS submissao_arquivo (
  submissao_id  BIGINT      NOT NULL REFERENCES submissao (id) ON DELETE CASCADE,
  tarefa_ordem  INTEGER     NOT NULL,
  tarefa_api_id BIGINT      NOT NULL DEFAULT 0,
  nome_arquivo  TEXT        NOT NULL,
  mime_type     TEXT        NOT NULL,
  tamanho_bytes BIGINT      NOT NULL CHECK (tamanho_bytes > 0),
  sha256        TEXT        NOT NULL,
  conteudo      BYTEA       NOT NULL,
  criado_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (submissao_id, tarefa_ordem)
);

CREATE INDEX IF NOT EXISTS submissao_arquivo_sub_idx ON submissao_arquivo (submissao_id);

COMMIT;
