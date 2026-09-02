-- Migração: rascunhos do editor online (backup em tempo real)
-- Convenção: nomes em inglês. Comentários/mensagens em português.

-- Rascunho de aluno numa tarefa: backup do que está sendo digitado na IDE.
-- Um rascunho por aluno/tarefa; sincronizado via upsert.
CREATE TABLE IF NOT EXISTS drafts (
    assignment_id    UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    student_user_id  UUID NOT NULL REFERENCES students(user_id) ON DELETE CASCADE,
    source_code      TEXT NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (assignment_id, student_user_id)
);
