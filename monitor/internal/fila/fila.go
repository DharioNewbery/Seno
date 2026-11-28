// Package fila implementa a fila PostgreSQL do Monitor (mesmo desenho de
// ARQUITETURA §6): jobs na tabela `jobs`, worker embutido (pool de
// goroutines), dequeue FOR UPDATE SKIP LOCKED, 5 tentativas com backoff
// (1/5/15/60/60 min), heartbeat ativo de jobs em execução e recuperação
// periódica de jobs órfãos para evitar travamentos pós-crash.
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

// FailureHook é chamado quando um job esgota todas as tentativas e vai para 'falhou'.
type FailureHook func(ctx context.Context, kind string, payload json.RawMessage, err error)

// backoff das 5 tentativas (ARQUITETURA §6).
var backoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	60 * time.Minute,
	60 * time.Minute,
}

const (
	trabalhoMaxTentativas = 5
	heartbeatIntervalo    = 25 * time.Second
	janelaOrfao           = 2 * time.Minute
)

// Agenda engata um job na fila usando o pool. payload deve ser serializável.
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

// AgendaTx engata um job dentro de uma transação existente (Transactional Outbox),
// garantindo atomicidade entre a operação de negócio e o agendamento assíncrono.
func AgendaTx(ctx context.Context, tx pgx.Tx, kind string, payload any, prioridade int) (int64, error) {
	corpo, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("fila: payload tx: %w", err)
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO jobs (kind, payload, priority)
	    VALUES ($1, $2::jsonb, $3) RETURNING id`, kind, string(corpo), prioridade).Scan(&id); err != nil {
		return 0, fmt.Errorf("fila: agendando tx %s: %w", kind, err)
	}
	return id, nil
}

// Start recupera jobs presos e sobe o pool de workers.
func Start(ctx context.Context, pool *pgxpool.Pool, handlers map[string]Handler, workers int) {
	StartWithHooks(ctx, pool, handlers, workers, nil)
}

// StartWithHooks recupera jobs presos, inicia o sweeper periódico anti-travamento
// e sobe o pool de workers com callback para falhas definitivas.
func StartWithHooks(
	ctx context.Context,
	pool *pgxpool.Pool,
	handlers map[string]Handler,
	workers int,
	onFinalFailure FailureHook,
) {
	// Em boot de instância única do Monitor, recupera imediatamente jobs de
	// execuções interrompidas sem heartbeat recente.
	recuperarOrfaos(ctx, pool, 5*time.Second)

	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				recuperarOrfaos(ctx, pool, janelaOrfao)
			}
		}
	}()

	for i := 0; i < workers; i++ {
		go loop(ctx, pool, handlers, onFinalFailure)
	}
}

func recuperarOrfaos(ctx context.Context, pool *pgxpool.Pool, idade time.Duration) {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	interv := fmt.Sprintf("%fs", idade.Seconds())
	tag, err := pool.Exec(c, `UPDATE jobs
	    SET status = 'agendado', updated_at = now()
	    WHERE status = 'executando' AND updated_at < now() - $1::interval`, interv)
	if err != nil {
		slog.Warn("fila: recuperação de órfãos falhou", "err", err)
		return
	}
	if tag.RowsAffected() > 0 {
		slog.Info("fila: jobs órfãos reenfileirados", "qtd", tag.RowsAffected())
	}
}

func loop(ctx context.Context, pool *pgxpool.Pool, handlers map[string]Handler, onFinalFailure FailureHook) {
	for {
		if ctx.Err() != nil {
			return
		}
		achou, err := um(ctx, pool, handlers, onFinalFailure)
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

func um(ctx context.Context, pool *pgxpool.Pool, handlers map[string]Handler, onFinalFailure FailureHook) (bool, error) {
	ctxJob, cancel := context.WithTimeout(ctx, 10*time.Minute)
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
	if errors.Is(err, pgx.ErrNoRows) || ctx.Err() != nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fila: dequeue: %w", err)
	}
	h, ok := handlers[kind]
	if !ok {
		_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'falhou', last_error = $2, updated_at = now()
		    WHERE id = $1`, id, "sem handler para kind "+kind)
		return true, err
	}

	// Heartbeat mantém updated_at vivo enquanto o worker trabalha, permitindo
	// distinguir jobs ativos de jobs órfãos de um crash.
	hbCtx, hbStop := context.WithCancel(ctxJob)
	go func() {
		t := time.NewTicker(heartbeatIntervalo)
		defer t.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-t.C:
				_, _ = pool.Exec(hbCtx, `UPDATE jobs SET updated_at = now() WHERE id = $1 AND status = 'executando'`, id)
			}
		}
	}()

	errExec := h(ctxJob, payload)
	hbStop()

	// Usa contexto independente curto caso ctxJob tenha estourado timeout, garantindo a gravação do status final.
	finCtx, finCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finCancel()
	return true, concluir(finCtx, pool, id, kind, payload, errExec, attempts, onFinalFailure)
}

// concluir resolve o fim do job: falha final / retentativa / concluído.
func concluir(
	ctx context.Context,
	pool *pgxpool.Pool,
	id int64,
	kind string,
	payload json.RawMessage,
	errJob error,
	attempts int,
	onFinalFailure FailureHook,
) error {
	if errJob == nil {
		_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'concluido', updated_at = now()
		    WHERE id = $1`, id)
		return err
	}
	novo := attempts + 1
	if novo >= trabalhoMaxTentativas {
		_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'falhou', attempts = $3, last_error = $2, updated_at = now()
		    WHERE id = $1`, id, errJob.Error(), novo)
		if onFinalFailure != nil {
			onFinalFailure(ctx, kind, payload, errJob)
		}
		return err
	}
	idx := attempts
	if idx >= len(backoff) {
		idx = len(backoff) - 1
	}
	_, err := pool.Exec(ctx, `UPDATE jobs SET status = 'agendado', attempts = $3,
	    run_after = now() + $4::interval, last_error = $2, updated_at = now()
	    WHERE id = $1`, id, errJob.Error(), novo, backoff[idx].String())
	return err
}
