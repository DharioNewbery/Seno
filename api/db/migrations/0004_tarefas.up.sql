-- 0004_tarefas.up.sql — Banco de tarefas (PROJETO §Tarefas/§Teste).
-- Tarefa: unidade de trabalho do professor; a linguagem NÃO pertence à
-- tarefa (definida por tarefa na Atividade). Limites de execução opcionais
-- (quando ausentes valem o padrão global de §Limites de execução).
-- Teste: caso de entrada/saída, público (aluno usa no "testar") ou privado.

BEGIN;

CREATE TABLE tarefas (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    professor_id   BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    nome           TEXT        NOT NULL CHECK (length(btrim(nome)) > 0),
    enunciado      TEXT        NOT NULL,
    tempo_cpu_ms   INTEGER     NOT NULL DEFAULT 0, -- 0 = padrão global
    tempo_total_ms INTEGER     NOT NULL DEFAULT 0,
    memoria_mb     INTEGER     NOT NULL DEFAULT 0,
    created_by     BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ -- soft delete quando usada em Atividade
);

CREATE INDEX tarefas_professor_idx ON tarefas (professor_id);

CREATE TABLE testes_tarefa (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tarefa_id       BIGINT      NOT NULL REFERENCES tarefas (id) ON DELETE CASCADE,
    stdin           TEXT        NOT NULL DEFAULT '',
    stdout_esperado TEXT        NOT NULL DEFAULT '',
    publico         BOOLEAN     NOT NULL DEFAULT false,
    created_by      BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX testes_tarefa_tarefa_idx ON testes_tarefa (tarefa_id);

COMMIT;
