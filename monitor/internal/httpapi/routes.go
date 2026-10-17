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
	Audit      *platform.Audit
	AtividadeS *atividades.Service
	SessaoS    *sessao.Service
	TentativaS *tentativas.Service
	EnsaiosS   *ensaios.Service
	PreAvaS    *preavaliacao.Service
	SubmissaoS *submissoes.Service
	EstadoS    *estado.Service
	Entradas   *sessao.Limiter
}

// Ligo o redirect do /entrar p/ a tela da prova (§AppServir) e as rotas
// estáticas da realização; middleware da rota de aluno valida sessão.
func RegisterRoutes(r *gin.Engine, d *Dependencies) {
	r.Use(d.limiteCorpo)
	r.GET("/healthz", d.Healthz)
	r.POST("/interna/v1/atividades", d.ReceberAtividade)
	r.GET("/entrar", d.Entrar)
	r.GET("/interna/v1/atividades/:monitor_id/estado", d.EstadoAtividade)
	r.POST("/interna/v1/atividades/:monitor_id/alunos/:aluno_id/reexecutar", d.ReexecutarCorrecao)
	r.DELETE("/interna/v1/atividades/:monitor_id/alunos/:aluno_id/submissao", d.ExcluirSubmissao)
	r.POST("/interna/v1/atividades/:monitor_id/alunos/:aluno_id/coletar", d.ColetarTentativa)


	// rotas de aluno: sessão obrigatória + bloqueio final + guarda de Origin
	// (as mutações /a são tudo POST/PUT — SameSite Lax + origem conhecida).
	a := r.Group("/a", d.RequireSessao, d.guardaOrigin, d.BloquearFinalizado)
	a.POST("/atividade/:monitor_id/abrir", d.AbrirAtividade)
	a.PUT("/atividade/:monitor_id/tentativa", d.SalvarTentativa)
	a.POST("/atividade/:monitor_id/testar", d.Ensaio)
	a.GET("/atividade/:monitor_id/testar/:run_id", d.EnsaioResultado)
	a.POST("/atividade/:monitor_id/submissao/avaliar", d.PreAvaliar)
	a.POST("/atividade/:monitor_id/submissao/cancelar", d.CancelarSubmissao)
	a.POST("/atividade/:monitor_id/submissao/confirmar", d.ConfirmarSubmissao)

	// tela da realização (assets embebidos; a página não tem sessão própria:
	// o app.js descobre a sessão chamando os endpoints /a, protegidos).
	// A página vive em /a/app (sem slash): hrefs relativos resolvem
	// /a/app.{css,js} — que também são registrados por serem os caminhos
	// naturais do navegador.
	r.GET("/a/app", d.AppServir("index.html"))
	r.GET("/a/app/", d.AppServir("index.html"))
	r.GET("/a/app/app.css", d.AppServir("app.css"))
	r.GET("/a/app/app.js", d.AppServir("app.js"))
	r.GET("/a/app.css", d.AppServir("app.css"))
	r.GET("/a/app.js", d.AppServir("app.js"))

	// callback do Judge0 (autenticado por ?key=, não HMAC)
	r.PUT("/interna/callbacks/judge0", d.CallbackJudge0)
}

// limiteCorpo é o teto GLOBAL de corpo (Fase 8.3: endurecimento): nenhum
// corpo passa do envelope de transferência (o maior corpo esperado). Handlers
// com limites menores (autosave) continuam com o deles, e o readBody local
// detecta o estouro como 413.
func (d *Dependencies) limiteCorpo(c *gin.Context) {
	if d.Cfg != nil && c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body,
			int64(d.Cfg.MaxPayloadBytes))
	}
	c.Next()
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
