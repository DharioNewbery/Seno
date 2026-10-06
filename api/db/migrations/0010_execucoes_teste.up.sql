-- 0010_execucoes_teste.up.sql — Execução de teste (PROJETO §Execução de
-- teste): registro de cada "testar" do ALUNO (o do professor não). Só
-- existe para tarefas com testes públicos; guarda resultados apenas
-- públicos. Retenção de 7 dias (limpeza nas inserções).

BEGIN;

CREATE TABLE execucoes_teste (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    aluno_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    atribuicao_id BIGINT      NOT NULL REFERENCES atribuicoes (id) ON DELETE CASCADE,
    tarefa_id     BIGINT      NOT NULL REFERENCES tarefas (id) ON DELETE CASCADE,
    linguagem     TEXT        NOT NULL,
    codigo        TEXT        NOT NULL,
    resultados    JSONB       NOT NULL,
    criado_em     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX execucoes_aluno_idx ON execucoes_teste (aluno_id, criado_em);

COMMIT;
