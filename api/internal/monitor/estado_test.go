package monitor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/seno-project/seno/contract"
)

// TestPollEstado — Fase 9.3: GET assinado por atribuição snapshot e upsert em
// monitor_estado_aluno. Monitor-fake devolve o estado consolidado.
func TestPollEstado(t *testing.T) {
	st := testeDB(t)
	ctx := context.Background()
	atrID, alunoID := semeia(t, st, ctx)
	if _, err := st.Pool.Exec(ctx,
		`UPDATE atribuicoes SET monitor_id = 'atv_estado_1' WHERE id = $1`, atrID); err != nil {
		t.Fatal(err)
	}

	entrega := time.Now().Add(-7 * time.Minute)
	nota := 8.5
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxMAC := context.Background()
		_ = ctxMAC
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// MAC de GET: corpo vazio assinado — validado com o mesmo código do
		// contrato (o Monitor usa VerificarMAC no handler).
		if err := contract.VerificarMAC([]byte(segTeste), []byte(""),
			r.Header.Get(contract.HeaderMAC), r.Header.Get(contract.HeaderTime),
			time.Minute); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		est := contract.EstadoAtividade{
			MonitorID: "atv_estado_1",
			Status:    "ativa",
			Agregados: contract.AgregadosEstado{Total: 1, Entregues: 1},
			Alunos: []contract.EstadoAluno{{
				AlunoAPIID:      alunoID,
				Estado:          "entregue",
				NT:              1,
				UltimaSubmissao: &entrega,
				NotaAuto:        &nota,
				CorrecaoStatus:  "concluida",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(est)
	}))
	defer srv.Close()

	svc := New(st, nil, srv.URL, srv.URL, segTeste)
	if err := svc.PollEstado(ctx); err != nil {
		t.Fatalf("poll: %v", err)
	}
	var estado string
	var nt int
	var entregaDB *time.Time
	var notaDB float64
	if err := st.Pool.QueryRow(ctx, `
		SELECT estado, n_t, entregue_em, nota_auto::float8
		FROM monitor_estado_aluno WHERE atribuicao_id = $1 AND aluno_id = $2`,
		atrID, alunoID).Scan(&estado, &nt, &entregaDB, &notaDB); err != nil {
		t.Fatalf("estado não gravado: %v", err)
	}
	if estado != "entregue" || nt != 1 || entregaDB == nil || notaDB != nota {
		t.Fatalf("snapshot inesperado: %s n_t=%d entrega=%v nota=%v", estado, nt, entregaDB, notaDB)
	}

	// 404 (atividade removida do Monitor) → pula sem derrubar o ciclo.
	st2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer st2.Close()
	svc2 := New(st, nil, st2.URL, st2.URL, segTeste)
	if err := svc2.PollEstado(ctx); err != nil {
		t.Fatalf("poll com 404 não deveria falhar: %v", err)
	}
}
