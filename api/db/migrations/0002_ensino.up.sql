-- 0002_ensino.sql — Organização acadêmica da fundação (PROJETO §Organização):
-- matérias, períodos letivos, turmas e matrículas. O professor cria e comanda
-- a própria turma (§Portal professor); admin/super enxergam tudo (§5.4).

BEGIN;

CREATE TABLE materias (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo      TEXT        NOT NULL UNIQUE CHECK (length(btrim(codigo)) > 0),
    nome        TEXT        NOT NULL CHECK (length(btrim(nome)) > 0),
    created_by  BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ -- soft delete: histórico de turmas se mantém
);

CREATE TABLE periodos_letivos (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trimestre  SMALLINT    NOT NULL CHECK (trimestre BETWEEN 1 AND 3),
    ano        SMALLINT    NOT NULL CHECK (ano BETWEEN 2000 AND 2100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (trimestre, ano)
);

CREATE TABLE turmas (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    materia_id    BIGINT      NOT NULL REFERENCES materias (id) ON DELETE RESTRICT,
    periodo_id    BIGINT      NOT NULL REFERENCES periodos_letivos (id) ON DELETE RESTRICT,
    professor_id  BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    titulo        TEXT, -- T1, T2, ... (opcional)
    encerrado_em  TIMESTAMPTZ,
    created_by    BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX turmas_professor_idx ON turmas (professor_id);
CREATE INDEX turmas_materia_idx ON turmas (materia_id);

CREATE TABLE matriculas (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    turma_id    BIGINT      NOT NULL REFERENCES turmas (id) ON DELETE CASCADE,
    aluno_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    entrada_em  TIMESTAMPTZ NOT NULL DEFAULT now(),
    saida_em    TIMESTAMPTZ, -- encerramento da matrícula ("remover aluno")
    created_by  BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (saida_em IS NULL OR saida_em >= entrada_em)
);

-- Uma matrícula ativa por aluno/turma; re-matrícula após saída cria nova linha.
CREATE UNIQUE INDEX matriculas_ativas_unicas
    ON matriculas (turma_id, aluno_id)
    WHERE saida_em IS NULL;

CREATE INDEX matriculas_aluno_idx ON matriculas (aluno_id);

COMMIT;
