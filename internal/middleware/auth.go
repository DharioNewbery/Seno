package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"seno/internal/utils/jwt"
	"seno/pkg/response"
)

type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyEmail  contextKey = "email"
)

// UserIDFromContext extrai o ID do usuário autenticado do contexto da requisição.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(uuid.UUID)
	return v, ok
}

// RequireAuth valida o token de acesso (Bearer) e popula o contexto da requisição.
func RequireAuth(mgr *jwt.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				response.JSON(w, http.StatusUnauthorized, response.Error("Token de acesso não fornecido"))
				return
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				response.JSON(w, http.StatusUnauthorized, response.Error("Formato de autorização inválido"))
				return
			}

			claims, err := mgr.Parse(parts[1])
			if err != nil {
				response.JSON(w, http.StatusUnauthorized, response.Error("Token inválido ou expirado"))
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RoleChecker abstrai a verificação de papéis/permissões para evitar acoplamento.
type RoleChecker interface {
	HasAnyRole(ctx context.Context, userID uuid.UUID, roles []string) (bool, error)
	HasAnyPermission(ctx context.Context, userID uuid.UUID, permissions []string) (bool, error)
}

// RequireRole garante que o usuário autenticado possua ao menos um dos papéis informados.
// Deve ser usado após RequireAuth.
func RequireRole(checker RoleChecker, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
				return
			}
			has, err := checker.HasAnyRole(r.Context(), userID, roles)
			if err != nil {
				response.JSON(w, http.StatusInternalServerError, response.Error("Erro ao verificar papéis"))
				return
			}
			if !has {
				response.JSON(w, http.StatusForbidden, response.Error("Acesso negado: papel insuficiente"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission garante que o usuário autenticado possua ao menos uma das permissões.
// Deve ser usado após RequireAuth.
func RequirePermission(checker RoleChecker, permissions ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
				return
			}
			has, err := checker.HasAnyPermission(r.Context(), userID, permissions)
			if err != nil {
				response.JSON(w, http.StatusInternalServerError, response.Error("Erro ao verificar permissões"))
				return
			}
			if !has {
				response.JSON(w, http.StatusForbidden, response.Error("Acesso negado: permissão insuficiente"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
