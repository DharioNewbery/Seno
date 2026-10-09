package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/monitor/internal/platform"
)

// PGXPool é a interface mínima do pool usada pelos handlers (facilita testes).
type PGXPool interface {
	Ping(ctx context.Context) error
}

// Dependencies carrega os serviços compartilhados dos handlers. Rotas de
// aluno (Fase 4) e rotas internas HMAC (Fase 2) entram aqui nas fases seguintes.
type Dependencies struct {
	Cfg  *platform.Config
	Pool PGXPool
}

// RegisterRoutes registra as rotas correntes do Monitor.
func RegisterRoutes(r *gin.Engine, d *Dependencies) {
	r.GET("/healthz", d.Healthz)
}

// Healthz verifica integridade do processo e do banco (compose healthcheck).
func (d *Dependencies) Healthz(c *gin.Context) {
	ctx := c.Request.Context()
	if d.Pool != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := d.Pool.Ping(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "database": "down"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "up"})
}
