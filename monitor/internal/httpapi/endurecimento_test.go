package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/seno-project/seno/monitor/internal/platform"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

// TestAutosaveDuasAbasConcorrente — Fase 8.3: escrevas com a MESMA revisão de
// duas abas ao vivo: exatamente uma vence (revisão 0→1) e a outra recebe
// ErrRevisao. Requer Postgres de teste (skipa sem).
func TestAutosaveDuasAbasConcorrente(t *testing.T) {
	pool := testeDB(t)
	deps, _ := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	if _, err := deps.TentativaS.Abrir(context.Background(), midTest, int64(subTest), time.Now()); err != nil {
		t.Fatalf("abrir: %v", err)
	}

	const n = 4
	type res struct {
		nova int
		err  error
	}
	ch := make(chan res, n)
	for i := 0; i < n; i++ {
		go func() {
			nova, err := deps.TentativaS.Salvar(context.Background(), midTest, int64(subTest),
				map[string]any{"1": "codigo"}, 0, time.Now())
			ch <- res{nova, err}
		}()
	}
	ganhas, perdidas := 0, 0
	for i := 0; i < n; i++ {
		r := <-ch
		switch {
		case r.err == nil && r.nova == 1:
			ganhas++
		case errors.Is(r.err, tentativas.ErrRevisao):
			perdidas++
		default:
			t.Fatalf("resultado inesperado: nova=%d err=%v", r.nova, r.err)
		}
	}
	if ganhas != 1 || perdidas != n-1 {
		t.Fatalf("concorrência: %d vencedoras e %d rejeitadas; quero 1 e %d", ganhas, perdidas, n-1)
	}
}

// TestLogAuditoria — Fase 8.2: entrada (login) e primeira abertura de
// tentativa ficam na tabela log.
func TestLogAuditoria(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaMonitorJudge0(t, pool)
	deps.Audit = platform.NewAudit(pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.9.0.9:9")
	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != 200 {
		t.Fatalf("abrir: %d %s", w.Code, w.Body.String())
	}

	for _, kk := range [][2]string{{"entrada", "ip:10.9.0.9"}, {"abrir", "aluno:12"}} {
		var nRows int
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM log WHERE tipo = $1 AND agente = $2`, kk[0], kk[1],
		).Scan(&nRows); err != nil {
			t.Fatalf("conta log %s: %v", kk[0], err)
		}
		if nRows == 0 {
			t.Fatalf("log de auditoria sem registro de %s (agente %s)", kk[0], kk[1])
		}
	}
}
