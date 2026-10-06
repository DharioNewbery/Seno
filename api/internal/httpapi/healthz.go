package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/auth"
	"github.com/seno-project/seno/api/internal/ensino"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/tarefas"
	"github.com/seno-project/seno/api/internal/usuarios"
)

// PGXPool é a interface mínima do pool usada pelos handlers (facilita testes).
type PGXPool interface {
	Ping(ctx context.Context) error
}

// Dependencies carrega os serviços compartilhados dos handlers.
type Dependencies struct {
	Cfg      *platform.Config
	Pool     PGXPool
	Store    *store.Store
	Audit    *platform.Audit
	Auth     *auth.Service
	Cargo    *auth.Recoverer
	UsuarioS *usuarios.Service
	Ensino   *ensino.Service
	TarefaS  *tarefas.Service
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
