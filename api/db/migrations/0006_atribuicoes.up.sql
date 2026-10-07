-- 0006_atribuicoes.up.sql — Atribuições (PROJETO §Atribuição): conecta
-- uma Turma a uma Atividade. Config: autocomplete, início, prazo, duração
-- e entrega atrasada; os padrões quando ausentes seguem §Atribuição e o
-- prazo efetivo é calculado no servidor (min(início+duração, prazo)).
-- Edição é bloqueada a partir da primeira Submissão (futuro).

BEGIN;

CREATE TABLE atribuicoes (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    turma_id       BIGINT      NOT NULL REFERENCES turmas (id) ON DELETE CASCADE,
    atividade_id   BIGINT      NOT NULL REFERENCES atividades (id) ON DELETE RESTRICT,
    autocomplete   BOOLEAN     NOT NULL DEFAULT false,
    inicio         TIMESTAMPTZ, -- ausente: disponível imediatamente
    prazo          TIMESTAMPTZ, -- ausente: aberta até encerrar a turma
    duracao_seg    INTEGER,    -- ausente: sem limite além do prazo
    pode_atrasado  BOOLEAN     NOT NULL DEFAULT false,
    created_by     BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (turma_id, atividade_id),
    CHECK (duracao_seg IS NULL OR duracao_seg > 0)
);

CREATE INDEX atribuicoes_turma_idx ON atribuicoes (turma_id);
CREATE INDEX atribuicoes_atividade_idx ON atribuicoes (atividade_id);

COMMIT;
