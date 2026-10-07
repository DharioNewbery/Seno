-- 0005_atividades.up.sql — Atividades (PROJETO §Atividades/§Atribuição).
-- A atividade é um conjunto de tarefas (envelopadas), com conteúdo JSON
-- (schema_version; hidratado pelo backend na vá renderização) e, por
-- tarefa: ordem, pontuação e linguagem (python/c/cpp na v1). A partir da
-- primeira submissão não pode mais ser editada; exclusão segue §Atividades
-- (hard se nunca usada por submissão; soft caso contrário — futuro).

BEGIN;

CREATE TABLE atividades (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    professor_id BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    nome         TEXT        NOT NULL CHECK (length(btrim(nome)) > 0),
    conteudo     JSONB       NOT NULL,
    created_by   BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX atividades_professor_idx ON atividades (professor_id);

CREATE TABLE atividade_tarefas (
    atividade_id BIGINT      NOT NULL REFERENCES atividades (id) ON DELETE CASCADE,
    tarefa_id    BIGINT      NOT NULL REFERENCES tarefas (id) ON DELETE RESTRICT,
    ordem        INTEGER     NOT NULL,
    valor_pts    INTEGER     NOT NULL DEFAULT 0,
    linguagem    TEXT        NOT NULL CHECK (linguagem IN ('python', 'c', 'cpp')),
    PRIMARY KEY (atividade_id, tarefa_id)
);

CREATE INDEX atividade_tarefas_tarefa_idx ON atividade_tarefas (tarefa_id);

COMMIT;
