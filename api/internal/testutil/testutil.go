// Package testutil — helpers de teste compartilhados pelos pacotes da API.
// Cada pacote de teste usa banco PRÓPRIO (ex.: seno_test x seno_test_monitor):
// `go test ./...` roda os binários de teste em paralelo e TRUNCATEs no banco
// compartilhado colidem (Fase 9, lição da integração).
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/api/db"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// TestDB conecta ao Postgres de teste do pacote (default: banco de nome
// passado), cria o banco se faltar, aplica migrações e limpa as tabelas.
// Skipa quando o servidor não atende (dev local sem docker ou CI sem service).
func TestDB(t *testing.T, bancoPadrao string) *store.Store {
	t.Helper()
	dsn := os.Getenv("SENO_DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf("postgres://seno:seno@localhost:5432/%s?sslmode=disable", bancoPadrao)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if os.Getenv("SENO_DATABASE_URL") == "" {
		garanteBanco(ctx, dsn)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	if err := platform.MigrateUp(ctx, pool, db.Migrations); err != nil {
		t.Fatalf("migrações: %v", err)
	}
	_, err = pool.Exec(ctx, `TRUNCATE persons, users, user_roles, sessions,
		one_time_tokens, log_entries, jobs, tarefas, testes_tarefa,
		atividades, atividade_tarefas, atribuicoes,
		tentativas, submissoes, resultados, correcao_tarefas,
		correcoes, execucoes_teste, materias, periodos_letivos, turmas, matriculas,
		monitor_estado_aluno RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("limpeza: %v", err)
	}
	return store.New(pool)
}

// garanteBanco cria o banco de teste se ele não existir no servidor do DSN
// (POSTGRES_USER é superuser no container de dev/CI). Mesma mecânica do
// ensureDatabase do Monitor.
func garanteBanco(ctx context.Context, dsn string) {
	adminCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return // DSN customizado: banco já deve existir
	}
	if cfg.Database == "" {
		return
	}
	alvo := cfg.Database
	cfg.Database = "postgres"
	cfg.RuntimeParams = nil

	conn, err := pgx.ConnectConfig(adminCtx, cfg)
	if err != nil {
		return
	}
	defer conn.Close(adminCtx)

	var existe bool
	if err := conn.QueryRow(adminCtx,
		`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, alvo).Scan(&existe); err != nil || existe {
		return
	}
	_, _ = conn.Exec(adminCtx, fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{alvo}.Sanitize()))
}
