package fila

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/testutil"
)

// testeDB conecta ao banco dedicado do pacote (evita colisão de TRUNCATE com httpapi em ./...).
func testeDB(t *testing.T) *store.Store {
	return testutil.TestDB(t, "seno_test_fila")
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
