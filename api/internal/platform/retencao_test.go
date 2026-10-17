package platform_test

import (
	"context"
	"testing"
	"time"

	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/testutil"
)

func TestLimparRetencao(t *testing.T) {
	st := testutil.TestDB(t, "seno_test_retencao")
	ctx := context.Background()

	// Insere log_entry com data antiga (> 730 dias) e recente (< 730 dias)
	_, err := st.Pool.Exec(ctx, `
		INSERT INTO log_entries (occurred_at, actor, kind, detail)
		VALUES (now() - interval '800 days', 'test@seno.dev', 'login', 'antigo'),
		       (now() - interval '10 days', 'test@seno.dev', 'login', 'recente')
	`)
	if err != nil {
		t.Fatalf("setup log_entries: %v", err)
	}

	res, err := platform.LimparRetencao(ctx, st.Pool, 730*24*time.Hour)
	if err != nil {
		t.Fatalf("LimparRetencao: %v", err)
	}

	if res.LogEntriesRemovidas != 1 {
		t.Errorf("esperava 1 log_entry removida, obteve %d", res.LogEntriesRemovidas)
	}

	var count int
	err = st.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM log_entries`).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("esperava 1 log_entry restante, obteve %d", count)
	}
}
