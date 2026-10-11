// Package fila implementa a fila de jobs PostgreSQL da API
// (ARQUITETURA §6): jobs na tabela `jobs`, worker embutido (pool de
// goroutines), dequeue FOR UPDATE SKIP LOCKED, 5 tentativas com backoff
// (1/5/15/60/60 min). Portado do Monitor; os status são os do schema da API
// (scheduled/running/done/failed) e o acesso passa pela camada sqlc (store).
package fila

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/store"
)

// Handler executa um job. Erro devolvido agenda retentativa com backoff.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Backoff das 5 tentativas (ARQUITETURA §6).
var backoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	60 * time.Minute,
	60 * time.Minute,
}

const maxTentativas = 5

// Agenda engata um job na fila. payload deve ser serializável.
func Agenda(ctx context.Context, st *store.Store, kind string, payload any, prioridade int16) (int64, error) {
	corpo, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("fila: payload de %s: %w", kind, err)
	}
	id, err := st.Q.InsertJob(ctx, gen.InsertJobParams{
		Kind:     kind,
		Payload:  corpo,
		Priority: prioridade,
		RunAfter: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}, // elegível já
	})
	if err != nil {
		return 0, fmt.Errorf("fila: agendando %s: %w", kind, err)
	}
	return id, nil
}

// Start requeue jobs presos ('running' de um boot anterior) e sobe o pool de
// workers. Não bloqueia: os loops terminam quando ctx é cancelado.
func Start(ctx context.Context, st *store.Store, handlers map[string]Handler, workers int) {
	_, err := st.Pool.Exec(ctx, `UPDATE jobs SET status = 'scheduled', updated_at = now()
	    WHERE status = 'running' AND updated_at < now() - interval '30 minutes'`)
	if err != nil {
		slog.Warn("fila: requeue em boot falhou", "err", err)
	}
	for i := 0; i < workers; i++ {
		go loop(ctx, st, handlers)
	}
}

func loop(ctx context.Context, st *store.Store, handlers map[string]Handler) {
	for {
		if ctx.Err() != nil {
			return
		}
		achou, err := um(ctx, st, handlers)
		if err != nil {
			slog.Warn("fila: worker", "err", err)
		}
		if !achou {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

// um pega um job e executa. Devolve true quando algo foi processado
// (DequeueJob: FOR UPDATE SKIP LOCKED, prioridade desc, FIFO por id).
func um(ctx context.Context, st *store.Store, handlers map[string]Handler) (bool, error) {
	ctxJob, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	job, err := st.Q.DequeueJob(ctxJob)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fila: dequeue: %w", err)
	}
	h, ok := handlers[job.Kind]
	if !ok {
		// kind sem handler é bug de código: falha definitiva imediata.
		motivo := "sem handler para kind " + job.Kind
		err := st.Q.MarkJobFailed(ctx, gen.MarkJobFailedParams{ID: job.ID, LastError: &motivo})
		return true, err
	}
	return true, concluir(ctx, st, job.ID, h(ctxJob, job.Payload), int(job.Attempts))
}

// concluir resolve o fim do job: concluído / retentativa com backoff / falha.
func concluir(ctx context.Context, st *store.Store, id int64, errJob error, attempts int) error {
	if errJob == nil {
		return st.Q.MarkJobDone(ctx, id)
	}
	motivo := errJob.Error()
	// Retentativa com backoff (ou falha definitiva na 5ª).
	if attempts+1 >= maxTentativas {
		return st.Q.MarkJobFailed(ctx, gen.MarkJobFailedParams{ID: id, LastError: &motivo})
	}
	return st.Q.ScheduleJobRetry(ctx, gen.ScheduleJobRetryParams{
		ID:        id,
		RunAfter:  pgtype.Timestamptz{Time: time.Now().UTC().Add(backoff[attempts]), Valid: true},
		LastError: &motivo,
	})
}
