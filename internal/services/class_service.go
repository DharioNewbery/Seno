package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"seno/internal/models"
	"seno/internal/repositories"
)

const (
	// studentRoleName é o papel exigido para ingressar em turmas.
	studentRoleName = "student"

	classNameMinLen        = 3
	classDescriptionMaxLen = 255
	joinCodeLength         = 6
	joinCodeAttempts       = 3
	// Alfabeto sem caracteres ambíguos (0/O, 1/I) para leitura manual.
	joinCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

type ClassService struct {
	classRepo ClassRepository
	roleRepo  RoleRepository
}

func NewClassService(classRepo ClassRepository, roleRepo RoleRepository) *ClassService {
	return &ClassService{classRepo: classRepo, roleRepo: roleRepo}
}

type CreateClassInput struct {
	ProfessorUserID uuid.UUID
	Name            string
	Description     string
}

func (in CreateClassInput) validate() *ValidationError {
	if len(strings.TrimSpace(in.Name)) < classNameMinLen {
		return NewValidationError(fmt.Sprintf("nome da turma deve ter no mínimo %d caracteres", classNameMinLen))
	}
	if len(strings.TrimSpace(in.Description)) > classDescriptionMaxLen {
		return NewValidationError(fmt.Sprintf("descrição deve ter no máximo %d caracteres", classDescriptionMaxLen))
	}
	return nil
}

// CreateClass valida e cria a turma do professor, gerando um código de
// ingresso único (tenta novamente em caso de colisão).
func (s *ClassService) CreateClass(ctx context.Context, in CreateClassInput) (*models.Class, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	description := nullableDescription(in.Description)

	var class *models.Class
	for attempt := 0; attempt < joinCodeAttempts; attempt++ {
		code, err := generateJoinCode()
		if err != nil {
			return nil, err
		}
		candidate := &models.Class{
			Name:            strings.TrimSpace(in.Name),
			Description:     description,
			JoinCode:        code,
			ProfessorUserID: in.ProfessorUserID,
		}
		err = s.classRepo.Create(ctx, candidate)
		if err == nil {
			class = candidate
			break
		}
		if !errors.Is(err, repositories.ErrJoinCodeCollision) {
			return nil, err
		}
	}
	if class == nil {
		return nil, fmt.Errorf("não foi possível gerar um código de turma único após %d tentativas", joinCodeAttempts)
	}

	return class, nil
}

type JoinClassInput struct {
	StudentUserID uuid.UUID
	Code          string
}

// JoinByCode ingressa o aluno na turma do código informado. Idempotente no
// banco; reingressar na mesma turma retorna ErrAlreadyClassMember.
func (s *ClassService) JoinByCode(ctx context.Context, in JoinClassInput) (*models.Class, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		return nil, NewValidationError("código da turma é obrigatório")
	}

	roles, err := s.roleRepo.GetByUserID(ctx, in.StudentUserID)
	if err != nil {
		return nil, err
	}
	isStudent := false
	for _, r := range roles {
		if r.Name == studentRoleName {
			isStudent = true
			break
		}
	}
	if !isStudent {
		return nil, ErrNotStudent
	}

	class, err := s.classRepo.GetByJoinCode(ctx, code)
	if err != nil {
		return nil, err
	}

	added, err := s.classRepo.AddMember(ctx, class.ID, in.StudentUserID)
	if err != nil {
		return nil, err
	}
	if !added {
		return nil, ErrAlreadyClassMember
	}

	return class, nil
}

// ListByProfessor retorna as turmas criadas pelo professor, com contagem de alunos.
func (s *ClassService) ListByProfessor(ctx context.Context, professorUserID uuid.UUID) ([]models.ClassSummary, error) {
	return s.classRepo.ListByProfessor(ctx, professorUserID)
}

// ListByStudent retorna as turmas nas quais o aluno ingressou.
func (s *ClassService) ListByStudent(ctx context.Context, studentUserID uuid.UUID) ([]models.StudentClass, error) {
	return s.classRepo.ListByStudent(ctx, studentUserID)
}

func nullableDescription(d string) *string {
	d = strings.TrimSpace(d)
	if d == "" {
		return nil
	}
	return &d
}

func generateJoinCode() (string, error) {
	buf := make([]byte, joinCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("erro ao gerar código de turma: %w", err)
	}
	for i := range buf {
		buf[i] = joinCodeAlphabet[int(buf[i])%len(joinCodeAlphabet)]
	}
	return string(buf), nil
}
