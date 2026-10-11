package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/monitor/internal/atividades"
	"github.com/seno-project/seno/monitor/internal/ensaios"
	"github.com/seno-project/seno/monitor/internal/estado"
	"github.com/seno-project/seno/monitor/internal/platform"
	"github.com/seno-project/seno/monitor/internal/preavaliacao"
	"github.com/seno-project/seno/monitor/internal/sessao"
	"github.com/seno-project/seno/monitor/internal/submissoes"
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
	EnsaiosS   *ensaios.Service
	PreAvaS    *preavaliacao.Service
	SubmissaoS *submissoes.Service
	EstadoS    *estado.Service
	Entradas   *sessao.Limiter
}

// RegisterRoutes registra as rotas correntes do Monitor.
func RegisterRoutes(r *gin.Engine, d *Dependencies) {
	r.GET("/healthz", d.Healthz)
	r.POST("/interna/v1/atividades", d.ReceberAtividade)
	r.GET("/entrar", d.Entrar)
	r.GET("/interna/v1/atividades/:monitor_id/estado", d.EstadoAtividade)

	// rotas de aluno: sessão obrigatória + bloqueio final (cota esgotada)
	a := r.Group("/a", d.RequireSessao, d.BloquearFinalizado)
	a.POST("/atividade/:monitor_id/abrir", d.AbrirAtividade)
	a.PUT("/atividade/:monitor_id/tentativa", d.SalvarTentativa)
	a.POST("/atividade/:monitor_id/testar", d.Ensaio)
	a.GET("/atividade/:monitor_id/testar/:run_id", d.EnsaioResultado)
	a.POST("/atividade/:monitor_id/submissao/avaliar", d.PreAvaliar)
	a.POST("/atividade/:monitor_id/submissao/cancelar", d.CancelarSubmissao)
	a.POST("/atividade/:monitor_id/submissao/confirmar", d.ConfirmarSubmissao)

	// callback do Judge0 (autenticado por ?key=, não HMAC)
	r.PUT("/interna/callbacks/judge0", d.CallbackJudge0)
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
