// Command monitor é o processo principal do Monitor: carrega configuração,
// garante o banco próprio, aplica migrações e serve o serviço (/healthz por
// enquanto; rotas de aluno e vv nas fases 2–6 do plano).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/seno-project/seno/monitor/db"
	"github.com/seno-project/seno/monitor/internal/atividades"
	"github.com/seno-project/seno/monitor/internal/httpapi"
	"github.com/seno-project/seno/monitor/internal/platform"
	"github.com/seno-project/seno/monitor/internal/sessao"
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

	if err := ensureDatabase(ctx, cfg.DatabaseURL); err != nil {
		slog.Error("banco de dados indisponível", "err", err)
		os.Exit(1)
	}

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

	if os.Getenv("SENO_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	httpapi.RegisterRoutes(r, &httpapi.Dependencies{
		Cfg:        cfg,
		Pool:       pool,
		DB:         pool,
		AtividadeS: atividades.New(pool),
		SessaoS:    sessao.New(pool),
		Entradas:   sessao.NovoLimiter(10, 5*time.Minute),
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: r}
	go func() {
		slog.Info("monitor escutando", "addr", cfg.Addr)
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

// ensureDatabase garante que o banco do Monitor exista no servidor apontado
// pelo DSN (o Monitor mantém banco de dados próprio: seno_monitor). Conecta ao
// banco administrativo padrão do servidor e cria o banco-alvo se faltar.
// Necessário para volumes de dev já initializados antes do Monitor (o
// docker-entrypoint-initdb.d só roda em volume vazio).
func ensureDatabase(ctx context.Context, dsn string) error {
	adminCtx, adminCancel := context.WithTimeout(ctx, 10*time.Second)
	defer adminCancel()

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("DSN do banco inválido: %w", err)
	}
	if cfg.Database == "" {
		return errors.New("DSN sem banco: informe o banco do Monitor no DSN")
	}
	target := cfg.Database
	cfg.Database = "postgres"
	cfg.RuntimeParams = nil

	conn, err := pgx.ConnectConfig(adminCtx, cfg)
	if err != nil {
		return fmt.Errorf("conectando ao banco administrativo: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(adminCtx,
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", target,
	).Scan(&exists); err != nil {
		return fmt.Errorf("verificando banco %s: %w", target, err)
	}
	if exists {
		return nil
	}
	// Identificador do CREATE DATABASE não aceita parâmetro.
	if _, err := conn.Exec(adminCtx,
		fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{target}.Sanitize())); err != nil {
		return fmt.Errorf("criando banco %s: %w", target, err)
	}
	slog.Info("banco criado", "db", target)
	return nil
}
