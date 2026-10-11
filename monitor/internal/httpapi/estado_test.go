package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/seno-project/seno/contract"
)

// estadoHMAC assina (tempo+corpo) como a API faria no GET do estado.
func estadoHMAC(t *testing.T, r actorRouter, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	mac, tempo := contract.AssinarMAC([]byte(segTest), time.Now(), nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

func TestEstadoSemMAC(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/interna/v1/atividades/atv_x/estado", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("sem MAC: %d, quero 401", w.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if got := codigoErro(t, out); got != contract.CodeMACInvalido {
		t.Fatalf("código %q, quero MAC_INVALIDO", got)
	}
}

func TestEstadoMonitorInexistente(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	w, out := estadoHMAC(t, r, "/interna/v1/atividades/atv_sem_este/estado")
	if w.Code != http.StatusNotFound || codigoErro(t, out) != contract.CodeNaoEncontrado {
		t.Fatalf("monitor inexistente: %d %s", w.Code, w.Body.String())
	}
}

// TestEstadoConsolidado — reproduz os estados do aluno por SQL (sem Judge0):
//
//	12: entregou n_t=1, correção concluída nota 10 (tentativa apagada, §5.5)
//	13: tentativa aberta
//	99: ainda não abriu (estado "pendente", prazo da carga da janela)
func TestEstadoConsolidado(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", 12, 13, 99); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	sess12 := entrarCookie(t, r, 12, ipF+"10.1")
	if w, _ := chamaAluno(t, r, sess12, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir 12: %d %s", w.Code, w.Body.String())
	}
	sess13 := entrarCookie(t, r, 13, ipF+"10.2")
	if w, _ := chamaAluno(t, r, sess13, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir 13: %d", w.Code)
	}

	// entrega do 12 (vindo da Fase 6): submissão + correção concluída nota 10
	if _, err := pool.Exec(context.Background(), `INSERT INTO submissao
	    (monitor_id, aluno_api_id, n_t, dados, entregue_em, atrasada)
	    VALUES ($1, 12, 1, '{}'::jsonb, now(), false)`, midTest); err != nil {
		t.Fatalf("subm: %v", err)
	}
	var subID int64
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM submissao WHERE monitor_id = $1 AND aluno_api_id = 12`,
		midTest).Scan(&subID); err != nil {
		t.Fatalf("sub id: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO correcao
	    (submissao_id, monitor_id, aluno_api_id, status, nota_auto, concluida_em)
	    VALUES ($1, $2, 12, 'concluida', 10, now())`, subID, midTest); err != nil {
		t.Fatalf("corr: %v", err)
	}
	// §5.5 efeito 3: a entrega apaga a tentativa
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM tentativa WHERE monitor_id = $1 AND aluno_api_id = 12`, midTest); err != nil {
		t.Fatalf("tent del: %v", err)
	}

	w, _ := estadoHMAC(t, r, "/interna/v1/atividades/"+midTest+"/estado")
	if w.Code != http.StatusOK {
		t.Fatalf("estado: %d %s", w.Code, w.Body.String())
	}
	var est contract.EstadoAtividade
	if err := json.Unmarshal(w.Body.Bytes(), &est); err != nil {
		t.Fatalf("corpo: %v: %s", err, w.Body.String())
	}

	a := est.Agregados
	if a.Entregues != 1 || a.Abertas != 1 || a.Atrasos != 0 || a.Total != 3 {
		t.Fatalf("agregados: %+v", a)
	}
	if est.Status != "ativa" || len(est.Alunos) != 3 {
		t.Fatalf("status=%s alunos=%d", est.Status, len(est.Alunos))
	}
	slices.SortFunc(est.Alunos, func(x, y contract.EstadoAluno) int {
		switch {
		case x.AlunoAPIID < y.AlunoAPIID:
			return -1
		case x.AlunoAPIID == y.AlunoAPIID:
			return 0
		default:
			return 1
		}
	})

	a12 := est.Alunos[0]
	if a12.Estado != "entregue" || a12.NT != 1 || a12.UltimaSubmissao == nil ||
		a12.CorrecaoStatus != "concluida" || a12.NotaAuto == nil || *a12.NotaAuto != 10 {
		t.Fatalf("aluno 12: %+v", a12)
	}
	// entrega apaga a tentativa: prazo vem da janela da carga (fim fixo)
	if a12.PrazoEfetivo == nil {
		t.Fatalf("prazo 12 nulo (deveria vir da janela da carga)")
	}
	a13 := est.Alunos[1]
	if a13.Estado != "aberta" || a13.NT != 0 || a13.PrazoEfetivo == nil ||
		a13.NotaAuto != nil || a13.CorrecaoStatus != "pendente" {
		t.Fatalf("aluno 13: %+v", a13)
	}
	a99 := est.Alunos[2]
	if a99.Estado != "pendente" || a99.PrazoEfetivo == nil {
		t.Fatalf("aluno 99: %+v", a99)
	}
}
