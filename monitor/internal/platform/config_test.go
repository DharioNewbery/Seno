package platform

import (
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef" // 32 bytes
	const dsn = "postgres://seno:seno@localhost:5432/seno_monitor"

	t.Run("valida mínimo obrigatório", func(t *testing.T) {
		t.Setenv("SENO_DATABASE_URL", dsn)
		t.Setenv("SENO_MONITOR_SECRET", secret)
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("não deveria errar: %v", err)
		}
		if cfg.Addr != ":8081" {
			t.Errorf("Addr default = %q, quero :8081", cfg.Addr)
		}
		if cfg.SnapshotIntervalo != 60*time.Second {
			t.Errorf("SnapshotIntervalo default = %v, quero 60s", cfg.SnapshotIntervalo)
		}
	})

	t.Run("sem DSN", func(t *testing.T) {
		t.Setenv("SENO_DATABASE_URL", "")
		t.Setenv("SENO_MONITOR_SECRET", secret)
		if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "SENO_DATABASE_URL") {
			t.Fatalf("deveria exigir SENO_DATABASE_URL, veio: %v", err)
		}
	})

	t.Run("segredo curto", func(t *testing.T) {
		t.Setenv("SENO_DATABASE_URL", dsn)
		t.Setenv("SENO_MONITOR_SECRET", "curto-demais")
		if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "SENO_MONITOR_SECRET") {
			t.Fatalf("deveria exigir segredo de 32 bytes, veio: %v", err)
		}
	})
}
