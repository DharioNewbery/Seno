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

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (full_name, email, username)
		VALUES ($1, $2, $3)
		RETURNING id, full_name, email, username, created_at, updated_at`

	if err := r.db.QueryRowxContext(ctx, query, user.FullName, user.Email, user.Username).
		StructScan(user); err != nil {
		if isUniqueViolation(err) {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("erro ao criar usuário: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	query := `SELECT id, full_name, email, username, created_at, updated_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`

	if err := r.db.GetContext(ctx, &u, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("erro ao buscar usuário por id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	query := `SELECT id, full_name, email, username, created_at, updated_at
		FROM users WHERE email = $1 AND deleted_at IS NULL`

	if err := r.db.GetContext(ctx, &u, query, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("erro ao buscar usuário por email: %w", err)
	}
	return &u, nil
}

// GetByLogin busca um usuário ativo por email ou nome de usuário.
// O identificador deve vir normalizado (trim + lowercase) da camada de serviço.
func (r *UserRepository) GetByLogin(ctx context.Context, login string) (*models.User, error) {
	var u models.User
	query := `SELECT id, full_name, email, username, created_at, updated_at
		FROM users WHERE deleted_at IS NULL AND (email = $1 OR username = $1)`

	if err := r.db.GetContext(ctx, &u, query, login); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("erro ao buscar usuário por login: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context) ([]models.User, error) {
	var users []models.User
	query := `SELECT id, full_name, email, username, created_at, updated_at
		FROM users WHERE deleted_at IS NULL ORDER BY created_at DESC`

	if err := r.db.SelectContext(ctx, &users, query); err != nil {
		return nil, fmt.Errorf("erro ao listar usuários: %w", err)
	}
	return users, nil
}
