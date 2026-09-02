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
	DefaultUserRole        = "user"
	MaxFailedLoginAttempts = 5
	MinPasswordLength      = 8
)

type AuthService struct {
	userRepo       UserRepository
	credentialRepo CredentialRepository
	roleRepo       RoleRepository
	refreshRepo    RefreshTokenRepository
	jwt            JWTManager
}

func NewAuthService(
	userRepo UserRepository,
	credentialRepo CredentialRepository,
	roleRepo RoleRepository,
	refreshRepo RefreshTokenRepository,
	jwt JWTManager,
) *AuthService {
	return &AuthService{
		userRepo:       userRepo,
		credentialRepo: credentialRepo,
		roleRepo:       roleRepo,
		refreshRepo:    refreshRepo,
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
	if strings.TrimSpace(in.FullName) == "" {
		return NewValidationError("nome completo é obrigatório")
	}
	if !isValidEmail(in.Email) {
		return NewValidationError("email inválido")
	}
	if len(in.Password) < MinPasswordLength {
		return NewValidationError(fmt.Sprintf("a senha deve ter no mínimo %d caracteres", MinPasswordLength))
	}
	return nil
}

// Register cria um novo usuário, sua credencial e atribui o papel padrão.
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
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	credential := &models.UserCredential{UserID: user.ID, PasswordHash: hashed}
	if err := s.credentialRepo.Create(ctx, credential); err != nil {
		return nil, err
	}

	role, err := s.roleRepo.GetByName(ctx, DefaultUserRole)
	if err != nil {
		return nil, err
	}
	if err := s.roleRepo.AssignToUser(ctx, user.ID, role.ID); err != nil {
		return nil, err
	}

	return &RegisterResult{User: *user}, nil
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	User   models.User
	Roles  []string
	Tokens *jwt.TokenPair
}

func (in LoginInput) validate() *ValidationError {
	if !isValidEmail(in.Email) {
		return NewValidationError("email inválido")
	}
	if in.Password == "" {
		return NewValidationError("senha é obrigatória")
	}
	return nil
}

// Login valida as credenciais, controla tentativas falhas e emite o par de tokens.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if vErr := in.validate(); vErr != nil {
		return nil, vErr
	}

	user, err := s.userRepo.GetByEmail(ctx, in.Email)
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

func isValidEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}
