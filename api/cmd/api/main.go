// Command api é o processo principal do Seno: carrega configuração,
// abre o PostgreSQL, aplica migrações e serve a API REST (/v1).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/db"
	"github.com/seno-project/seno/api/internal/analytics"
	"github.com/seno-project/seno/api/internal/atividades"
	"github.com/seno-project/seno/api/internal/atribuicoes"
	"github.com/seno-project/seno/api/internal/auth"
	"github.com/seno-project/seno/api/internal/correcoes"
	"github.com/seno-project/seno/api/internal/ensino"
	"github.com/seno-project/seno/api/internal/fila"
	"github.com/seno-project/seno/api/internal/httpapi"
	"github.com/seno-project/seno/api/internal/judge0"
	"github.com/seno-project/seno/api/internal/mail"
	"github.com/seno-project/seno/api/internal/monitor"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/submissoes"
	"github.com/seno-project/seno/api/internal/tarefas"
	"github.com/seno-project/seno/api/internal/tentativas"
	"github.com/seno-project/seno/api/internal/testar"
	"github.com/seno-project/seno/api/internal/usuarios"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// dev nativo: deploy/.env viram variáveis (containers injetam via compose;
	// o arquivo não existe na imagem — no-op lá).
	platform.LoadDotEnv()

	cfg, err := platform.LoadConfig()
	if err != nil {
		slog.Error("configuração inválida", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := platform.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("banco de dados indisponível", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := platform.MigrateUp(ctx, pool, db.Migrations); err != nil {
		slog.Error("falha nas migrações", "err", err)
		os.Exit(1)
	}
	slog.Info("migrações aplicadas")

	audit := platform.NewAudit(pool)
	st := store.New(pool)
	authSvc := auth.New(st, audit, cfg.SessionTTL, cfg.SessionMaxTTL, cfg.WebOrigin)
	recoverer := auth.NewRecoverer(st, audit, mail.Choose(cfg, audit), cfg.WebOrigin)

	if err := auth.BootstrapSuperAdmin(ctx, st, cfg.SuperEmail, cfg.SuperPassword); err != nil {
		slog.Error("bootstrap do super admin", "err", err)
		os.Exit(1)
	}

	svcTarefas := tarefas.New(st, audit)
	// Judge0 (§Execução do código): vazio = integração desligada. A base
	// do callback é a API como o Judge0 a alcança (interna) — nunca a
	// origem do navegador, que aponta para o BFF.
	judge0Cfg := judge0.CarregarConfigDasEnvLigada(
		cfg.Judge0URL, cfg.Judge0Token, cfg.Judge0Secret, cfg.Judge0CallbackURL,
	)
	judge0Cli := judge0.NewClient(judge0Cfg)
	monitorSvc := monitor.New(st, audit, cfg.MonitorURL, cfg.MonitorPublicURL, cfg.MonitorSecret)
	svcCorrecao := correcoes.New(st, audit, judge0Cli, judge0Cfg)
	deps := &httpapi.Dependencies{
		Cfg:         cfg,
		Pool:        pool,
		Store:       st,
		Audit:       audit,
		Auth:        authSvc,
		Cargo:       recoverer,
		UsuarioS:    usuarios.New(st, audit, recoverer),
		Ensino:      ensino.New(st, audit, recoverer),
		TarefaS:     svcTarefas,
		AtividadeS:  atividades.New(st, audit, svcTarefas),
		AtribuicaoS: atribuicoes.New(st, audit),
		TentativaS:  tentativas.New(st, audit),
		SubmissaoS:  submissoes.New(st, audit),
		CorrecaoS:   svcCorrecao,
		TestarS:     testar.New(st, audit, judge0Cli, judge0Cfg),
		Monitor:     monitorSvc,
		Analytics:   analytics.NewService(st),
	}

	if os.Getenv("SENO_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	httpapi.RegisterRoutes(r, deps)

	// Worker embutido da fila PostgreSQL (ARQUITETURA §6): transferência ao
	// Monitor (Fase 9.2) + poll do estado (Fase 9.3, ticker independente).
	fila.Start(ctx, st, map[string]fila.Handler{
		monitor.KindTransferir: monitorSvc.Handler(),
	}, 4)
	go monitorSvc.Ciclo(ctx, cfg.PollEstado)

	// Watchdog de correção (correcoes.FecharPerdidas): callback do Judge0
	// perdido → 'falhou' em vez de 'executando' eterno. Callback tardio
	// ainda conclui a correção — só o estado muda.
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ids, err := svcCorrecao.FecharPerdidas(ctx)
				if err != nil {
					slog.Warn("watchdog de correções", "err", err)
				} else if len(ids) > 0 {
					slog.Warn("correções marcadas 'falhou' sem callback do Judge0", "ids", ids)
				}
			}
		}
	}()

	// Retenção LGPD (ARQUITETURA §9.2): submissões, monitor_estado_aluno e log
	// por 2 anos (default); ciclo diário.
	platform.LacaRetencao(ctx, pool, cfg.DadosRetencao, 24*time.Hour)

	srv := &http.Server{Addr: cfg.Addr, Handler: r}
	go func() {
		slog.Info("api escutando", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("servidor encerrado", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("encerrando: sinal recebido")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownDeadline)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("encerramento com falhas", "err", err)
	}
}
