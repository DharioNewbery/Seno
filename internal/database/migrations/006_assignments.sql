-- Migração: tarefas de programação e submissões de alunos
-- Convenção: nomes em inglês. Comentários/mensagens em português.

-- Tarefa publicada pelo professor numa turma dele.
-- language: 'python' | 'c' | 'cpp' (suporte inicial do produto).
-- status das submissões fica 'pending' até a correção automática (milestone 6).
CREATE TABLE IF NOT EXISTS assignments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id    UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    title       VARCHAR(150) NOT NULL,
    statement   TEXT NOT NULL,
    language    VARCHAR(20) NOT NULL,
    due_at      TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assignments_class_id ON assignments(class_id);

-- Submissão de código de um aluno para uma tarefa (via IDE web).
CREATE TABLE IF NOT EXISTS submissions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id    UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    student_user_id  UUID NOT NULL REFERENCES students(user_id) ON DELETE CASCADE,
    language         VARCHAR(20) NOT NULL,
    source_code      TEXT NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_submissions_assignment_id ON submissions(assignment_id);
CREATE INDEX IF NOT EXISTS idx_submissions_student_user_id ON submissions(student_user_id);
