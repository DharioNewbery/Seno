-- 0008_submissoes.up.sql — Submissão (PROJETO §Submissão/§Atribuição):
-- entrega final por aluno/atribuição, com o snapshot das respostas e
-- `comecou_em` original da Tentativa (que é apagada em seguida). Única
-- por (aluno, atribuição); excluí-la pelo professor gera nova Tentativa
-- pré-preenchida (futuro). Atrasada é marcada quando entregue depois do
-- prazo efetivo com atraso permitido.

BEGIN;

CREATE TABLE submissoes (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    atribuicao_id BIGINT      NOT NULL REFERENCES atribuicoes (id) ON DELETE CASCADE,
    aluno_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    comecou_em    TIMESTAMPTZ NOT NULL,
    entregue_em   TIMESTAMPTZ NOT NULL DEFAULT now(),
    atrasada      BOOLEAN     NOT NULL DEFAULT false,
    observacao    TEXT, -- observação do aluno (opcional, §Submissão)
    snapshot      JSONB       NOT NULL,
    created_by    BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    UNIQUE (atribuicao_id, aluno_id),
    CHECK (entregue_em >= comecou_em)
);

CREATE INDEX submissoes_aluno_idx ON submissoes (aluno_id);
CREATE INDEX submissoes_turma_atrib_idx ON submissoes (atribuicao_id);

COMMIT;
