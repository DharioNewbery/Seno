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

// CreateWithTests cria a tarefa e seus casos de teste em uma única transação.
// Popula a.ID e as posições dos testes (1..N).
func (r *AssignmentRepository) CreateWithTests(ctx context.Context, a *models.Assignment, tests []models.AssignmentTest) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op após Commit

	if err := tx.QueryRowxContext(ctx,
		`INSERT INTO assignments (class_id, title, statement, language, due_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, class_id, title, statement, language, due_at, created_at, updated_at`,
		a.ClassID, a.Title, a.Statement, a.Language, a.DueAt).
		StructScan(a); err != nil {
		return fmt.Errorf("erro ao criar tarefa: %w", err)
	}

	for i := range tests {
		tests[i].AssignmentID = a.ID
		tests[i].Position = i + 1
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO assignment_tests (assignment_id, position, input, expected_output)
			VALUES ($1, $2, $3, $4)`,
			tests[i].AssignmentID, tests[i].Position, tests[i].Input, tests[i].ExpectedOutput); err != nil {
			return fmt.Errorf("erro ao criar caso de teste: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao confirmar transação: %w", err)
	}
	return nil
}

func (r *AssignmentRepository) ListTests(ctx context.Context, assignmentID uuid.UUID) ([]models.AssignmentTest, error) {
	var tests []models.AssignmentTest
	query := `SELECT id, assignment_id, position, input, expected_output, created_at
		FROM assignment_tests
		WHERE assignment_id = $1
		ORDER BY position ASC`

	if err := r.db.SelectContext(ctx, &tests, query, assignmentID); err != nil {
		return nil, fmt.Errorf("erro ao listar casos de teste: %w", err)
	}
	return tests, nil
}

// submissionStatusPending é o status de submissão aguardando correção
// (o worker de correção consome essas linhas).
const submissionStatusPending = "pending"

// ListPendingSubmissions retorna as submissões aguardando correção, mais
// antigas primeiro (consumidas pelo worker).
func (r *AssignmentRepository) ListPendingSubmissions(ctx context.Context, limit int) ([]models.Submission, error) {
	var submissions []models.Submission
	query := `SELECT id, assignment_id, student_user_id, language, source_code, status, result, created_at
		FROM submissions
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2`

	if err := r.db.SelectContext(ctx, &submissions, query, submissionStatusPending, limit); err != nil {
		return nil, fmt.Errorf("erro ao listar submissões pendentes: %w", err)
	}
	return submissions, nil
}

// UpdateSubmissionResult grava o status final e o detalhe (JSON) da correção.
func (r *AssignmentRepository) UpdateSubmissionResult(ctx context.Context, id uuid.UUID, status, result string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE submissions SET status = $2, result = $3 WHERE id = $1`,
		id, status, result)
	if err != nil {
		return fmt.Errorf("erro ao atualizar resultado da submissão: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrSubmissionNotFound
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
			s.source_code, s.status, s.result, s.created_at
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
			s.source_code, s.status, s.result, s.created_at
		FROM submissions s
		JOIN users u ON u.id = s.student_user_id
		WHERE s.assignment_id = $1 AND s.student_user_id = $2
		ORDER BY s.created_at DESC`

	if err := r.db.SelectContext(ctx, &submissions, query, assignmentID, studentUserID); err != nil {
		return nil, fmt.Errorf("erro ao listar submissões do aluno: %w", err)
	}
	return submissions, nil
}

// UpsertDraft cria ou atualiza o rascunho do aluno na tarefa (backup do editor).
func (r *AssignmentRepository) UpsertDraft(ctx context.Context, d *models.Draft) error {
	query := `INSERT INTO drafts (assignment_id, student_user_id, source_code)
		VALUES ($1, $2, $3)
		ON CONFLICT (assignment_id, student_user_id)
		DO UPDATE SET source_code = EXCLUDED.source_code, updated_at = NOW()
		RETURNING assignment_id, student_user_id, source_code, updated_at`

	if err := r.db.QueryRowxContext(ctx, query,
		d.AssignmentID, d.StudentUserID, d.SourceCode).
		StructScan(d); err != nil {
		return fmt.Errorf("erro ao salvar rascunho: %w", err)
	}
	return nil
}

func (r *AssignmentRepository) GetDraft(ctx context.Context, assignmentID, studentUserID uuid.UUID) (*models.Draft, error) {
	var d models.Draft
	query := `SELECT assignment_id, student_user_id, source_code, updated_at
		FROM drafts WHERE assignment_id = $1 AND student_user_id = $2`

	if err := r.db.GetContext(ctx, &d, query, assignmentID, studentUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDraftNotFound
		}
		return nil, fmt.Errorf("erro ao buscar rascunho: %w", err)
	}
	return &d, nil
}
