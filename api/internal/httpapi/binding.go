package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
)

// Binding de JSON genérico; handlers de domínio reutilizam.
func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		platform.AbortWithError(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Corpo da requisição inválido."))
		return false
	}
	return true
}
