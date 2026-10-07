-- 0009_correcoes.up.sql — Correção (PROJETO §Correção/§Correção de
-- tarefa/§Resultado). 1:1 com submissão; gerada na entrega; execução do
-- Judge0 assíncrona em lote (resultados chegam por callback). Nota
-- automática por tarefa: value_pts se todos os testes passaram, senão 0;
-- vazia em tarefas abertas. O professor ajusta nota_final/feedback,
-- confirma e publica (ordem garantida por CHECK).

BEGIN;

CREATE TABLE correcoes (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    submissao_id   BIGINT      NOT NULL UNIQUE REFERENCES submissoes (id) ON DELETE CASCADE,
    status         TEXT        NOT NULL DEFAULT 'pendente'
                   CHECK (status IN ('pendente', 'executando', 'concluida', 'falhou')),
    feedback       TEXT,
    confirmada_por BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    confirmada_em  TIMESTAMPTZ,
    publicada_em   TIMESTAMPTZ,
    created_by     BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (publicada_em IS NULL OR (confirmada_em IS NOT NULL AND publicada_em >= confirmada_em))
);

CREATE TABLE correcao_tarefas (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    correcao_id   BIGINT      NOT NULL REFERENCES correcoes (id) ON DELETE CASCADE,
    tarefa_id     BIGINT      NOT NULL REFERENCES tarefas (id) ON DELETE RESTRICT,
    valor_pts     INTEGER     NOT NULL DEFAULT 0, -- o definido na atividade
    nota_auto     INTEGER,    -- automática; NULL em tarefas abertas
    nota_final    INTEGER,    -- o professor ajusta (livre, até parcial)
    feedback      TEXT,
    UNIQUE (correcao_id, tarefa_id)
);

CREATE TABLE resultados (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    correcao_tarefa_id BIGINT      NOT NULL REFERENCES correcao_tarefas (id) ON DELETE CASCADE,
    teste_id           BIGINT      NOT NULL REFERENCES testes_tarefa (id) ON DELETE RESTRICT,
    status             TEXT, -- veredito do Judge0 (id + descrição)
    stdout             TEXT,
    stderr             TEXT,
    compile_output     TEXT,
    tempo              TEXT,    -- segundos (string decimal)
    memoria            TEXT,    -- KB
    cpu                TEXT,    -- segundos (string decimal)
    UNIQUE (correcao_tarefa_id, teste_id)
);

CREATE INDEX resultados_ct_idx ON resultados (correcao_tarefa_id);

COMMIT;
