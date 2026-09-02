package services

import "errors"

var (
	ErrInvalidCredentials      = errors.New("credenciais inválidas")
	ErrAccountLocked           = errors.New("conta bloqueada temporariamente")
	ErrTokenInvalid            = errors.New("token inválido ou expirado")
	ErrTokenRevoked            = errors.New("token de atualização revogado")
	ErrCurrentPasswordMismatch = errors.New("senha atual incorreta")
	ErrNotStudent              = errors.New("apenas alunos podem ingressar em turmas")
	ErrAlreadyClassMember      = errors.New("você já está nesta turma")
	ErrNotClassOwner           = errors.New("você não é o professor desta turma")
	ErrNotClassMember          = errors.New("você não é membro desta turma")
	ErrUnsupportedLanguage     = errors.New("linguagem não suportada")
)

// ValidationError representa um erro de validação de entrada, mapeado para HTTP 400.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}
