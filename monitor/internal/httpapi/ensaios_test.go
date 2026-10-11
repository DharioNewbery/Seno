package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/monitor/internal/atividades"
	"github.com/seno-project/seno/monitor/internal/ensaios"
	"github.com/seno-project/seno/monitor/internal/fila"
	"github.com/seno-project/seno/monitor/internal/judge0"
	"github.com/seno-project/seno/monitor/internal/platform"
	"github.com/seno-project/seno/monitor/internal/preavaliacao"
	"github.com/seno-project/seno/monitor/internal/sessao"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

const (
	callbackKeyTest = "callback-segredo-do-teste-judge0"
	ipF             = "10.9.0."
)

// montaMonitorJudge0 monta deps com Judge0 (se configurado) e um servidor
// de teste REAL em 0.0.0.0 (o Judge0 em Docker chama via host.docker.internal).
func montaMonitorJudge0(t *testing.T, pool *pgxpool.Pool) (*Dependencies, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &platform.Config{
		Secret:             segTest,
		MaxPayloadBytes:    512 * 1024,
		MaxSnapshotBytes:   4096,
		TokenSSOTTL:        5 * time.Minute,
		HMACJanela:         time.Minute,
		SnapshotIntervalo:  0,
		PreAvaTTL:          15 * time.Minute,
		AlunoExecMaxPerMin: 5,
		ExecucaoRetencao:   7 * 24 * time.Hour,
		Judge0URL:          os.Getenv("SENO_JUDGE0_URL"),
		Judge0Secret:       callbackKeyTest,
	}
	tentativaS := tentativas.New(pool, 0)
	j0 := judge0.NewClient(judge0.CarregarConfig(cfg.Judge0URL, ""))
	deps := &Dependencies{
		Cfg:        cfg,
		Pool:       pool,
		DB:         pool,
		AtividadeS: atividades.New(pool),
		SessaoS:    sessao.New(pool),
		TentativaS: tentativaS,
		Entradas:   sessao.NovoLimiter(50, 5*time.Minute),
	}
	r := gin.New()
	RegisterRoutes(r, deps)

	// Listener 0.0.0.0: o host.docker.internal do Docker não resolve para
	// Processos só em 127.0.0.1 com confiabilidade.
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: r}}
	srv.Start()
	t.Cleanup(srv.Close)
	porta := ln.Addr().(*net.TCPAddr).Port
	base := "http://host.docker.internal:" + strconv.Itoa(porta)

	deps.EnsaiosS = ensaios.New(pool, j0, tentativaS, base, callbackKeyTest, 5, 7*24*time.Hour)
	deps.PreAvaS = preavaliacao.New(pool, j0, tentativaS, cfg.PreAvaTTL)

	// fila real no teste (worker embutido); lookup corrente: deps apoteará
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	fila.Start(ctx, pool, map[string]fila.Handler{
		ensaios.KindEnsaio: func(ctx context.Context, payload json.RawMessage) error {
			return deps.EnsaiosS.Handler()(ctx, payload)
		},
	}, 4)
	return deps, r
}

// callbackBody é o corpo exato que o Judge0 entrega (probe ao vivo, §5.3).
const callbackBody = `{"stdout":"Ng==","time":"0.011","memory":"3496","stderr":null,
 "token":"manual","compile_output":null,"message":null,
 "status":{"id":4,"description":"Wrong Answer"}}`

// ---- Fila isolada ----------------------------------------------------------

func TestFilaConcluidoEKindDesconhecido(t *testing.T) {
	pool := testeDB(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	fila.Start(ctx, pool, map[string]fila.Handler{
		"tarefa_ok": func(ctx context.Context, payload json.RawMessage) error { return nil },
	}, 2)

	idOK, err := fila.Agenda(context.Background(), pool, "tarefa_ok", map[string]any{"x": 1}, 0)
	if err != nil {
		t.Fatalf("agenda ok: %v", err)
	}
	if _, err := fila.Agenda(context.Background(), pool, "kind_inexistente", map[string]any{}, 0); err != nil {
		t.Fatalf("agenda desconhecida: %v", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var sOK, sFalhou string
		err1 := pool.QueryRow(context.Background(),
			`SELECT status FROM jobs WHERE id = $1`, idOK).Scan(&sOK)
		err2 := pool.QueryRow(context.Background(),
			`SELECT status FROM jobs WHERE kind = 'kind_inexistente' ORDER BY id DESC LIMIT 1`).Scan(&sFalhou)
		if err1 == nil && err2 == nil && sOK == "concluido" && sFalhou == "falhou" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	var sOK string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM jobs WHERE id = $1`, idOK).Scan(&sOK); err != nil || sOK != "concluido" {
		t.Fatalf("job ok: status=%s err=%v, quero concluido", sOK, err)
	}
	var sFalhou string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM jobs WHERE kind = 'kind_inexistente' ORDER BY id DESC LIMIT 1`).Scan(&sFalhou); err != nil || sFalhou != "falhou" {
		t.Fatalf("kind inexistente: status=%s err=%v, quero falhou", sFalhou, err)
	}
}

// ---- Guardas do callback (sem Judge0) ---------------------------------------

func TestCallbackJudge0Guarda(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitorJudge0(t, pool)

	// key errada: 401
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/interna/callbacks/judge0?key=errado", strings.NewReader(`{}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("key errada: %d, quero 401", w.Code)
	}
	// key correta e corpo ilegível: 400
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/interna/callbacks/judge0?key="+callbackKeyTest,
		strings.NewReader("lixo")))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("corpo ilegível: %d, quero 400", w.Code)
	}
	// key correta, run inexistente: 200 silencioso (callback atrasado)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/interna/callbacks/judge0?key="+callbackKeyTest+"&run=inexistente&t=1",
		strings.NewReader(callbackBody)))
	if w.Code != http.StatusOK {
		t.Fatalf("run inexistente: %d, quero 200", w.Code)
	}
}

