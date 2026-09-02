package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"seno/internal/models"
)

// professorRoleName é o papel atribuído a toda conta de professor (migration 003).
const professorRoleName = "professor"

type ProfessorRepository struct {
	db *sqlx.DB
}

func NewProfessorRepository(db *sqlx.DB) *ProfessorRepository {
	return &ProfessorRepository{db: db}
}

// CreateWithAccount cria a conta completa de professor (usuário, credencial,
// vínculo e papel) em uma única transação: ou tudo existe, ou nada.
func (r *ProfessorRepository) CreateWithAccount(ctx context.Context, user *models.User, passwordHash string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
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
		`INSERT INTO professors (user_id) VALUES ($1)`,
		user.ID); err != nil {
		return fmt.Errorf("erro ao criar vínculo de professor: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2`,
		user.ID, professorRoleName)
	if err != nil {
		return fmt.Errorf("erro ao atribuir papel de professor: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrRoleNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao confirmar transação: %w", err)
	}
	return nil
}

// List retorna os professores ativos com seus dados públicos, ordenados
// pelo vínculo mais recente.
func (r *ProfessorRepository) List(ctx context.Context) ([]models.ProfessorUser, error) {
	var professors []models.ProfessorUser
	query := `SELECT p.user_id, u.full_name, u.email, p.created_at
		FROM professors p
		JOIN users u ON u.id = p.user_id
		WHERE u.deleted_at IS NULL
		ORDER BY p.created_at DESC`

	if err := r.db.SelectContext(ctx, &professors, query); err != nil {
		return nil, fmt.Errorf("erro ao listar professores: %w", err)
	}
	return professors, nil
}
