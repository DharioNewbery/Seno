package models

import (
	"time"

	"github.com/google/uuid"
)

// User armazena os dados do usuário (sem dados sensíveis).
type User struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	FullName  string     `db:"full_name" json:"full_name"`
	Email     string     `db:"email" json:"email"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// UserCredential armazena as informações de login do usuário.
type UserCredential struct {
	ID                  uuid.UUID  `db:"id" json:"-"`
	UserID              uuid.UUID  `db:"user_id" json:"user_id"`
	PasswordHash        string     `db:"password_hash" json:"-"`
	LastLoginAt         *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	FailedLoginAttempts int        `db:"failed_login_attempts" json:"-"`
	LockedUntil         *time.Time `db:"locked_until" json:"-"`
	CreatedAt           time.Time  `db:"created_at" json:"-"`
	UpdatedAt           time.Time  `db:"updated_at" json:"-"`
}

// IsLocked indica se a credencial está bloqueada no momento.
func (c *UserCredential) IsLocked() bool {
	return c.LockedUntil != nil && c.LockedUntil.After(time.Now())
}
