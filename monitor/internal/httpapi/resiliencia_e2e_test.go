package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/judge0"
	"github.com/seno-project/seno/monitor/internal/preavaliacao"
	"github.com/seno-project/seno/monitor/internal/submissoes"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

// TestMonitorResilienciaJudge0EAPIOffline valida:
//  1. Com Judge0 E API fora do ar, o aluno consegue avaliar (recebendo aviso claro) e confirmar a entrega sem perder nada;
//  2. A submissão é persistida e os jobs de correção e sincronização com a API ficam agendados para execução posterior;
//  3. Quando Judge0 e API voltam ao ar, a reconciliação corrige a prova e entrega o pacote consolidado à API.
func TestMonitorResilienciaJudge0EAPIOffline(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	// Judge0 começa FORA DO AR (503) e depois passa a responder normalmente.
	var judge0Online atomic.Bool
	j0Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !judge0Online.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if req.URL.Path == "/system_info" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if req.URL.Path == "/submissions" && req.URL.Query().Get("wait") == "true" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"stdout":"6\n","time":"0.01","memory":3200,"status":{"id":3,"description":"Accepted"}}`))
			return
		}
		http.NotFound(w, req)
	}))
	defer j0Srv.Close()

	// API começa FORA DO AR (503) e depois passa a aceitar o pacote assinado.
	var apiOnline atomic.Bool
	var mu sync.Mutex
	var ultimoPacote *contract.PacoteSubmissaoMonitor
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !apiOnline.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if req.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if req.URL.Path == "/v1/interna/monitor/submissoes" {
			b, _ := io.ReadAll(req.Body)
			var p contract.PacoteSubmissaoMonitor
			if json.Unmarshal(b, &p) == nil {
				mu.Lock()
				ultimoPacote = &p
				mu.Unlock()
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, req)
	}))
	defer apiSrv.Close()

	j0Cli := judge0.NewClient(judge0.CarregarConfig(j0Srv.URL, ""))
	deps.PreAvaS = preavaliacao.New(pool, j0Cli, deps.TentativaS, 15*time.Minute)
	deps.SubmissaoS = submissoes.New(pool, deps.TentativaS, j0Cli).WithAPIPush(apiSrv.URL, segTest)

	sess := entrarCookie(t, r, subTest, "10.77.0.1:1")
	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir: %d", w.Code)
	}
	if w, _ := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{"1":"print(int(input())*2)"}}`); w.Code != http.StatusOK {
		t.Fatalf("autosave: %d", w.Code)
	}

	// 1. Pré-avaliação com Judge0 OFFLINE: retorna 200 com aviso claro e veredito avaliador_indisponivel (não bloqueia entrega!).
	w, preOut := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("avaliar com Judge0 offline falhou: %d %s", w.Code, w.Body.String())
	}
	if preOut["aviso_avaliador"] == nil || preOut["aviso_avaliador"] == "" {
		t.Fatalf("esperava aviso_avaliador preenchido quando Judge0 está offline: %+v", preOut)
	}
	preID := preOut["pre_id"].(string)

	// 2. Confirmação com Judge0 E API OFFLINE: submissão salva e jobs agendados na fila!
	w, confOut := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/confirmar",
		`{"pre_id":"`+preID+`"}`)
	if w.Code != http.StatusOK || confOut["status"] != "entregue" {
		t.Fatalf("confirmar com API/Judge0 offline falhou: %d %s", w.Code, w.Body.String())
	}

	var qtdNotificar int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM jobs WHERE kind = $1`, submissoes.KindNotificarAPI).Scan(&qtdNotificar); err != nil || qtdNotificar == 0 {
		t.Fatalf("esperava job %s agendado após falha da API; qtd=%d err=%v", submissoes.KindNotificarAPI, qtdNotificar, err)
	}

	// 3. Judge0 e API voltam ao ar: antecipa run_after dos retries agendados e roda reconciliação.
	judge0Online.Store(true)
	apiOnline.Store(true)
	_, _ = pool.Exec(context.Background(), `UPDATE jobs SET run_after = now() WHERE status IN ('agendado', 'falhou')`)
	deps.SubmissaoS.ReconciliarPendentes(context.Background())

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := ultimoPacote != nil && ultimoPacote.Correcao != nil && ultimoPacote.Correcao.Status == "concluida"
		mu.Unlock()
		if ok {
			break
		}
		_, _ = pool.Exec(context.Background(), `UPDATE jobs SET run_after = now() WHERE status = 'agendado'`)
		time.Sleep(150 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if ultimoPacote == nil || ultimoPacote.Correcao == nil || ultimoPacote.Correcao.Status != "concluida" {
		t.Fatalf("pacote consolidado não sincronizou após retorno do Judge0 e da API: %+v", ultimoPacote)
	}
}

// TestMonitorRecuperaCallbackPerdidoDoJudge0 valida que quando o Monitor estava
// fora do ar no momento do callback do Judge0, ReconciliarPendentes recupera o veredito pelo token.
func TestMonitorRecuperaCallbackPerdidoDoJudge0(t *testing.T) {
	pool := testeDB(t)
	deps, _ := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	j0Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/submissions/tok-perdido-1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"tok-perdido-1","stdout":"6\n","time":"0.012","memory":3100,"status":{"id":3,"description":"Accepted"}}`))
			return
		}
		http.NotFound(w, req)
	}))
	defer j0Srv.Close()

	j0Cli := judge0.NewClient(judge0.CarregarConfig(j0Srv.URL, ""))
	deps.EnsaiosS = newEnsaiosParaTeste(pool, j0Cli, deps.TentativaS)

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO execucao
		(run, monitor_id, aluno_api_id, tipo, tarefa_ordem, teste_ordem, publico, codigo, token, criada_em)
		VALUES ('run_cb_perdido', $1, $2, 'ensaio', 1, 1, true, 'print(6)', 'tok-perdido-1', now())`,
		midTest, int64(subTest)); err != nil {
		t.Fatalf("insert execucao pendente: %v", err)
	}

	deps.EnsaiosS.ReconciliarPendentes(ctx)

	var veredito *string
	if err := pool.QueryRow(ctx, `SELECT veredito FROM execucao WHERE run = 'run_cb_perdido' AND teste_ordem = 1`).Scan(&veredito); err != nil {
		t.Fatalf("query veredito: %v", err)
	}
	if veredito == nil || *veredito != "accepted" {
		t.Fatalf("callback perdido não foi recuperado: veredito=%v", veredito)
	}
}

// TestAutosave25AlunosSimultaneosSemDeadlock garante que 25 alunos salvando o
// primeiro snapshot simultaneamente (acima de MaxConns=20 do pool) não travam o Monitor.
func TestAutosave25AlunosSimultaneosSemDeadlock(t *testing.T) {
	pool := testeDB(t)
	tentS := tentativas.New(pool, time.Millisecond) // intervalo > 0 exercita o caminho completo de precisaSnapshot

	alunos := make([]int64, 25)
	for i := range alunos {
		alunos[i] = int64(500 + i)
	}
	if err := semeiaAtividadeDB(pool, "ativa", alunos...); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, al := range alunos {
		if _, err := tentS.Abrir(ctx, midTest, al, time.Now().Add(-time.Second)); err != nil {
			t.Fatalf("abrir %d: %v", al, err)
		}
	}

	errs := make(chan error, len(alunos))
	var wg sync.WaitGroup
	for _, al := range alunos {
		wg.Add(1)
		go func(alunoID int64) {
			defer wg.Done()
			_, err := tentS.Salvar(ctx, midTest, alunoID, map[string]any{"1": fmt.Sprintf("print(%d)", alunoID)}, 0, time.Now())
			errs <- err
		}(al)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("autosave concorrente falhou ou entrou em deadlock: %v", err)
		}
	}
}
