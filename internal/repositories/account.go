package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"seno/internal/models"
)

// createProfiledAccount cria usuário, credencial, vínculo de perfil e papel em
// uma única transação: ou tudo existe, ou nada. profileTable é a tabela de
// composição 1:1 com users (ex.: professors, students) e roleName o papel
// correspondente; chamadores passam apenas constantes deste pacote.
func createProfiledAccount(ctx context.Context, db *sqlx.DB, user *models.User, passwordHash, profileTable, roleName string) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op após Commit

	if err := tx.QueryRowxContext(ctx,
		`INSERT INTO users (full_name, email, username)
		VALUES ($1, $2, $3)
		RETURNING id, full_name, email, username, created_at, updated_at`,
		user.FullName, user.Email, user.Username).StructScan(user); err != nil {
		if isUniqueViolation(err) {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("erro ao criar usuário: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2)`,
		user.ID, passwordHash); err != nil {
		return fmt.Errorf("erro ao criar credencial: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (user_id) VALUES ($1)`, profileTable),
		user.ID); err != nil {
		return fmt.Errorf("erro ao criar vínculo de perfil: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2`,
		user.ID, roleName)
	if err != nil {
		return fmt.Errorf("erro ao atribuir papel: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrRoleNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao confirmar transação: %w", err)
	}
	return nil
}
