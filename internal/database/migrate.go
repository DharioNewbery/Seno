package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies pending SQL migrations embedded in the binary.
// Migrations are tracked in the schema_migrations table and applied in
// lexicographic order, each inside its own transaction.
func Migrate(ctx context.Context, db *sqlx.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version     VARCHAR(255) PRIMARY KEY,
		applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("erro ao criar tabela de migrações: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("erro ao ler diretório de migrações: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		var applied bool
		if err := db.GetContext(ctx, &applied,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, f); err != nil {
			return fmt.Errorf("erro ao verificar status da migração %s: %w", f, err)
		}
		if applied {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("erro ao ler arquivo de migração %s: %w", f, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("erro ao iniciar transação para %s: %w", f, err)
		}

		for _, stmt := range splitStatements(string(content)) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("erro ao aplicar migração %s: %w", f, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, f); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("erro ao registrar migração %s: %w", f, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("erro ao confirmar migração %s: %w", f, err)
		}
	}

	return nil
}

// splitStatements divide um script SQL em comandos individuais, removendo
// comentários de linha (--). Suficiente para migrações DDL sem funções.
func splitStatements(sql string) []string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	cleaned := strings.Join(lines, "\n")

	var statements []string
	for _, part := range strings.Split(cleaned, ";") {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}
	return statements
}
