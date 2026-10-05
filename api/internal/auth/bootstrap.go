// Package auth — bootstrap do super admin (ARQUITETURA §5.4).
package auth

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/store"
)

// BootstrapSuperAdmin cria o primeiro super admin na inicialização quando o
// sistema ainda não tem nenhum usuário (deploy/.env.example: "criado na
// inicialização se não existir"). Idempotente: com qualquer usuário
// existente, não faz nada. Todo o cadastro acontece em uma transação.
func BootstrapSuperAdmin(ctx context.Context, st *store.Store, email, password string) error {
	if email == "" || password == "" {
		return fmt.Errorf("bootstrap: SENO_SUPER_EMAIL e SENO_SUPER_PASSWORD são obrigatórios")
	}
	email = normalizeEmail(email)

	total, err := st.Q.CountUsers(ctx, gen.CountUsersParams{})
	if err != nil {
		return fmt.Errorf("bootstrap: contagem de usuários: %w", err)
	}
	if total > 0 {
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("bootstrap: hash da senha: %w", err)
	}

	if err := st.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		personID, err := q.InsertPerson(ctx, gen.InsertPersonParams{
			FirstName: "Super",
			LastName:  "Admin",
		})
		if err != nil {
			return fmt.Errorf("pessoa do super admin: %w", err)
		}
		user, err := q.InsertUser(ctx, gen.InsertUserParams{
			PersonID:     personID,
			Email:        email,
			PasswordHash: &hash,
			Status:       domain.StatusActive,
		})
		if err != nil {
			return fmt.Errorf("usuário do super admin: %w", err)
		}
		if err := q.InsertUserRole(ctx, gen.InsertUserRoleParams{
			UserID: user.ID,
			Role:   string(domain.RoleSuper),
		}); err != nil {
			return fmt.Errorf("cargo do super admin: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	slog.Info("super admin criado", "email", email)
	return nil
}
