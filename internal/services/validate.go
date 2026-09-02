package services

import (
	"fmt"
	"strings"
)

// Validadores de entrada compartilhados entre serviços.

func validateFullName(name string) *ValidationError {
	if strings.TrimSpace(name) == "" {
		return NewValidationError("nome completo é obrigatório")
	}
	return nil
}

func validateEmail(email string) *ValidationError {
	if !isValidEmail(email) {
		return NewValidationError("email inválido")
	}
	return nil
}

func validatePassword(pwd string) *ValidationError {
	if len(pwd) < MinPasswordLength {
		return NewValidationError(fmt.Sprintf("a senha deve ter no mínimo %d caracteres", MinPasswordLength))
	}
	return nil
}

func isValidEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}

// isValidUsername verifica o formato básico de nome de usuário: 3 a 100
// caracteres, sem espaços. A comparação é case-insensitive (normalização
// para lowercase em normalizeLogin).
func isValidUsername(u string) bool {
	if len(u) < 3 || len(u) > 100 {
		return false
	}
	return !strings.ContainsAny(u, " \t\r\n")
}
