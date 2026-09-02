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

type RoleRepository struct {
	db *sqlx.DB
}

func NewRoleRepository(db *sqlx.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(ctx context.Context, role *models.Role) error {
	query := `INSERT INTO roles (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, created_at, updated_at`

	if err := r.db.QueryRowxContext(ctx, query, role.Name, role.Description).
		StructScan(role); err != nil {
		if isUniqueViolation(err) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("erro ao criar papel: %w", err)
	}
	return nil
}

func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	var role models.Role
	if err := r.db.GetContext(ctx, &role,
		`SELECT id, name, description, created_at, updated_at FROM roles WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("erro ao buscar papel por id: %w", err)
	}
	return &role, nil
}

func (r *RoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	var role models.Role
	if err := r.db.GetContext(ctx, &role,
		`SELECT id, name, description, created_at, updated_at FROM roles WHERE name = $1`, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("erro ao buscar papel por nome: %w", err)
	}
	return &role, nil
}

func (r *RoleRepository) List(ctx context.Context) ([]models.Role, error) {
	var roles []models.Role
	if err := r.db.SelectContext(ctx, &roles,
		`SELECT id, name, description, created_at, updated_at FROM roles ORDER BY name ASC`); err != nil {
		return nil, fmt.Errorf("erro ao listar papéis: %w", err)
	}
	return roles, nil
}

func (r *RoleRepository) AssignToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, roleID)
	if err != nil {
		return fmt.Errorf("erro ao atribuir papel ao usuário: %w", err)
	}
	return nil
}

func (r *RoleRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	var roles []models.Role
	query := `SELECT r.id, r.name, r.description, r.created_at, r.updated_at
		FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1 ORDER BY r.name ASC`

	if err := r.db.SelectContext(ctx, &roles, query, userID); err != nil {
		return nil, fmt.Errorf("erro ao buscar papéis do usuário: %w", err)
	}
	return roles, nil
}

// HasAnyRole verifica se o usuário possui ao menos um dos papéis informados.
func (r *RoleRepository) HasAnyRole(ctx context.Context, userID uuid.UUID, roles []string) (bool, error) {
	if len(roles) == 0 {
		return true, nil
	}
	query, args, err := sqlx.In(
		`SELECT EXISTS(
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = ? AND r.name IN (?))`,
		userID, roles)
	if err != nil {
		return false, fmt.Errorf("erro ao montar consulta de papéis: %w", err)
	}
	query = r.db.Rebind(query)

	var has bool
	if err := r.db.GetContext(ctx, &has, query, args...); err != nil {
		return false, fmt.Errorf("erro ao verificar papéis do usuário: %w", err)
	}
	return has, nil
}

// HasAnyPermission verifica se o usuário possui ao menos uma das permissões informadas
// (via papéis atribuídos).
func (r *RoleRepository) HasAnyPermission(ctx context.Context, userID uuid.UUID, permissions []string) (bool, error) {
	if len(permissions) == 0 {
		return true, nil
	}
	query, args, err := sqlx.In(
		`SELECT EXISTS(
			SELECT 1 FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = ? AND p.name IN (?))`,
		userID, permissions)
	if err != nil {
		return false, fmt.Errorf("erro ao montar consulta de permissões: %w", err)
	}
	query = r.db.Rebind(query)

	var has bool
	if err := r.db.GetContext(ctx, &has, query, args...); err != nil {
		return false, fmt.Errorf("erro ao verificar permissões do usuário: %w", err)
	}
	return has, nil
}
