package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"seno/internal/models"
	"seno/internal/repositories"
	"seno/internal/utils/password"
)

// SuperRoleName é o papel de superusuário, criado pela migration 002.
const SuperRoleName = "super"

// SeedService provisiona dados obrigatórios do sistema no startup.
type SeedService struct {
	userRepo       UserRepository
	credentialRepo CredentialRepository
	roleRepo       RoleRepository
}

func NewSeedService(
	userRepo UserRepository,
	credentialRepo CredentialRepository,
	roleRepo RoleRepository,
) *SeedService {
	return &SeedService{
		userRepo:       userRepo,
		credentialRepo: credentialRepo,
		roleRepo:       roleRepo,
	}
}

type SuperUserInput struct {
	Login    string
	Email    string
	FullName string
	Password string
}

// EnsureSuperUser garante a existência do superusuário e da atribuição do
// papel super. É idempotente: se o usuário já existe, apenas reatribui o
// papel e não sobrescreve a senha (preserva trocas da senha temporária).
// Retorna true quando o usuário foi criado neste chamada.
func (s *SeedService) EnsureSuperUser(ctx context.Context, in SuperUserInput) (bool, error) {
	login := normalizeLogin(in.Login)

	role, err := s.roleRepo.GetByName(ctx, SuperRoleName)
	if err != nil {
		if errors.Is(err, repositories.ErrRoleNotFound) {
			return false, fmt.Errorf("papel %s não encontrado (migration 002 pendente): %w", SuperRoleName, err)
		}
		return false, err
	}

	existing, err := s.userRepo.GetByLogin(ctx, login)
	if err != nil {
		if !errors.Is(err, repositories.ErrUserNotFound) {
			return false, err
		}
	} else {
		if err := s.roleRepo.AssignToUser(ctx, existing.ID, role.ID); err != nil {
			return false, err
		}
		return false, nil
	}

	hashed, err := password.Hash(in.Password)
	if err != nil {
		return false, fmt.Errorf("erro ao proteger senha do superusuário: %w", err)
	}

	user := &models.User{
		FullName: strings.TrimSpace(in.FullName),
		Email:    strings.ToLower(strings.TrimSpace(in.Email)),
		Username: &login,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return false, err
	}

	credential := &models.UserCredential{UserID: user.ID, PasswordHash: hashed}
	if err := s.credentialRepo.Create(ctx, credential); err != nil {
		return false, err
	}

	if err := s.roleRepo.AssignToUser(ctx, user.ID, role.ID); err != nil {
		return false, err
	}

	return true, nil
}
