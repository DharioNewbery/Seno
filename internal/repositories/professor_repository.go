package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"seno/internal/models"
)

// professorRoleName é o papel atribuído a toda conta de professor (migration 003).
const professorRoleName = "professor"

// professorProfileTable é a tabela de composição 1:1 do perfil de professor.
const professorProfileTable = "professors"

type ProfessorRepository struct {
	db *sqlx.DB
}

func NewProfessorRepository(db *sqlx.DB) *ProfessorRepository {
	return &ProfessorRepository{db: db}
}

// CreateWithAccount cria a conta completa de professor (usuário, credencial,
// vínculo e papel) em uma única transação.
func (r *ProfessorRepository) CreateWithAccount(ctx context.Context, user *models.User, passwordHash string) error {
	return createProfiledAccount(ctx, r.db, user, passwordHash, professorProfileTable, professorRoleName)
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
