-- Migração: casos de teste das tarefas e resultado da correção automática
-- Convenção: nomes em inglês. Comentários/mensagens em português.

-- Caso de teste: entrada (stdin) e saída esperada (stdout).
-- Todos os casos são visíveis ao aluno (decisão do MVP).
CREATE TABLE IF NOT EXISTS assignment_tests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id   UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    position        INT NOT NULL,
    input           TEXT NOT NULL DEFAULT '',
    expected_output TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assignment_tests_assignment_id ON assignment_tests(assignment_id);

-- Detalhe da correção automática (JSON por caso de teste), gravado pelo worker
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS result TEXT;
