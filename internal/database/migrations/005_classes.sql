-- Migração: turmas e ingresso de alunos por código
-- Convenção: nomes em inglês. Comentários/mensagens em português.

-- Turma criada por um professor. join_code é o código de ingresso dos alunos.
CREATE TABLE IF NOT EXISTS classes (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name               VARCHAR(100) NOT NULL,
    description        VARCHAR(255),
    join_code          VARCHAR(8) NOT NULL UNIQUE,
    professor_user_id  UUID NOT NULL REFERENCES professors(user_id) ON DELETE CASCADE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_classes_professor_user_id ON classes(professor_user_id);

-- Associação aluno <-> turma (N:M). Ingresso via join_code.
CREATE TABLE IF NOT EXISTS class_members (
    class_id         UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    student_user_id  UUID NOT NULL REFERENCES students(user_id) ON DELETE CASCADE,
    joined_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (class_id, student_user_id)
);

CREATE INDEX IF NOT EXISTS idx_class_members_student_user_id ON class_members(student_user_id);