// ---- Callback manual (sem Judge0): registra na linha correta ----------------

func TestCallbackJudge0Manual(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, ipF+"4")
	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", w.Code, w.Body.String())
	}
	run, err := deps.EnsaiosS.Agendar(context.Background(), midTest, subTest, 1, "python", "print(1)", time.Now())
	if err != nil {
		t.Fatalf("agendar: %v", err)
	}

	// PUT com o corpo ao vivo do judge0 (código errado: espera 6, sai 6?? não:
	// o corpo do callbackBody representa "6" com veredito judge0:4). Marcado
	// como errado manualmente para exercitar a gravação do veredito.
	w, out := chamaAluno(t, r, sess, "PUT", "/interna/callbacks/judge0?key="+
		callbackKeyTest+"&run="+run+"&t=1", callbackBody)
	if w.Code != http.StatusOK || out["ok"] != true {
		t.Fatalf("callback: %d %s %+v", w.Code, w.Body.String(), out)
	}
	var veredito, stdoutObt *string
	if err := pool.QueryRow(context.Background(),
		`SELECT veredito, stdout_obtido FROM execucao WHERE run = $1 AND teste_ordem = 1`,
		run).Scan(&veredito, &stdoutObt); err != nil {
		t.Fatalf("row: %v", err)
	}
	if veredito == nil || *veredito != "judge0:4" {
		t.Fatalf("veredito %#v, quero judge0:4", veredito)
	}
	if stdoutObt == nil || strings.TrimSpace(*stdoutObt) != "6" {
		t.Fatalf("stdout %#v, quero '6'", stdoutObt)
	}
}

// ---- E2E real com Judge0 (roda em dev com a instância no ar) ----------------

func TestEnsaioPreAvaliacaoE2E(t *testing.T) {
	if os.Getenv("SENO_JUDGE0_URL") == "" {
		t.Skip("SENO_JUDGE0_URL não definido: integração desligada")
	}
	pool := testeDB(t)
	_, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest), 101); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, ipF+"2")

	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", w.Code, w.Body.String())
	}

	// ensaio com código errado: judge0:4 (wrong_answer) tirando stdout.
	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/testar",
		`{"ordem":1,"linguagem":"python","codigo":"print(int(input())-1)"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("testar: %d %s", w.Code, w.Body.String())
	}
	estado := esperaPronto(t, r, sess, out["run_id"].(string))
	if !estado.Pronto || len(estado.Resultados) != 1 || estado.Resultados[0].Veredito == "accepted" {
		t.Fatalf("código errado não devolve veredito: %+v", estado)
	}

	// ensaio com código certo: accepted + stdout "6\n".
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/testar",
		`{"ordem":1,"linguagem":"python","codigo":"print(int(input())*2)"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("testar 2: %d %s", w.Code, w.Body.String())
	}
	estado = esperaPronto(t, r, sess, out["run_id"].(string))
	if !estado.Pronto || estado.Resultados[0].Veredito != "accepted" {
		t.Fatalf("código certo não aceitou: %+v", estado)
	}
	if got := strings.TrimSpace(*estado.Resultados[0].StdoutObtido); got != pubStdout {
		t.Fatalf("stdout %q, quero %q", got, pubStdout)
	}

	// pré-avaliação com a resposta certa (autosave antes): relatório accepted.
	if w, _ := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{"1":"print(int(input())*2)"}}`); w.Code != http.StatusOK {
		t.Fatalf("autosave: %d %s", w.Code, w.Body.String())
	}
	w, outAv := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("avaliar: %d %s", w.Code, w.Body.String())
	}
	tarefas := outAv["tarefas"].([]any)
	if len(tarefas) != 1 || tarefas[0].(map[string]any)["veredito"] != "accepted" {
		t.Fatalf("relatório inesperado: %s", w.Body.String())
	}
}

func esperaPronto(t *testing.T, r actorRouter, sess, run string) *ensaios.EnsaioEstado {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		w, _ := chamaAluno(t, r, sess, "GET", "/a/atividade/"+midTest+"/testar/"+run, "")
		if w.Code == http.StatusOK {
			var e ensaios.EnsaioEstado
			if err := json.Unmarshal([]byte(w.Body.String()), &e); err == nil {
				return &e
			}
			t.Fatalf("corpo ilegível: %s", w.Body.String())
		}
		if w.Code != http.StatusAccepted {
			t.Fatalf("poll: %d: %s", w.Code, w.Body.String())
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("run não ficou pronto em 20 s")
	return nil
}

// rate limit: max configurado 2; 3º ensaio no minuto → 429 (spec §5.3).
func TestEnsaioRateLimitE2E(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaMonitorJudge0(t, pool)
	// troca o serviço por um com max 2 (callback fora, irrelevante aqui)
	deps.EnsaiosS = ensaios.New(pool, judge0.NewClient(judge0.CarregarConfig("", "")),
		tentativas.New(pool, 0), "", callbackKeyTest, 2, 7*24*time.Hour)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, ipF+"3")
	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir: %d", w.Code)
	}
	for i := 0; i < 2; i++ {
		w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/testar",
			`{"ordem":1,"linguagem":"python","codigo":"print(1)"}`)
		if w.Code != http.StatusAccepted {
			t.Fatalf("ensaio %d: %d %s", i+1, w.Code, w.Body.String())
		}
	}
	w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/testar",
		`{"ordem":1,"linguagem":"python","codigo":"print(2)"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("ensaio 3: %d, quero 429", w.Code)
	}
}
