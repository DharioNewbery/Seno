// Package worker processa submissões pendentes em segundo plano,
// acionando a correção automática em intervalos regulares.
package worker

import (
	"context"
	"log"
	"time"

	"seno/internal/services"
)

// evalTimeout limita o tempo total de correção de uma submissão
// (compilações + todos os casos de teste).
const evalTimeout = 5 * time.Minute

type GradingWorker struct {
	grading   *services.GradingService
	interval  time.Duration
	batchSize int
}

func NewGradingWorker(grading *services.GradingService, interval time.Duration, batchSize int) *GradingWorker {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if batchSize < 1 {
		batchSize = 1
	}
	return &GradingWorker{grading: grading, interval: interval, batchSize: batchSize}
}

// Start executa o ciclo de correção até o contexto ser cancelado.
func (w *GradingWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("Worker de correção automática iniciado (intervalo: %s, lote: %d)",
		w.interval, w.batchSize)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *GradingWorker) processBatch(ctx context.Context) {
	submissions, err := w.grading.ListPending(ctx, w.batchSize)
	if err != nil {
		log.Printf("Erro ao buscar submissões pendentes: %v", err)
		return
	}

	for _, s := range submissions {
		evalCtx, cancel := context.WithTimeout(ctx, evalTimeout)
		err := w.grading.Evaluate(evalCtx, s)
		cancel()
		if err != nil {
			// Infraestrutura indisponível: a submissão segue pending
			// e será tentada novamente no próximo ciclo.
			log.Printf("Erro ao corrigir submissão %s (nova tentativa no próximo ciclo): %v", s.ID, err)
		}
	}
}
