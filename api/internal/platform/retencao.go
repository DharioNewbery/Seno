package platform

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResultadoRetencao sumariza as linhas removidas em uma rodada de limpeza LGPD.
type ResultadoRetencao struct {
	SubmissoesRemovidas    int64
	MonitorEstadoRemovidas int64
	LogEntriesRemovidas    int64
}

// LimparRetencao aplica a regra de retenção LGPD (ARQUITETURA §9.2):
// submissoes antigas (cascateando correções, correcao_tarefas e resultados),
// monitor_estado_aluno antigo e log_entries antigos com mais de retIntervalo.
// Registra a quantidade de linhas removidas no log.
func LimparRetencao(ctx context.Context, pool *pgxpool.Pool, retIntervalo time.Duration) (ResultadoRetencao, error) {
	var res ResultadoRetencao

	// 1. submissoes (correcoes, correcao_tarefas e resultados saem por ON DELETE CASCADE)
	tag, err := pool.Exec(ctx, `DELETE FROM submissoes WHERE entregue_em < now() - $1::interval`, retIntervalo)
	if err != nil {
		slog.Error("api: retenção de dados: submissoes", "err", err)
		return res, err
	}
	res.SubmissoesRemovidas = tag.RowsAffected()

	// 2. monitor_estado_aluno
	tag, err = pool.Exec(ctx, `DELETE FROM monitor_estado_aluno WHERE atualizado_em < now() - $1::interval`, retIntervalo)
	if err != nil {
		slog.Error("api: retenção de dados: monitor_estado_aluno", "err", err)
		return res, err
	}
	res.MonitorEstadoRemovidas = tag.RowsAffected()

	// 3. log_entries
	tag, err = pool.Exec(ctx, `DELETE FROM log_entries WHERE occurred_at < now() - $1::interval`, retIntervalo)
	if err != nil {
		slog.Error("api: retenção de dados: log_entries", "err", err)
		return res, err
	}
	res.LogEntriesRemovidas = tag.RowsAffected()

	slog.Info("api: limpeza de retenção LGPD executada",
		"retencao", retIntervalo.String(),
		"submissoes_removidas", res.SubmissoesRemovidas,
		"monitor_estado_removidas", res.MonitorEstadoRemovidas,
		"log_entries_removidas", res.LogEntriesRemovidas,
	)

	return res, nil
}

// LacaRetencao roda a limpeza em loop em segundo plano (a cada ciclo, começando após o primeiro tick).
func LacaRetencao(ctx context.Context, pool *pgxpool.Pool, retIntervalo, ciclo time.Duration) {
	go func() {
		t := time.NewTicker(ciclo)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w, cancel := context.WithTimeout(ctx, 5*time.Minute)
				_, _ = LimparRetencao(w, pool, retIntervalo)
				cancel()
			}
		}
	}()
}
