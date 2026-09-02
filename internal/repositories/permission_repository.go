package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"seno/internal/models"
)

type PermissionRepository struct {
	db *sqlx.DB
}

func NewPermissionRepository(db *sqlx.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

func (r *PermissionRepository) Create(ctx context.Context, p *models.Permission) error {
	query := `INSERT INTO permissions (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, created_at, updated_at`

	if err := r.db.QueryRowxContext(ctx, query, p.Name, p.Description).
		StructScan(p); err != nil {
		if isUniqueViolation(err) {
			return ErrPermissionNotFound
		}
		return fmt.Errorf("erro ao criar permissão: %w", err)
	}
	return nil
}

func (r *PermissionRepository) GetByName(ctx context.Context, name string) (*models.Permission, error) {
	var p models.Permission
	if err := r.db.GetContext(ctx, &p,
		`SELECT id, name, description, created_at, updated_at FROM permissions WHERE name = $1`, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPermissionNotFound
		}
		return nil, fmt.Errorf("erro ao buscar permissão por nome: %w", err)
	}
	return &p, nil
}

func (r *PermissionRepository) List(ctx context.Context) ([]models.Permission, error) {
	var perms []models.Permission
	if err := r.db.SelectContext(ctx, &perms,
		`SELECT id, name, description, created_at, updated_at FROM permissions ORDER BY name ASC`); err != nil {
		return nil, fmt.Errorf("erro ao listar permissões: %w", err)
	}
	return perms, nil
}
