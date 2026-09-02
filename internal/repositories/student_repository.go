package repositories

import (
	"context"

	"github.com/jmoiron/sqlx"

	"seno/internal/models"
)

// studentRoleName é o papel atribuído a toda conta de aluno (migration 004).
const studentRoleName = "student"

// studentProfileTable é a tabela de composição 1:1 do perfil de aluno.
const studentProfileTable = "students"

type StudentRepository struct {
	db *sqlx.DB
}

func NewStudentRepository(db *sqlx.DB) *StudentRepository {
	return &StudentRepository{db: db}
}

// CreateWithAccount cria a conta completa de aluno (usuário, credencial,
// vínculo e papel) em uma única transação.
func (r *StudentRepository) CreateWithAccount(ctx context.Context, user *models.User, passwordHash string) error {
	return createProfiledAccount(ctx, r.db, user, passwordHash, studentProfileTable, studentRoleName)
}
