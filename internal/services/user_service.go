package services

import (
	"context"

	"github.com/google/uuid"

	"seno/internal/models"
)

type UserService struct {
	userRepo UserRepository
	roleRepo RoleRepository
}

func NewUserService(userRepo UserRepository, roleRepo RoleRepository) *UserService {
	return &UserService{userRepo: userRepo, roleRepo: roleRepo}
}

// List retorna todos os usuários ativos.
func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	return s.userRepo.List(ctx)
}

// GetByID retorna um usuário pelo seu identificador.
func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetRoles retorna os papéis atribuídos a um usuário.
func (s *UserService) GetRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	return s.roleRepo.GetByUserID(ctx, userID)
}
