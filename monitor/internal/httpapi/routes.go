package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/monitor/internal/atividades"
	"github.com/seno-project/seno/monitor/internal/platform"
	"github.com/seno-project/seno/monitor/internal/sessao"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

// PGXPool é a interface mínima do pool usada pelos handlers (facilita testes).
type PGXPool interface {
	Ping(ctx context.Context) error
}

// Dependencies carrega os serviços compartilhados dos handlers. Rotas de
// aluno (Fase 4) e rotas internas HMAC (Fase 2) entram aqui nas fases seguintes.
type Dependencies struct {
	Cfg        *platform.Config
	Pool       PGXPool
	DB         *pgxpool.Pool
	AtividadeS *atividades.Service
	SessaoS    *sessao.Service
	TentativaS *tentativas.Service
	Entradas   *sessao.Limiter
}

// RegisterRoutes registra as rotas correntes do Monitor.
func RegisterRoutes(r *gin.Engine, d *Dependencies) {
	r.GET("/healthz", d.Healthz)
	r.POST("/interna/v1/atividades", d.ReceberAtividade)
	r.GET("/entrar", d.Entrar)

	// rotas de aluno: protegidas por sessão de entrada
	a := r.Group("/a", d.RequireSessao)
	a.POST("/atividade/:monitor_id/abrir", d.AbrirAtividade)
	a.PUT("/atividade/:monitor_id/tentativa", d.SalvarTentativa)
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
