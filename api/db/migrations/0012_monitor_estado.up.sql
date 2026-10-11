-- 0012_monitor_estado — estado de exibição do Monitor persistido pela API
-- (Fase 9.3, PLANO_MONITOR): job periódico consome GET /interna/.../estado
-- e grava o snapshot por aluno para alimentar /v1/me/atribuicoes. Só
-- metadados de exibição (a dívida de "publicação" fica no fluxo da API).

BEGIN;

CREATE TABLE monitor_estado_aluno (
    atribuicao_id   BIGINT      NOT NULL REFERENCES atribuicoes (id) ON DELETE CASCADE,
    aluno_id        BIGINT      NOT NULL,
    estado          TEXT        NOT NULL CHECK (estado IN ('pendente', 'aberta', 'entregue')),
    n_t             INTEGER,
    entregue_em     TIMESTAMPTZ,
    nota_auto       NUMERIC,
    correcao_status TEXT,
    prazo_efetivo   TIMESTAMPTZ,
    atualizado_em   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (atribuicao_id, aluno_id)
);

CREATE INDEX monitor_estado_aluno_aluno_idx ON monitor_estado_aluno (aluno_id);

COMMIT;
