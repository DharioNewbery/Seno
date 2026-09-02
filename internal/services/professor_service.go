package services

import (
	"context"
	"fmt"
	"strings"

	"seno/internal/models"
	"seno/internal/utils/password"
)

type ProfessorService struct {
	professorRepo ProfessorRepository
}

func NewProfessorService(professorRepo ProfessorRepository) *ProfessorService {
	return &ProfessorService{professorRepo: professorRepo}
}

type CreateProfessorInput struct {
	FullName string
	Email    string
	Password string
}

func (in CreateProfessorInput) validate() *ValidationError {
	if vErr := validateFullName(in.FullName); vErr != nil {
		return vErr
	}
	if vErr := validateEmail(in.Email); vErr != nil {
		return vErr
	}
	if vErr := validatePassword(in.Password); vErr != nil {
		return vErr
	}
	return nil
}

// CreateProfessor valida e cria a conta completa de professor de forma
// atômica. A senha informada é temporária: o professor deve trocá-la no
// primeiro acesso.
func (s *ProfessorService) CreateProfessor(ctx context.Context, in CreateProfessorInput) (*models.User, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	hashed, err := password.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("erro ao proteger senha: %w", err)
	}

	user := &models.User{
		FullName: strings.TrimSpace(in.FullName),
		Email:    strings.ToLower(strings.TrimSpace(in.Email)),
	}
	if err := s.professorRepo.CreateWithAccount(ctx, user, hashed); err != nil {
		return nil, err
	}
	return user, nil
}

// List retorna os professores ativos com seus dados públicos.
func (s *ProfessorService) List(ctx context.Context) ([]models.ProfessorUser, error) {
	return s.professorRepo.List(ctx)
}
