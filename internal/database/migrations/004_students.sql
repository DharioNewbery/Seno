-- Migração: cadastro de alunos (composição 1:1 com users)
-- Convenção: nomes em inglês. Comentários/mensagens em português.

-- Perfil de aluno: estende users. O ingresso em turmas (milestone seguinte)
-- será feito por código de turma, em tabela de associação própria.
CREATE TABLE IF NOT EXISTS students (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Papel de aluno
INSERT INTO roles (name, description) VALUES
    ('student', 'Aluno: entra em turmas via código e submete tarefas')
ON CONFLICT (name) DO NOTHING;
