// Package fila implementa a fila de jobs PostgreSQL da API
// (ARQUITETURA §6): jobs na tabela `jobs`, worker embutido (pool de
// goroutines), dequeue FOR UPDATE SKIP LOCKED, 5 tentativas com backoff
// (1/5/15/60/60 min), heartbeat ativo e sweeper periódico de órfãos para
// evitar jobs eternamente presos em 'running' após reinício/queda.
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

// FailureHook é invocado quando um job esgota todas as tentativas e é marcado como 'failed'.
type FailureHook func(ctx context.Context, kind string, payload json.RawMessage, err error)

// Backoff das 5 tentativas (ARQUITETURA §6).
var backoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	60 * time.Minute,
	60 * time.Minute,
}

const (
	maxTentativas      = 5
	heartbeatIntervalo = 25 * time.Second
	janelaOrfao        = 2 * time.Minute
)

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

// Start recupera jobs presos ('running' de um boot anterior) e sobe o pool de
// workers. Não bloqueia: os loops terminam quando ctx é cancelado.
func Start(ctx context.Context, st *store.Store, handlers map[string]Handler, workers int) {
	StartWithHooks(ctx, st, handlers, workers, nil)
}

// StartWithHooks recupera jobs presos, inicia o sweeper periódico de órfãos
// e sobe o pool de workers com callback de falha final.
func StartWithHooks(
	ctx context.Context,
	st *store.Store,
	handlers map[string]Handler,
	workers int,
	onFinalFailure FailureHook,
) {
	recuperarOrfaos(ctx, st, 5*time.Second)

	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				recuperarOrfaos(ctx, st, janelaOrfao)
			}
		}
	}()

	for i := 0; i < workers; i++ {
		go loop(ctx, st, handlers, onFinalFailure)
	}
}

func recuperarOrfaos(ctx context.Context, st *store.Store, idade time.Duration) {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	interv := fmt.Sprintf("%fs", idade.Seconds())
	tag, err := st.Pool.Exec(c, `UPDATE jobs SET status = 'scheduled', updated_at = now()
	    WHERE status = 'running' AND updated_at < now() - $1::interval`, interv)
	if err != nil {
		slog.Warn("fila: recuperação de órfãos falhou", "err", err)
		return
	}
	if tag.RowsAffected() > 0 {
		slog.Info("fila: jobs órfãos reenfileirados", "qtd", tag.RowsAffected())
	}
}

func loop(ctx context.Context, st *store.Store, handlers map[string]Handler, onFinalFailure FailureHook) {
	for {
		if ctx.Err() != nil {
			return
		}
		achou, err := um(ctx, st, handlers, onFinalFailure)
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
func um(ctx context.Context, st *store.Store, handlers map[string]Handler, onFinalFailure FailureHook) (bool, error) {
	ctxJob, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	job, err := st.Q.DequeueJob(ctxJob)
	if errors.Is(err, pgx.ErrNoRows) || ctx.Err() != nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fila: dequeue: %w", err)
	}
	h, ok := handlers[job.Kind]
	if !ok {
		motivo := "sem handler para kind " + job.Kind
		err := st.Q.MarkJobFailed(ctx, gen.MarkJobFailedParams{ID: job.ID, LastError: &motivo})
		return true, err
	}

	hbCtx, hbStop := context.WithCancel(ctxJob)
	go func() {
		t := time.NewTicker(heartbeatIntervalo)
		defer t.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-t.C:
				_, _ = st.Pool.Exec(hbCtx, `UPDATE jobs SET updated_at = now() WHERE id = $1 AND status = 'running'`, job.ID)
			}
		}
	}()

	errExec := h(ctxJob, job.Payload)
	hbStop()

	finCtx, finCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finCancel()
	return true, concluir(finCtx, st, job.ID, job.Kind, job.Payload, errExec, int(job.Attempts), onFinalFailure)
}

// concluir resolve o fim do job: concluído / retentativa com backoff / falha.
func concluir(
	ctx context.Context,
	st *store.Store,
	id int64,
	kind string,
	payload json.RawMessage,
	errJob error,
	attempts int,
	onFinalFailure FailureHook,
) error {
	if errJob == nil {
		return st.Q.MarkJobDone(ctx, id)
	}
	motivo := errJob.Error()
	if attempts+1 >= maxTentativas {
		err := st.Q.MarkJobFailed(ctx, gen.MarkJobFailedParams{ID: id, LastError: &motivo})
		if onFinalFailure != nil {
			onFinalFailure(ctx, kind, payload, errJob)
		}
		return err
	}
	idx := attempts
	if idx >= len(backoff) {
		idx = len(backoff) - 1
	}
	return st.Q.ScheduleJobRetry(ctx, gen.ScheduleJobRetryParams{
		ID:        id,
		RunAfter:  pgtype.Timestamptz{Time: time.Now().UTC().Add(backoff[idx]), Valid: true},
		LastError: &motivo,
	})
}
