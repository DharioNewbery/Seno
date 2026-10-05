// Package domain carrega os tipos centrais compartilhados entre os
// serviços da API (usuários, cargos).
package domain

import "time"

// Role representa um cargo de usuário (PROJETO §Usuário).
type Role string

const (
	RoleStudent   Role = "student"
	RoleProfessor Role = "professor"
	RoleAdmin     Role = "admin"
	RoleSuper     Role = "super"
)

// AllRoles é o conjunto fechado de cargos válidos.
var AllRoles = []Role{RoleStudent, RoleProfessor, RoleAdmin, RoleSuper}

// IsValidRole verifica se o cargo existe.
func IsValidRole(r Role) bool {
	for _, valid := range AllRoles {
		if r == valid {
			return true
		}
	}
	return false
}

// HasAnyRole verifica se o usuário possui pelo menos um dos cargos.
func HasAnyRole(roles []Role, wanted ...Role) bool {
	for _, have := range roles {
		for _, w := range wanted {
			if have == w {
				return true
			}
		}
	}
	return false
}

// Status possíveis do usuário.
const (
	StatusActive   = "active"
	StatusPending  = "pending"
	StatusDisabled = "disabled"
)

// User é o usuário autenticável (pessoa + credenciais + cargos).
type User struct {
	ID         int64      `json:"id"`
	Email      string     `json:"email"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	Status     string     `json:"status"`
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	Roles      []Role     `json:"roles"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

// IsStaff informa se o usuário tem privilégios administrativos.
func (u User) IsStaff() bool {
	return HasAnyRole(u.Roles, RoleAdmin, RoleSuper)
}

// IsSuper informa se o usuário é super admin.
func (u User) IsSuper() bool {
	return HasAnyRole(u.Roles, RoleSuper)
}
