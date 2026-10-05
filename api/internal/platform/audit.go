package platform

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Audit grava eventos de escrita e login na tabela log_entries
// (PROJETO §Log, ARQUITETURA §9.3).
type Audit struct{ pool *pgxpool.Pool }

func NewAudit(pool *pgxpool.Pool) *Audit { return &Audit{pool: pool} }

// Registro do Log: data-hora, agente, tipo, descrição.
type LogEntry struct {
	ActorID *int64 // nulo para eventos de sistema
	Actor   string
	Kind    string
	Detail  string
}

func (a *Audit) Record(ctx context.Context, e LogEntry) {
	const q = `INSERT INTO log_entries (actor_id, actor, kind, detail) VALUES ($1, $2, $3, $4)`
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := a.pool.Exec(ctx, q, e.ActorID, e.Actor, e.Kind, e.Detail); err != nil {
		// O Log nunca derruba a requisição; falha é visível nos logs da API.
		slog.Error("falha ao gravar no log de auditoria", "err", err, "kind", e.Kind)
	}
}
