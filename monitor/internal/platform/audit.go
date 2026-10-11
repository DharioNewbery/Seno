package platform

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Audit grava eventos de escrita e login na tabela log (ARQUITETURA §9.3,
// Fase 8.2 do plano). Duplicado da API de propósito (plataformas separadas).
type Audit struct{ pool *pgxpool.Pool }

func NewAudit(pool *pgxpool.Pool) *Audit { return &Audit{pool: pool} }

// Registro do Log: data-hora, agente, tipo, descrição.
type LogEntry struct {
	Agent     string
	Kind      string
	Descricao string
}

func (a *Audit) Record(ctx context.Context, e LogEntry) {
	if a == nil { // protege testes antigos sem audit
		return
	}
	const q = `INSERT INTO log (agente, tipo, descricao) VALUES ($1, $2, $3)`
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := a.pool.Exec(ctx, q, e.Agent, e.Kind, e.Descricao); err != nil {
		// O Log nunca derruba a requisição; falha é visível nos logs do Monitor.
		slog.Error("monitor: falha ao gravar no log de auditoria", "err", err, "kind", e.Kind)
	}
}
