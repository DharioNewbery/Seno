package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"seno/internal/models"
	"seno/internal/repositories"
	"seno/internal/utils/hash"
	jwt "seno/internal/utils/jwt"
	"seno/internal/utils/password"
)

const (
	MaxFailedLoginAttempts = 5
	MinPasswordLength      = 8
)

type AuthService struct {
	userRepo       UserRepository
	credentialRepo CredentialRepository
	roleRepo       RoleRepository
	refreshRepo    RefreshTokenRepository
	studentRepo    StudentRepository
	jwt            JWTManager
}

func NewAuthService(
	userRepo UserRepository,
	credentialRepo CredentialRepository,
	roleRepo RoleRepository,
	refreshRepo RefreshTokenRepository,
	studentRepo StudentRepository,
	jwt JWTManager,
) *AuthService {
	return &AuthService{
		userRepo:       userRepo,
		credentialRepo: credentialRepo,
		roleRepo:       roleRepo,
		refreshRepo:    refreshRepo,
		studentRepo:    studentRepo,
		jwt:            jwt,
	}
}

type RegisterInput struct {
	FullName string
	Email    string
	Password string
}

type RegisterResult struct {
	User models.User
}

func (in RegisterInput) validate() *ValidationError {
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

// Register é o autocadastro público de aluno: cria usuário, credencial,
// vínculo em students e papel student em transação única. Professores são
// cadastrados pelo superusuário via /api/v1/professors.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*RegisterResult, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	if _, err := s.userRepo.GetByEmail(ctx, in.Email); err != nil {
		if !errors.Is(err, repositories.ErrUserNotFound) {
			return nil, err
		}
	} else {
		return nil, repositories.ErrUserAlreadyExists
	}

	hashed, err := password.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("erro ao proteger senha: %w", err)
	}

	user := &models.User{FullName: strings.TrimSpace(in.FullName), Email: strings.ToLower(strings.TrimSpace(in.Email))}
	if err := s.studentRepo.CreateWithAccount(ctx, user, hashed); err != nil {
		return nil, err
	}

	return &RegisterResult{User: *user}, nil
}

// LoginInput aceita email ou nome de usuário como identificador de login.
type LoginInput struct {
	Login    string
	Password string
}

type LoginResult struct {
	User   models.User
	Roles  []string
	Tokens *jwt.TokenPair
}

func (in LoginInput) validate() *ValidationError {
	login := normalizeLogin(in.Login)
	if login == "" {
		return NewValidationError("login é obrigatório")
	}
	if strings.Contains(login, "@") {
		if !isValidEmail(login) {
			return NewValidationError("email inválido")
		}
	} else if !isValidUsername(login) {
		return NewValidationError("nome de usuário deve ter entre 3 e 100 caracteres, sem espaços")
	}
	if in.Password == "" {
		return NewValidationError("senha é obrigatória")
	}
	return nil
}

func normalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

// Login valida as credenciais, controla tentativas falhas e emite o par de tokens.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	user, err := s.userRepo.GetByLogin(ctx, normalizeLogin(in.Login))
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	credential, err := s.credentialRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	if credential.IsLocked() {
		return nil, ErrAccountLocked
	}

	if err := password.Compare(credential.PasswordHash, in.Password); err != nil {
		attempts, _ := s.credentialRepo.IncrementFailedAttempts(ctx, user.ID)
		if attempts >= MaxFailedLoginAttempts {
			return nil, ErrAccountLocked
		}
		return nil, ErrInvalidCredentials
	}

	if err := s.credentialRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		return nil, err
	}

	tokens, err := s.jwt.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	refreshToken := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash.SHA256(tokens.RefreshToken),
		ExpiresAt: tokens.RefreshExpiresAt,
	}
	if err := s.refreshRepo.Create(ctx, refreshToken); err != nil {
		return nil, err
	}

	roles, err := s.roleRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(roles))
	for _, r := range roles {
		roleNames = append(roleNames, r.Name)
	}

	return &LoginResult{User: *user, Roles: roleNames, Tokens: tokens}, nil
}

type ChangePasswordInput struct {
	UserID          uuid.UUID
	CurrentPassword string
	NewPassword     string
}

func (in ChangePasswordInput) validate() *ValidationError {
	if in.CurrentPassword == "" {
		return NewValidationError("senha atual é obrigatória")
	}
	if vErr := validatePassword(in.NewPassword); vErr != nil {
		return vErr
	}
	return nil
}

// ChangePassword verifica a senha atual, define a nova e revoga todos os
// refresh tokens existentes (sessões de outros dispositivos perdem o acesso).
func (s *AuthService) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	if vErr := in.validate(); vErr != nil {
		return vErr
	}

	credential, err := s.credentialRepo.GetByUserID(ctx, in.UserID)
	if err != nil {
		return err
	}

	if err := password.Compare(credential.PasswordHash, in.CurrentPassword); err != nil {
		return ErrCurrentPasswordMismatch
	}

	hashed, err := password.Hash(in.NewPassword)
	if err != nil {
		return fmt.Errorf("erro ao proteger senha: %w", err)
	}

	if err := s.credentialRepo.UpdatePassword(ctx, in.UserID, hashed); err != nil {
		return err
	}

	return s.refreshRepo.RevokeAllByUserID(ctx, in.UserID)
}

// Refresh valida e rotaciona um refresh token, emitindo um novo par de tokens.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	claims, err := s.jwt.Parse(refreshToken)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	stored, err := s.refreshRepo.GetByTokenHash(ctx, hash.SHA256(refreshToken))
	if err != nil {
		if errors.Is(err, repositories.ErrRefreshTokenNotFound) {
			return nil, ErrTokenRevoked
		}
		return nil, err
	}
	if !stored.IsValid() {
		return nil, ErrTokenRevoked
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	if err := s.refreshRepo.Revoke(ctx, stored.ID); err != nil {
		return nil, err
	}

	tokens, err := s.jwt.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	newRefresh := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash.SHA256(tokens.RefreshToken),
		ExpiresAt: tokens.RefreshExpiresAt,
	}
	if err := s.refreshRepo.Create(ctx, newRefresh); err != nil {
		return nil, err
	}

	return tokens, nil
}

type MeResult struct {
	User  models.User
	Roles []models.Role
}

// GetMe retorna os dados do usuário autenticado e seus papéis.
func (s *AuthService) GetMe(ctx context.Context, userID uuid.UUID) (*MeResult, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, err := s.roleRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &MeResult{User: *user, Roles: roles}, nil
}
