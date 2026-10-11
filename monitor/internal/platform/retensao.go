package platform

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Retensao aplica a regra LGPD (ARQUITETURA §9.2, Fase 8.1): submissões e
// logs retidos por SENO_RETENCAO_DADOS_D dias (default 2 anos); execucao
// (7 dias) é limpa na inserção. O ciclo mais antigo é apagado antes do
// filho (pre_avaliacao referencia submissao sem CASCADE).
func Limpar(retIntervalo time.Duration) func(context.Context, *pgxpool.Pool) {
	return func(ctx context.Context, pool *pgxpool.Pool) {
		for _, stmt := range []struct{ q, rotulo string }{
			{`DELETE FROM log WHERE ocorrida_em < now() - $1::interval`, "log"},
			{`DELETE FROM pre_avaliacao WHERE criada_em < now() - $1::interval`, "pre_avaliacao"},
			{`DELETE FROM submissao WHERE entregue_em < now() - $1::interval`, "submissao"},
		} {
			if _, err := pool.Exec(ctx, stmt.q, retIntervalo); err != nil {
				slog.Error("monitor: retenção de dados", "tabela", stmt.rotulo, "err", err)
			}
		}
	}
}

// LacaRetensao roda a limpeza em loop (a cada ciclo, começando agora).
func LacaRetensao(ctx context.Context, pool *pgxpool.Pool, ret, ciclo time.Duration) {
	limpar := Limpar(ret)
	go func() {
		c := time.NewTicker(ciclo)
		defer c.Stop()
		for {
			w, cancel := context.WithTimeout(ctx, 5*time.Minute)
			limpar(w, pool)
			cancel()
			select {
			case <-ctx.Done():
				return
			case <-c.C:
			}
		}
	}()
}
