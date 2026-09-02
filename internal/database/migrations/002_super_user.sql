-- Migração: login por nome de usuário e papel de superusuário
-- Convenção: nomes em inglês. Comentários/mensagens em português.

-- Login alternativo por nome de usuário (opcional; normalizado para lowercase pela aplicação)
ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(100);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Papel de superusuário (privilégios totais, acima do admin)
INSERT INTO roles (name, description) VALUES
    ('super', 'Superusuário com privilégios totais')
ON CONFLICT (name) DO NOTHING;

-- Superusuário recebe todas as permissões
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'super'
ON CONFLICT DO NOTHING;
