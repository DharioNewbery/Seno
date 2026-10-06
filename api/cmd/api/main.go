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

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/db"
	"github.com/seno-project/seno/api/internal/auth"
	"github.com/seno-project/seno/api/internal/ensino"
	"github.com/seno-project/seno/api/internal/httpapi"
	"github.com/seno-project/seno/api/internal/mail"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/tarefas"
	"github.com/seno-project/seno/api/internal/usuarios"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

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

	deps := &httpapi.Dependencies{
		Cfg:      cfg,
		Pool:     pool,
		Store:    st,
		Audit:    audit,
		Auth:     authSvc,
		Cargo:    recoverer,
		UsuarioS: usuarios.New(st, audit, recoverer),
		Ensino:   ensino.New(st, audit, recoverer),
		TarefaS:  tarefas.New(st, audit),
	}

	if os.Getenv("SENO_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	httpapi.RegisterRoutes(r, deps)

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
