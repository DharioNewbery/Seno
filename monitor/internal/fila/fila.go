// Package fila implementa a fila PostgreSQL do Monitor (mesmo desenho de
// ARQUITETURA §6): jobs na tabela `jobs`, worker embutido (pool de
// goroutines), dequeue FOR UPDATE SKIP LOCKED, 5 tentativas com backoff
// (1/5/15/60 min), prioridade alta no "ensaiar" do aluno (hora da prova).
package fila

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler executa um job. Erro devolvido agenda retentativa.
type Handler func(ctx context.Context, payload json.RawMessage) error

// backoff dos 5 tentativas (ARQUITETURA §6).
var backoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	60 * time.Minute,
	60 * time.Minute,
}

const trabalhoMaxTentativas = 5

// Agenda engata um job na fila. payload deve ser serializável.
func Agenda(ctx context.Context, pool *pgxpool.Pool, kind string, payload any, prioridade int) (int64, error) {
	corpo, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("fila: payload: %w", err)
	}
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO jobs (kind, payload, priority)
	    VALUES ($1, $2::jsonb, $3) RETURNING id`, kind, string(corpo), prioridade).Scan(&id); err != nil {
		return 0, fmt.Errorf("fila: agendando %s: %w", kind, err)
	}
	return id, nil
}

// Start requeue jobs presos ('executando' de um boot anterior) e sobe o pool.
// Bloqueia? Não: devolve imediatamente, workers cortadas pelo ctx.
func Start(ctx context.Context, pool *pgxpool.Pool, handlers map[string]Handler, workers int) {
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'agendado', updated_at = now()
	    WHERE status = 'executando' AND run_after < now() - interval '30 minutes'`); err != nil {
		slog.Warn("fila: requeue em boot falhou", "err", err)
	}
	for i := 0; i < workers; i++ {
		go loop(ctx, pool, handlers)
	}
}

func loop(ctx context.Context, pool *pgxpool.Pool, handlers map[string]Handler) {
	for {
		if ctx.Err() != nil {
			return
		}
		achou, err := um(ctx, pool, handlers)
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

// um pega um job e executa. devolve true quando algo foi processado.
// Dequeue no mesmo desenho de api/db/queries/jobs.sql: UPDATE onde id =
// (SELECT ... FOR UPDATE SKIP LOCKED LIMIT 1).
func um(ctx context.Context, pool *pgxpool.Pool, handlers map[string]Handler) (bool, error) {
	ctxJob, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	var id int64
	var kind string
	var payload json.RawMessage
	var attempts int
	err := pool.QueryRow(ctxJob, `UPDATE jobs SET status = 'executando', updated_at = now()
	    WHERE id = (
	      SELECT id FROM jobs
	      WHERE status = 'agendado' AND run_after <= now()
	      ORDER BY priority DESC, id
	      FOR UPDATE SKIP LOCKED
	      LIMIT 1
	    )
	    RETURNING id, kind, payload, attempts`).Scan(&id, &kind, &payload, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fila: dequeue: %w", err)
	}
	h, ok := handlers[kind]
	if !ok {
		// kind sem handler é bug de código: falha definitiva imediata
		_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'falhou', last_error = $2, updated_at = now()
		    WHERE id = $1`, id, "sem handler para kind "+kind)
		return true, err
	}
	return true, concluir(ctxJob, pool, id, h(ctxJob, payload), attempts)
}

// concluir resolve o fim do job: falha final / retentativa / concluído.
func concluir(ctx context.Context, pool *pgxpool.Pool, id int64, errJob error, attempts int) error {
	if errJob == nil {
		_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'concluido', updated_at = now()
		    WHERE id = $1`, id)
		return err
	}
	// Retentativa com backoff ou falha definitiva.
	novo := attempts + 1
	if novo >= trabalhoMaxTentativas {
		_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'falhou', last_error = $2, updated_at = now()
		    WHERE id = $1`, id, errJob.Error())
		return err
	}
	_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'agendado', attempts = $3,
	    run_after = now() + $4::interval, last_error = $2, updated_at = now()
	    WHERE id = $1`, id, errJob.Error(), novo, backoff[attempts])
	return err
}
