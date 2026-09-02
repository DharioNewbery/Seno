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

type ClassRepository struct {
	db *sqlx.DB
}

func NewClassRepository(db *sqlx.DB) *ClassRepository {
	return &ClassRepository{db: db}
}

func (r *ClassRepository) Create(ctx context.Context, class *models.Class) error {
	query := `INSERT INTO classes (name, description, join_code, professor_user_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, join_code, professor_user_id, created_at, updated_at`

	if err := r.db.QueryRowxContext(ctx, query,
		class.Name, class.Description, class.JoinCode, class.ProfessorUserID).
		StructScan(class); err != nil {
		// join_code é a única coluna única de classes: 23505 aqui é colisão de código.
		if isUniqueViolation(err) {
			return ErrJoinCodeCollision
		}
		return fmt.Errorf("erro ao criar turma: %w", err)
	}
	return nil
}

func (r *ClassRepository) GetByJoinCode(ctx context.Context, code string) (*models.Class, error) {
	var c models.Class
	query := `SELECT id, name, description, join_code, professor_user_id, created_at, updated_at
		FROM classes WHERE join_code = $1`

	if err := r.db.GetContext(ctx, &c, query, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrClassNotFound
		}
		return nil, fmt.Errorf("erro ao buscar turma por código: %w", err)
	}
	return &c, nil
}

func (r *ClassRepository) ListByProfessor(ctx context.Context, professorUserID uuid.UUID) ([]models.ClassSummary, error) {
	var classes []models.ClassSummary
	query := `SELECT c.id, c.name, c.description, c.join_code, c.created_at,
			COUNT(cm.student_user_id) AS member_count
		FROM classes c
		LEFT JOIN class_members cm ON cm.class_id = c.id
		WHERE c.professor_user_id = $1
		GROUP BY c.id
		ORDER BY c.created_at DESC`

	if err := r.db.SelectContext(ctx, &classes, query, professorUserID); err != nil {
		return nil, fmt.Errorf("erro ao listar turmas do professor: %w", err)
	}
	return classes, nil
}

func (r *ClassRepository) ListByStudent(ctx context.Context, studentUserID uuid.UUID) ([]models.StudentClass, error) {
	var classes []models.StudentClass
	query := `SELECT c.id, c.name, c.description, u.full_name AS professor_name, cm.joined_at
		FROM classes c
		JOIN class_members cm ON cm.class_id = c.id
		JOIN users u ON u.id = c.professor_user_id
		WHERE cm.student_user_id = $1
		ORDER BY cm.joined_at DESC`

	if err := r.db.SelectContext(ctx, &classes, query, studentUserID); err != nil {
		return nil, fmt.Errorf("erro ao listar turmas do aluno: %w", err)
	}
	return classes, nil
}

// AddMember insere o aluno na turma. Retorna false quando ele já era membro
// (idempotente: reingressar na mesma turma não é erro de dados).
func (r *ClassRepository) AddMember(ctx context.Context, classID, studentUserID uuid.UUID) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO class_members (class_id, student_user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`,
		classID, studentUserID)
	if err != nil {
		return false, fmt.Errorf("erro ao ingressar aluno na turma: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}
