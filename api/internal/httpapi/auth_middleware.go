package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
)

// Chaves do contexto Gin para a sessão autenticada desta requisição.
const (
	ctxUserKey  = "seno.user"
	ctxTokenKey = "seno.token"
)

// withUser guarda o usuário e o token opaco da sessão corrente.
func withUser(c *gin.Context, user domain.User, token string) {
	c.Set(ctxUserKey, user)
	c.Set(ctxTokenKey, token)
}

// currentUser devolve o usuário autenticado da requisição (setado por
// RequireAuth).
func currentUser(c *gin.Context) (domain.User, bool) {
	v, ok := c.Get(ctxUserKey)
	if !ok {
		return domain.User{}, false
	}
	user, ok := v.(domain.User)
	return user, ok
}

// currentToken devolve o token opaco da sessão corrente.
func currentToken(c *gin.Context) string {
	if v, ok := c.Get(ctxTokenKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// bearerToken extrai o token do header Authorization: Bearer <token>.
func bearerToken(c *gin.Context) string {
	const prefix = "Bearer "
	h := c.GetHeader("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// RequireAuth valida a sessão Bearer contra o banco em cada requisição
// (sessão + cargos, ARQUITETURA §4.2/§5.2), renovando a janela sliding.
// 401 SESSAO_INVALIDA quando ausente/inválida.
func (d *Dependencies) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			platform.AbortWithError(c, platform.NewAPIError(
				http.StatusUnauthorized, platform.CodeUnauthorized,
				"Sessão ausente."))
			return
		}
		user, err := d.Auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			platform.AbortWithError(c, err)
			return
		}
		withUser(c, user, token)
		c.Next()
	}
}

// RequireCargo exige ao menos um dos cargos indicados (ARQUITETURA §5.4).
// 403 SEM_PERMISSAO quando o usuário não os possui.
func (d *Dependencies) RequireCargo(wanted ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := currentUser(c)
		if !ok || !domain.HasAnyRole(user.Roles, wanted...) {
			platform.AbortWithError(c, platform.NewAPIError(
				http.StatusForbidden, platform.CodeForbidden,
				"Sem permissão para esta operação."))
			return
		}
		c.Next()
	}
}
