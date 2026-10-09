package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/db"
	"github.com/seno-project/seno/monitor/internal/atividades"
	"github.com/seno-project/seno/monitor/internal/platform"
	"github.com/seno-project/seno/monitor/internal/sessao"
)

const segTest = "segredo-de-teste-para-o-monitor-32-bytes!!"

// testeDB conecta ao Postgres de teste (dev local ou CI: banco seno_monitor),
// aplica migrações e esvazia as tabelas antes de cada teste.
func testeDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SENO_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://seno:seno@localhost:5432/seno_monitor?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	if err := platform.MigrateUp(ctx, pool, db.Migrations); err != nil {
		t.Fatalf("migrações: %v", err)
	}
	_, err = pool.Exec(ctx, `TRUNCATE sso_consumo, pre_avaliacao, submissao,
		execucao, rascunho_ponto, tentativa, sessao, atividade_aluno, atividade
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("limpeza do banco: %v", err)
	}
	return pool
}

// montaMonitor constrói o roteador do Monitor (deps reais, config de teste).
func montaMonitor(t *testing.T, pool *pgxpool.Pool) (*Dependencies, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &platform.Config{
		Secret:            segTest,
		MaxPayloadBytes:   512 * 1024,
		TokenSSOTTL:       5 * time.Minute,
		HMACJanela:        time.Minute,
		SnapshotIntervalo: 60 * time.Second,
	}
	deps := &Dependencies{
		Cfg:        cfg,
		Pool:       pool,
		DB:         pool,
		AtividadeS: atividades.New(pool),
		SessaoS:    sessao.New(pool),
		Entradas:   sessao.NovoLimiter(10, 5*time.Minute),
	}
	r := gin.New()
	RegisterRoutes(r, deps)
	return deps, r
}

// enviarMAC assina o corpo como a API faria e valida o código HTTP esperado.
func enviarMAC(t *testing.T, r *gin.Engine, corpo []byte, statusEsperado int) map[string]any {
	t.Helper()
	mac, tempo := contract.AssinarMAC([]byte(segTest), time.Now(), corpo)
	req := httptest.NewRequest("POST", "/interna/v1/atividades", strings.NewReader(string(corpo)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != statusEsperado {
		t.Fatalf("status %d, quero %d: %s", w.Code, statusEsperado, w.Body.String())
	}
	var out map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("resposta ilegível: %v: %s", err, w.Body.String())
		}
	}
	return out
}

// transferenciaValida devolve o payload válido da spec §2 (jsonc) simpliado,
// com uma tarefa com teste público + privado (como na spec).
func transferenciaValida(t *testing.T) []byte {
	t.Helper()
	tt := contract.Transferencia{
		APIID:    42,
		GeradaEm: time.Now(),
		Turma:    contract.Turma{APIID: 7, Nome: "2026.1 Filas"},
		Alunos:   []contract.Aluno{{APIID: 12, Nome: "Ana R."}, {APIID: 13, Nome: "B. Lima"}},
		Janela:   contract.Janela{PodeAtrasado: true},
		Regras:   contract.Regras{MaxSubmissoes: 1, LinguagensPermitidas: []string{"python"}},
		Carga: contract.Carga{
			Atividade: contract.AtividadeCarga{APIID: 12, Nome: "Prova 1", Enunciado: "..."},
			Tarefas: []contract.TarefaCarga{{
				Ordem: 1, TarefaAPIID: 5, Nome: "Dobro", ValorPts: 10,
				Enunciado: "...", Linguagem: "python",
				Limites: &contract.Limites{TempoCPUMs: 2000, TempoTotalMs: 5000, MemoriaMB: 256},
				Testes: []contract.TesteCarga{
					{Stdin: "3", StdoutEsperado: "6", Publico: true},
					{Stdin: "", StdoutEsperado: "", Publico: false},
				},
			}},
		},
	}
	b, err := json.Marshal(tt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func codigoErro(t *testing.T, resp map[string]any) string {
	t.Helper()
	e, ok := resp["erro"].(map[string]any)
	if !ok {
		t.Fatalf("esperava envelope de erro, veio: %+v", resp)
	}
	return e["codigo"].(string)
}

// TestTransferenciaSemMAC — sem assinatura: 401 MAC_INVALIDO.
func TestTransferenciaSemMAC(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	req := httptest.NewRequest("POST", "/interna/v1/atividades", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, quero 401", w.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if got := codigoErro(t, out); got != contract.CodeMACInvalido {
		t.Fatalf("código %q, quero MAC_INVALIDO", got)
	}
}

// TestTransferenciaMACInvalido — MAC assinado com segredo errado: 401.
func TestTransferenciaMACInvalido(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	corpo := transferenciaValida(t)
	mac, tempo := contract.AssinarMAC([]byte("outro-segredo-alem-da-janela-32bytes!!"), time.Now(), corpo)
	req := httptest.NewRequest("POST", "/interna/v1/atividades", strings.NewReader(string(corpo)))
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, quero 401", w.Code)
	}
}

// TestTransferenciaIdempotente — reenvio com mesmo api_id: 200 e mesmo id.
func TestTransferenciaIdempotente(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	corpo := transferenciaValida(t)
	first := enviarMAC(t, r, corpo, http.StatusCreated)
	id1, ok := first["monitor_id"].(string)
	if !ok {
		t.Fatalf("monitor_id ausente: %+v", first)
	}
	second := enviarMAC(t, r, corpo, http.StatusOK)
	if second["monitor_id"] != id1 {
		t.Fatalf("monitor_id divergiu: %v vs %v", first["monitor_id"], second["monitor_id"])
	}
}

// TestTransferenciaAlunosVazio — "alunos vazio: 422" (spec §2).
func TestTransferenciaAlunosVazio(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	tt := contract.Transferencia{APIID: 99, GeradaEm: time.Now(),
		Turma: contract.Turma{APIID: 7, Nome: "T"}}
	b, _ := json.Marshal(tt)
	resp := enviarMAC(t, r, b, http.StatusUnprocessableEntity)
	if got := codigoErro(t, resp); got != contract.CodeValidacao {
		t.Fatalf("código %q, quero VALIDACAO", got)
	}
}

// TestTransferenciaSemAPIID — api_id zero: 422 VALIDACAO.
func TestTransferenciaSemAPIID(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	tt := contract.Transferencia{GeradaEm: time.Now(),
		Turma:  contract.Turma{APIID: 7, Nome: "T"},
		Alunos: []contract.Aluno{{APIID: 12, Nome: "A"}}}
	b, _ := json.Marshal(tt)
	resp := enviarMAC(t, r, b, http.StatusUnprocessableEntity)
	if got := codigoErro(t, resp); got != contract.CodeValidacao {
		t.Fatalf("código %q, quero VALIDACAO", got)
	}
}

// TestTransferenciaSemRegistno — asistente: recebida_em e alunos podem ser
// confirmados no banco (carga integral congelada, incluindo os privados).
func TestTransferenciaPersistencia(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	corpo := transferenciaValida(t)
	resp := enviarMAC(t, r, corpo, http.StatusCreated)
	id := resp["monitor_id"].(string)

	var status string
	var apiID int64
	var nAlunos, nTarefas int
	if err := pool.QueryRow(context.Background(),
		`SELECT status, api_id, (SELECT count(*) FROM atividade_aluno WHERE monitor_id = a.monitor_id),
		 (SELECT jsonb_array_length(carga->'carga'->'tarefas')) FROM atividade a WHERE monitor_id = $1`, id,
	).Scan(&status, &apiID, &nAlunos, &nTarefas); err != nil {
		t.Fatalf("consultando atividade: %v", err)
	}
	if status != "ativa" || apiID != 42 || nAlunos != 2 || nTarefas != 1 {
		t.Fatalf("estado inesperado: status=%s api_id=%d alunos=%d tarefas=%d", status, apiID, nAlunos, nTarefas)
	}
}
