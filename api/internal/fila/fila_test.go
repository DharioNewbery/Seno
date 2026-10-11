package fila

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/api/db"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// testeDB conecta ao Postgres de teste (CI ou dev local: banco seno_test),
// aplica migrações e limpa jobs antes do teste. Skipa sem banco.
func testeDB(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("SENO_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://seno:seno@localhost:5432/seno_test?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
	if _, err := pool.Exec(ctx, `TRUNCATE jobs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("limpeza: %v", err)
	}
	return store.New(pool)
}

func TestWorkerSucessoERetry(t *testing.T) {
	st := testeDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	feitas := make(chan json.RawMessage, 4)
	Start(ctx, st, map[string]Handler{
		"testa_ok": func(ctx context.Context, p json.RawMessage) error {
			feitas <- p
			return nil
		},
		"testa_erro": func(ctx context.Context, p json.RawMessage) error {
			return errors.New("cabo")
		},
	}, 2)

	// Sucesso: handler recebe o payload e o job termina 'done'.
	if _, err := Agenda(ctx, st, "testa_ok", map[string]any{"x": 1.0}, 0); err != nil {
		t.Fatalf("agenda ok: %v", err)
	}
	select {
	case p := <-feitas:
		var in map[string]any
		if err := json.Unmarshal(p, &in); err != nil || in["x"] != 1.0 {
			t.Fatalf("payload inesperado: %s", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("job de sucesso nunca rodou")
	}

	// Falha: 1ª tentativa agenda retry com backoff (+1 min), attempts=1.
	if _, err := Agenda(ctx, st, "testa_erro", nil, 0); err != nil {
		t.Fatalf("agenda erro: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var status string
	var attempts int16
	for time.Now().Before(deadline) {
		if err := st.Pool.QueryRow(ctx,
			`SELECT status, attempts FROM jobs WHERE kind = 'testa_erro'`,
		).Scan(&status, &attempts); err != nil {
			t.Fatal(err)
		}
		if status == "scheduled" && attempts == 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("retry não agendado: status=%s attempts=%d", status, attempts)
}
