-- 0013_tarefas_tipos_e_arquivos.up.sql — Suporte a múltiplos tipos de tarefa
-- (programacao, arquivo_submissao) e armazenamento de arquivos submetidos.

BEGIN;

ALTER TABLE tarefas
    ADD COLUMN tipo   TEXT  NOT NULL DEFAULT 'programacao'
        CHECK (tipo IN ('programacao', 'arquivo_submissao')),
    ADD COLUMN config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE atividade_tarefas
    ALTER COLUMN linguagem SET DEFAULT '',
    DROP CONSTRAINT IF EXISTS atividade_tarefas_linguagem_check,
    ADD CONSTRAINT atividade_tarefas_linguagem_check
        CHECK (linguagem IN ('', 'python', 'c', 'cpp'));

CREATE TABLE submissao_arquivos (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    submissao_id  BIGINT      NOT NULL REFERENCES submissoes (id) ON DELETE CASCADE,
    tarefa_id     BIGINT      NOT NULL REFERENCES tarefas (id) ON DELETE RESTRICT,
    nome_arquivo  TEXT        NOT NULL CHECK (length(btrim(nome_arquivo)) > 0),
    mime_type     TEXT        NOT NULL CHECK (length(btrim(mime_type)) > 0),
    tamanho_bytes BIGINT      NOT NULL CHECK (tamanho_bytes > 0),
    sha256        TEXT        NOT NULL CHECK (length(btrim(sha256)) > 0),
    conteudo      BYTEA       NOT NULL,
    criado_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submissao_id, tarefa_id)
);

CREATE INDEX submissao_arquivos_sub_idx ON submissao_arquivos (submissao_id);

COMMIT;
