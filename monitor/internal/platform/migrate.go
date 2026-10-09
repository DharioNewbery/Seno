package platform

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// advisoryLockID é o identificador do lock de migração (arbitrário, fixo).
// Locks advisory são por banco; colisão com a API num servidor compartilhado
// só serializa migrações, nunca bloqueia operação normal.
const advisoryLockID = 918273645

// MigrateUp aplica todas as migrações pendentes sob advisory lock,
// garantindo exclusão múltipla entre réplicas. Cada migração é transacional;
// a ordem de aplicação é o nome do arquivo. root deve enraizar o diretório
// de migrações (ex.: db.Migrations).
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, root fs.FS) error {
	entries, err := fs.ReadDir(root, "migrations")
	if err != nil {
		return fmt.Errorf("fs de migrações inválido: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("sem conexão para migrações: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", advisoryLockID)

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version     TEXT PRIMARY KEY,
		applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("tabela schema_migrations: %w", err)
	}

	for _, name := range files {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&exists); err != nil {
			return fmt.Errorf("checando migração %s: %w", name, err)
		}
		if exists {
			continue
		}
		sqlBytes, err := fs.ReadFile(root, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("lendo migração %s: %w", name, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("tx da migração %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("aplicando migração %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("registrando migração %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit da migração %s: %w", name, err)
		}
		slog.Info("migração aplicada", "arquivo", name)
	}
	return nil
}
