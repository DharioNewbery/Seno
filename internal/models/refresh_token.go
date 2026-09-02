package models

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken armazena o hash de um refresh token emitido, permitindo revogação.
type RefreshToken struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	UserID    uuid.UUID  `db:"user_id" json:"user_id"`
	TokenHash string     `db:"token_hash" json:"-"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// IsRevoked indica se o token foi revogado.
func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

// IsExpired indica se o token está expirado.
func (t *RefreshToken) IsExpired() bool {
	return t.ExpiresAt.Before(time.Now())
}

// IsValid indica se o token ainda pode ser utilizado.
func (t *RefreshToken) IsValid() bool {
	return !t.IsRevoked() && !t.IsExpired()
}
