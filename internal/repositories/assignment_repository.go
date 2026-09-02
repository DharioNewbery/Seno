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

type AssignmentRepository struct {
	db *sqlx.DB
}

func NewAssignmentRepository(db *sqlx.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

func (r *AssignmentRepository) Create(ctx context.Context, a *models.Assignment) error {
	query := `INSERT INTO assignments (class_id, title, statement, language, due_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, class_id, title, statement, language, due_at, created_at, updated_at`

	if err := r.db.QueryRowxContext(ctx, query,
		a.ClassID, a.Title, a.Statement, a.Language, a.DueAt).
		StructScan(a); err != nil {
		return fmt.Errorf("erro ao criar tarefa: %w", err)
	}
	return nil
}

func (r *AssignmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Assignment, error) {
	var a models.Assignment
	query := `SELECT id, class_id, title, statement, language, due_at, created_at, updated_at
		FROM assignments WHERE id = $1`

	if err := r.db.GetContext(ctx, &a, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("erro ao buscar tarefa por id: %w", err)
	}
	return &a, nil
}

func (r *AssignmentRepository) GetDetail(ctx context.Context, id uuid.UUID) (*models.AssignmentDetail, error) {
	var d models.AssignmentDetail
	query := `SELECT a.id, a.class_id, c.name AS class_name, a.title, a.statement,
			a.language, a.due_at, a.created_at, a.updated_at
		FROM assignments a
		JOIN classes c ON c.id = a.class_id
		WHERE a.id = $1`

	if err := r.db.GetContext(ctx, &d, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("erro ao buscar detalhe da tarefa: %w", err)
	}
	return &d, nil
}

func (r *AssignmentRepository) ListByClass(ctx context.Context, classID uuid.UUID) ([]models.AssignmentSummary, error) {
	var assignments []models.AssignmentSummary
	query := `SELECT a.id, a.class_id, c.name AS class_name, a.title, a.language, a.due_at, a.created_at
		FROM assignments a
		JOIN classes c ON c.id = a.class_id
		WHERE a.class_id = $1
		ORDER BY a.created_at DESC`

	if err := r.db.SelectContext(ctx, &assignments, query, classID); err != nil {
		return nil, fmt.Errorf("erro ao listar tarefas da turma: %w", err)
	}
	return assignments, nil
}

func (r *AssignmentRepository) ListByStudent(ctx context.Context, studentUserID uuid.UUID) ([]models.AssignmentSummary, error) {
	var assignments []models.AssignmentSummary
	query := `SELECT a.id, a.class_id, c.name AS class_name, a.title, a.language, a.due_at, a.created_at
		FROM assignments a
		JOIN classes c ON c.id = a.class_id
		JOIN class_members cm ON cm.class_id = a.class_id
		WHERE cm.student_user_id = $1
		ORDER BY a.created_at DESC`

	if err := r.db.SelectContext(ctx, &assignments, query, studentUserID); err != nil {
		return nil, fmt.Errorf("erro ao listar tarefas do aluno: %w", err)
	}
	return assignments, nil
}

// IsClassOwner verifica se o usuário é o professor dono da turma.
func (r *AssignmentRepository) IsClassOwner(ctx context.Context, classID, professorUserID uuid.UUID) (bool, error) {
	var owner bool
	if err := r.db.GetContext(ctx, &owner,
		`SELECT EXISTS(SELECT 1 FROM classes WHERE id = $1 AND professor_user_id = $2)`,
		classID, professorUserID); err != nil {
		return false, fmt.Errorf("erro ao verificar dono da turma: %w", err)
	}
	return owner, nil
}

// IsClassMember verifica se o usuário é aluno ingressado na turma.
// O JOIN com students garante que só contas de aluno podem ser membros.
func (r *AssignmentRepository) IsClassMember(ctx context.Context, classID, studentUserID uuid.UUID) (bool, error) {
	var member bool
	if err := r.db.GetContext(ctx, &member,
		`SELECT EXISTS(
			SELECT 1 FROM class_members cm
			JOIN students s ON s.user_id = cm.student_user_id
			WHERE cm.class_id = $1 AND cm.student_user_id = $2)`,
		classID, studentUserID); err != nil {
		return false, fmt.Errorf("erro ao verificar membro da turma: %w", err)
	}
	return member, nil
}

func (r *AssignmentRepository) CreateSubmission(ctx context.Context, s *models.Submission) error {
	query := `INSERT INTO submissions (assignment_id, student_user_id, language, source_code)
		VALUES ($1, $2, $3, $4)
		RETURNING id, assignment_id, student_user_id, language, source_code, status, created_at`

	if err := r.db.QueryRowxContext(ctx, query,
		s.AssignmentID, s.StudentUserID, s.Language, s.SourceCode).
		StructScan(s); err != nil {
		return fmt.Errorf("erro ao criar submissão: %w", err)
	}
	return nil
}

func (r *AssignmentRepository) ListSubmissionsByAssignment(ctx context.Context, assignmentID uuid.UUID) ([]models.SubmissionView, error) {
	var submissions []models.SubmissionView
	query := `SELECT s.id, s.student_user_id, u.full_name AS student_name, s.language,
			s.source_code, s.status, s.created_at
		FROM submissions s
		JOIN users u ON u.id = s.student_user_id
		WHERE s.assignment_id = $1
		ORDER BY s.created_at DESC`

	if err := r.db.SelectContext(ctx, &submissions, query, assignmentID); err != nil {
		return nil, fmt.Errorf("erro ao listar submissões da tarefa: %w", err)
	}
	return submissions, nil
}

func (r *AssignmentRepository) ListSubmissionsByStudent(ctx context.Context, assignmentID, studentUserID uuid.UUID) ([]models.SubmissionView, error) {
	var submissions []models.SubmissionView
	query := `SELECT s.id, s.student_user_id, u.full_name AS student_name, s.language,
			s.source_code, s.status, s.created_at
		FROM submissions s
		JOIN users u ON u.id = s.student_user_id
		WHERE s.assignment_id = $1 AND s.student_user_id = $2
		ORDER BY s.created_at DESC`

	if err := r.db.SelectContext(ctx, &submissions, query, assignmentID, studentUserID); err != nil {
		return nil, fmt.Errorf("erro ao listar submissões do aluno: %w", err)
	}
	return submissions, nil
}
