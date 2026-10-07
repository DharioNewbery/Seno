-- 0007_tentativas.up.sql — Tentativa (PROJETO §Tentativa): relatório do
-- estado atual da atividade para o aluno (autosave). Única por
-- (aluno, atribuição); revisão incrementada a cada gravação — gravações
-- com revisão antiga são rejeitadas no servidor (duas abas). Excluída
-- quando a Submissão é feita (futuro).

BEGIN;

CREATE TABLE tentativas (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    atribuicao_id BIGINT      NOT NULL REFERENCES atribuicoes (id) ON DELETE CASCADE,
    aluno_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    comecou_em    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revisao       INTEGER     NOT NULL DEFAULT 1 CHECK (revisao >= 1),
    gravado_em    TIMESTAMPTZ NOT NULL DEFAULT now(),
    snapshot      JSONB       NOT NULL,
    created_by    BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    UNIQUE (atribuicao_id, aluno_id),
    CHECK (gravado_em >= comecou_em)
);

CREATE INDEX tentativas_aluno_idx ON tentativas (aluno_id);

COMMIT;
