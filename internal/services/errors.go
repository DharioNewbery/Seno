package services

import "errors"

var (
	ErrInvalidCredentials      = errors.New("credenciais inválidas")
	ErrAccountLocked           = errors.New("conta bloqueada temporariamente")
	ErrTokenInvalid            = errors.New("token inválido ou expirado")
	ErrTokenRevoked            = errors.New("token de atualização revogado")
	ErrCurrentPasswordMismatch = errors.New("senha atual incorreta")
)

// ValidationError representa um erro de validação de entrada, mapeado para HTTP 400.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}
