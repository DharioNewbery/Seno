package handlers

import (
	"errors"
	"net/http"

	"seno/internal/repositories"
	"seno/internal/services"
)

// mapError traduz um erro de domínio em status HTTP e mensagem.
func mapError(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}

	var vErr *services.ValidationError
	if errors.As(err, &vErr) {
		return http.StatusBadRequest, vErr.Message
	}

	switch {
	case errors.Is(err, services.ErrCurrentPasswordMismatch):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, repositories.ErrUserNotFound),
		errors.Is(err, repositories.ErrCredentialNotFound),
		errors.Is(err, repositories.ErrRoleNotFound),
		errors.Is(err, repositories.ErrPermissionNotFound),
		errors.Is(err, repositories.ErrClassNotFound),
		errors.Is(err, repositories.ErrAssignmentNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, repositories.ErrUserAlreadyExists),
		errors.Is(err, services.ErrAlreadyClassMember):
		return http.StatusConflict, err.Error()
	case errors.Is(err, services.ErrNotStudent),
		errors.Is(err, services.ErrNotClassOwner),
		errors.Is(err, services.ErrNotClassMember),
		errors.Is(err, services.ErrUnsupportedLanguage):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, repositories.ErrRefreshTokenNotFound),
		errors.Is(err, services.ErrInvalidCredentials),
		errors.Is(err, services.ErrTokenInvalid),
		errors.Is(err, services.ErrTokenRevoked):
		return http.StatusUnauthorized, err.Error()
	case errors.Is(err, services.ErrAccountLocked):
		return http.StatusLocked, err.Error()
	}

	return http.StatusInternalServerError, "erro interno do servidor"
}
