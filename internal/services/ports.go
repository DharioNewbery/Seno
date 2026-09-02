package services

import (
	"context"

	"github.com/google/uuid"

	"seno/internal/models"
	jwt "seno/internal/utils/jwt"
)

// UserRepository define o contrato de acesso a dados de usuários.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByLogin(ctx context.Context, login string) (*models.User, error)
	List(ctx context.Context) ([]models.User, error)
}

// CredentialRepository define o contrato de acesso a credenciais de login.
type CredentialRepository interface {
	Create(ctx context.Context, c *models.UserCredential) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.UserCredential, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
	IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) (int, error)
	ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error
}

// RoleRepository define o contrato de acesso a papéis e permissões.
type RoleRepository interface {
	Create(ctx context.Context, role *models.Role) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error)
	GetByName(ctx context.Context, name string) (*models.Role, error)
	List(ctx context.Context) ([]models.Role, error)
	AssignToUser(ctx context.Context, userID, roleID uuid.UUID) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.Role, error)
	HasAnyRole(ctx context.Context, userID uuid.UUID, roles []string) (bool, error)
	HasAnyPermission(ctx context.Context, userID uuid.UUID, permissions []string) (bool, error)
}

// RefreshTokenRepository define o contrato de acesso a refresh tokens.
type RefreshTokenRepository interface {
	Create(ctx context.Context, t *models.RefreshToken) error
	GetByTokenHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error
}

// ProfessorRepository define o contrato de acesso a dados de professores.
type ProfessorRepository interface {
	CreateWithAccount(ctx context.Context, user *models.User, passwordHash string) error
	List(ctx context.Context) ([]models.ProfessorUser, error)
}

// JWTManager define o contrato do emissor/validador de tokens.
type JWTManager interface {
	GenerateTokenPair(userID uuid.UUID, email string) (*jwt.TokenPair, error)
	Parse(tokenString string) (*jwt.Claims, error)
}
