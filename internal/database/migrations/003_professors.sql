-- Migração: cadastro de professores (composição 1:1 com users)
-- Convenção: nomes em inglês. Comentários/mensagens em português.

-- Perfil de professor: estende users sem duplicar dados de conta.
-- Campos específicos (ex.: matrícula, departamento) entram em migrações futuras.
CREATE TABLE IF NOT EXISTS professors (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Papel de professor (permissões granulares chegam com as turmas, milestone 4)
INSERT INTO roles (name, description) VALUES
    ('professor', 'Professor: cria turmas e tarefas')
ON CONFLICT (name) DO NOTHING;
