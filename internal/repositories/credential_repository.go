package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"seno/internal/models"
)

type CredentialRepository struct {
	db *sqlx.DB
}

func NewCredentialRepository(db *sqlx.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

func (r *CredentialRepository) Create(ctx context.Context, c *models.UserCredential) error {
	query := `INSERT INTO user_credentials (user_id, password_hash)
		VALUES ($1, $2)
		RETURNING id, user_id, password_hash, failed_login_attempts, created_at, updated_at`

	if err := r.db.QueryRowxContext(ctx, query, c.UserID, c.PasswordHash).
		StructScan(c); err != nil {
		if isUniqueViolation(err) {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("erro ao criar credencial: %w", err)
	}
	return nil
}

func (r *CredentialRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.UserCredential, error) {
	var c models.UserCredential
	query := `SELECT id, user_id, password_hash, last_login_at,
		failed_login_attempts, locked_until, created_at, updated_at
		FROM user_credentials WHERE user_id = $1`

	if err := r.db.GetContext(ctx, &c, query, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCredentialNotFound
		}
		return nil, fmt.Errorf("erro ao buscar credencial: %w", err)
	}
	return &c, nil
}

func (r *CredentialRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `UPDATE user_credentials
		SET password_hash = $1, updated_at = NOW()
		WHERE user_id = $2`

	res, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar senha: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrCredentialNotFound
	}
	return nil
}

func (r *CredentialRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE user_credentials
		SET last_login_at = NOW(), failed_login_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE user_id = $1`

	res, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar último login: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrCredentialNotFound
	}
	return nil
}

// IncrementFailedAttempts registra uma tentativa falha e bloqueia a conta
// quando o limite de 5 tentativas é atingido. Retorna a contagem atual.
func (r *CredentialRepository) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) (int, error) {
	var c models.UserCredential
	query := `UPDATE user_credentials
		SET failed_login_attempts = failed_login_attempts + 1, updated_at = NOW()
		WHERE user_id = $1
		RETURNING failed_login_attempts`

	if err := r.db.GetContext(ctx, &c, query, userID); err != nil {
		return 0, fmt.Errorf("erro ao incrementar tentativas falhas: %w", err)
	}
	if c.FailedLoginAttempts >= 5 {
		if _, err := r.db.ExecContext(ctx,
			`UPDATE user_credentials SET locked_until = NOW() + INTERVAL '15 minutes', updated_at = NOW() WHERE user_id = $1`,
			userID); err != nil {
			return c.FailedLoginAttempts, fmt.Errorf("erro ao bloquear conta: %w", err)
		}
	}
	return c.FailedLoginAttempts, nil
}

func (r *CredentialRepository) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_credentials SET failed_login_attempts = 0, locked_until = NULL, updated_at = NOW() WHERE user_id = $1`,
		userID)
	if err != nil {
		return fmt.Errorf("erro ao reiniciar tentativas: %w", err)
	}
	return nil
}
