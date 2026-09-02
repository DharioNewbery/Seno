package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"seno/internal/models"
	"seno/internal/repositories"
)

const (
	assignmentTitleMinLen = 3
	sourceCodeMaxLen      = 64000
)

// supportedLanguages define as linguagens aceitas no MVP (execução no milestone 6).
var supportedLanguages = map[string]bool{
	"python": true,
	"c":      true,
	"cpp":    true,
}

// LanguageLabel devolve o rótulo em português da linguagem (para mensagens).
func LanguageLabel(language string) string {
	switch language {
	case "python":
		return "Python"
	case "c":
		return "C"
	case "cpp":
		return "C++"
	default:
		return language
	}
}

type AssignmentService struct {
	assignmentRepo AssignmentRepository
}

func NewAssignmentService(assignmentRepo AssignmentRepository) *AssignmentService {
	return &AssignmentService{assignmentRepo: assignmentRepo}
}

type CreateAssignmentInput struct {
	ProfessorUserID uuid.UUID
	ClassID         uuid.UUID
	Title           string
	Statement       string
	Language        string
	DueAt           *time.Time
}

func (in CreateAssignmentInput) validate() *ValidationError {
	if len(strings.TrimSpace(in.Title)) < assignmentTitleMinLen {
		return NewValidationError("título da tarefa deve ter no mínimo 3 caracteres")
	}
	if strings.TrimSpace(in.Statement) == "" {
		return NewValidationError("enunciado é obrigatório")
	}
	if !supportedLanguages[in.Language] {
		return NewValidationError("linguagem não suportada (use python, c ou cpp)")
	}
	return nil
}

// CreateAssignment valida e publica uma tarefa na turma do professor.
func (s *AssignmentService) CreateAssignment(ctx context.Context, in CreateAssignmentInput) (*models.Assignment, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	owner, err := s.assignmentRepo.IsClassOwner(ctx, in.ClassID, in.ProfessorUserID)
	if err != nil {
		return nil, err
	}
	if !owner {
		return nil, ErrNotClassOwner
	}

	assignment := &models.Assignment{
		ClassID:   in.ClassID,
		Title:     strings.TrimSpace(in.Title),
		Statement: in.Statement,
		Language:  in.Language,
		DueAt:     in.DueAt,
	}
	if err := s.assignmentRepo.Create(ctx, assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

// ListByClass retorna as tarefas da turma, se o solicitante for o professor dono.
func (s *AssignmentService) ListByClass(ctx context.Context, professorUserID, classID uuid.UUID) ([]models.AssignmentSummary, error) {
	owner, err := s.assignmentRepo.IsClassOwner(ctx, classID, professorUserID)
	if err != nil {
		return nil, err
	}
	if !owner {
		return nil, ErrNotClassOwner
	}
	return s.assignmentRepo.ListByClass(ctx, classID)
}

// ListMine retorna as tarefas de todas as turmas nas quais o aluno ingressou.
func (s *AssignmentService) ListMine(ctx context.Context, studentUserID uuid.UUID) ([]models.AssignmentSummary, error) {
	return s.assignmentRepo.ListByStudent(ctx, studentUserID)
}

// GetDetail retorna a tarefa com as submissões visíveis ao solicitante:
// todas (professor dono da turma) ou apenas as próprias (aluno membro).
func (s *AssignmentService) GetDetail(ctx context.Context, requesterID, assignmentID uuid.UUID) (*models.AssignmentDetail, error) {
	detail, err := s.assignmentRepo.GetDetail(ctx, assignmentID)
	if err != nil {
		return nil, err
	}

	owner, err := s.assignmentRepo.IsClassOwner(ctx, detail.ClassID, requesterID)
	if err != nil {
		return nil, err
	}
	if owner {
		detail.Submissions, err = s.assignmentRepo.ListSubmissionsByAssignment(ctx, assignmentID)
		return detail, err
	}

	member, err := s.assignmentRepo.IsClassMember(ctx, detail.ClassID, requesterID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotClassMember
	}
	detail.Submissions, err = s.assignmentRepo.ListSubmissionsByStudent(ctx, assignmentID, requesterID)
	if err != nil {
		return nil, err
	}

	// Rascunho do editor online (backup); ausência não é erro.
	draft, err := s.assignmentRepo.GetDraft(ctx, assignmentID, requesterID)
	if err != nil {
		if !errors.Is(err, repositories.ErrDraftNotFound) {
			return nil, err
		}
	} else {
		detail.Draft = draft
	}
	return detail, nil
}

type SaveDraftInput struct {
	AssignmentID  uuid.UUID
	StudentUserID uuid.UUID
	SourceCode    string
}

func (in SaveDraftInput) validate() *ValidationError {
	if len(in.SourceCode) > sourceCodeMaxLen {
		return NewValidationError("rascunho excede o limite de 64.000 caracteres")
	}
	return nil
}

// SaveDraft cria ou atualiza o rascunho do aluno na tarefa (backup do editor
// online). Rascunho vazio é válido: o aluno pode limpar o editor.
func (s *AssignmentService) SaveDraft(ctx context.Context, in SaveDraftInput) (*models.Draft, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	assignment, err := s.assignmentRepo.GetByID(ctx, in.AssignmentID)
	if err != nil {
		return nil, err
	}

	member, err := s.assignmentRepo.IsClassMember(ctx, assignment.ClassID, in.StudentUserID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotClassMember
	}

	draft := &models.Draft{
		AssignmentID:  in.AssignmentID,
		StudentUserID: in.StudentUserID,
		SourceCode:    in.SourceCode,
	}
	if err := s.assignmentRepo.UpsertDraft(ctx, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

type SubmitInput struct {
	AssignmentID  uuid.UUID
	StudentUserID uuid.UUID
	SourceCode    string
}

func (in SubmitInput) validate() *ValidationError {
	if in.SourceCode == "" {
		return NewValidationError("código-fonte é obrigatório")
	}
	if len(in.SourceCode) > sourceCodeMaxLen {
		return NewValidationError("código-fonte excede o limite de 64.000 caracteres")
	}
	return nil
}

// Submit registra a submissão de código do aluno na tarefa (via IDE web).
// A linguagem é herdada da tarefa; o status inicial é 'pending'.
func (s *AssignmentService) Submit(ctx context.Context, in SubmitInput) (*models.Submission, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	assignment, err := s.assignmentRepo.GetByID(ctx, in.AssignmentID)
	if err != nil {
		return nil, err
	}

	member, err := s.assignmentRepo.IsClassMember(ctx, assignment.ClassID, in.StudentUserID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotClassMember
	}

	submission := &models.Submission{
		AssignmentID:  in.AssignmentID,
		StudentUserID: in.StudentUserID,
		Language:      assignment.Language,
		SourceCode:    in.SourceCode,
	}
	if err := s.assignmentRepo.CreateSubmission(ctx, submission); err != nil {
		return nil, err
	}
	return submission, nil
}
