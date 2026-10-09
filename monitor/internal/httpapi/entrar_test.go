package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/sessao"
)

const (
	midTest = "atv_teste_sso"
	subTest = 12
)

// Marcadores exclusivos do fixture para o teste anti-vazamento: se o JSON
// de resposta de aluno contiver qualquer um, um privado escapou.
const (
	privStdin  = "PRIV-STDIN-secreto-Uai7Equ"
	privStdout = "PRIV-STDOUT-secreto-Ohsom3Es"
	pubStdin   = "3"
	pubStdout  = "6"
)

// semeiaAtividadeDB insere/reinsere a atividade de teste (envelope completo:
// janela+regras+carga com 1 tarefa, 1 teste público e 1 privado) com status
// dado e as matrículas do snapshot.
func semeiaAtividadeDB(pool *pgxpool.Pool, status string, alunos ...int64) error {
	tt := contract.Transferencia{
		APIID:    7,
		GeradaEm: time.Now(),
		Turma:    contract.Turma{APIID: 7, Nome: "2026.1 Filas"},
		Janela: contract.Janela{
			Inicio:       time.Now().Add(-5 * time.Minute),
			Prazo:        time.Now().Add(30 * time.Minute),
			DuracaoSeg:   intPtr(1800),
			PodeAtrasado: true,
		},
		Regras: contract.Regras{MaxSubmissoes: 1, LinguagensPermitidas: []string{"python"}},
		Carga: contract.Carga{
			Atividade: contract.AtividadeCarga{APIID: 12, Nome: "Prova 1", Enunciado: "Implemente o dobro."},
			Tarefas: []contract.TarefaCarga{{
				Ordem: 1, TarefaAPIID: 5, Nome: "Dobro", ValorPts: 10,
				Enunciado: "Leia um inteiro e imprima o dobro.", Linguagem: "python",
				Limites: &contract.Limites{TempoCPUMs: 2000, TempoTotalMs: 5000, MemoriaMB: 256},
				Testes: []contract.TesteCarga{
					{Stdin: pubStdin, StdoutEsperado: pubStdout, Publico: true},
					{Stdin: privStdin, StdoutEsperado: privStdout, Publico: false},
				},
			}},
		},
	}
	carga, err := json.Marshal(tt)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO atividade
	    (monitor_id, api_id, carga, recebida_em, status) VALUES ($1, $2, $3::jsonb, now(), $4)
	    ON CONFLICT (monitor_id) DO UPDATE SET status = excluded.status, carga = excluded.carga`,
		midTest, tt.APIID, string(carga), status); err != nil {
		return err
	}
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM atividade_aluno WHERE monitor_id = $1`, midTest); err != nil {
		return err
	}
	for _, a := range alunos {
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO atividade_aluno (monitor_id, aluno_api_id) VALUES ($1, $2)`,
			midTest, a); err != nil {
			return err
		}
	}
	return nil
}

func intPtr(i int) *int { return &i }

// semeiaAtividadeJanelaDB: variante com janela própria (testes de 409/410).
func semeiaAtividadeJanelaDB(pool *pgxpool.Pool, status string, j contract.Janela, alunos ...int64) error {
	tt := contract.Transferencia{
		APIID: 7, GeradaEm: time.Now(),
		Turma:  contract.Turma{APIID: 7, Nome: "T"},
		Janela: j,
		Regras: contract.Regras{MaxSubmissoes: 1},
	}
	carga, _ := json.Marshal(tt)
	if _, err := pool.Exec(context.Background(), `INSERT INTO atividade
	    (monitor_id, api_id, carga, recebida_em, status) VALUES ($1, $2, $3::jsonb, now(), $4)
	    ON CONFLICT (monitor_id) DO UPDATE SET status = excluded.status, carga = excluded.carga`,
		midTest, tt.APIID, string(carga), status); err != nil {
		return err
	}
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM atividade_aluno WHERE monitor_id = $1`, midTest); err != nil {
		return err
	}
	for _, a := range alunos {
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO atividade_aluno (monitor_id, aluno_api_id) VALUES ($1, $2)`,
			midTest, a); err != nil {
			return err
		}
	}
	return nil
}

// stDe assina um `st` de teste como a API faria (nonce rápido).
func stDe(t *testing.T, sub int64, mid string, ttl time.Duration) string {
	t.Helper()
	jti, err := contract.STNonce()
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	tok, err := contract.AssinarST([]byte(segTest), time.Now(), ttl,
		contract.SSOIss, contract.SSOAud, sub, mid, jti)
	if err != nil {
		t.Fatalf("AssinarST: %v", err)
	}
	return tok
}

// stAssinaCru assina um payload arbitrário de claims (para testes de exp).
func stAssinaCru(t *testing.T, payload []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(segTest))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		hex.EncodeToString(mac.Sum(nil))
}

func TestEntrarFluxoCompleto(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)

	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest), 101); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	// (a) Entrada válida: 200 + cookie HttpOnly + sessão gravada.
	tok := stDe(t, subTest, midTest, 5*time.Minute)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/entrar?st="+tok, nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, quero 200: %s", w.Code, w.Body.String())
	}
	cookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, sessao.CookieNome+"=") || !strings.Contains(cookie, "HttpOnly") {
		t.Fatalf("cookie hm_sess malformado: %q", cookie)
	}
	var ok map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ok); err != nil {
		t.Fatalf("corpo ilegível: %v", err)
	}
	if ok["sessao"] != "ok" || ok["monitor_id"] != midTest {
		t.Fatalf("corpo inesperado: %+v", ok)
	}

	// (b) Replay do mesmo st (jti consumido): 401.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/entrar?st="+tok, nil)
	req2.RemoteAddr = "10.0.0.1:1235"
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("replay: status %d, quero 401", w2.Code)
	}
}

func TestEntrarTokenCorrompido(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	tok := stDe(t, subTest, midTest, 5*time.Minute)
	corrompido := tok[:len(tok)-4] + "zzzz" // última quadra hex trocada

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/entrar?st="+corrompido, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, quero 401", w.Code)
	}
}

func TestEntrarExpirado(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	c := contract.STClaims{Alg: contract.SSOAlg, Iss: contract.SSOIss, Aud: contract.SSOAud,
		Sub: subTest, Mid: midTest, Exp: time.Now().Add(-time.Second).Unix(), Jti: 99}
	raw, _ := json.Marshal(c)
	tok := stAssinaCru(t, raw)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/entrar?st="+tok, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, quero 401 (expirado)", w.Code)
	}
}

func TestEntrarAlunoForaDoSnapshot(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	// 101 NÃO está no snapshot: 403 SEM_PERMISSAO.
	tok := stDe(t, 101, midTest, 5*time.Minute)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/entrar?st="+tok, nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, quero 403 (fora)", w.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if got := codigoErro(t, out); got != contract.CodeSemPermissao {
		t.Fatalf("código %q, quero SEM_PERMISSAO", got)
	}
}

func TestEntrarAtividadeEncerrada(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "encerrada", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	tok := stDe(t, subTest, midTest, 5*time.Minute)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/entrar?st="+tok, nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, quero 403 (encerrada)", w.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if got := codigoErro(t, out); got != contract.CodeFinalizada {
		t.Fatalf("código %q, quero FINALIZADA", got)
	}
}

func TestEntrarMidInexistente(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	tok := stDe(t, subTest, "atv_nao_existe", 5*time.Minute)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/entrar?st="+tok, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, quero 401 (mid inexistente)", w.Code)
	}
}

func TestEntrarRateLimit(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	// 10 falhas se esgotam; na 11ª, 429 (mesmo IP).
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/entrar?st=lixo", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("falha %d: status %d, quero 401", i+1, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/entrar?st=lixo", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, quero 429", w.Code)
	}
}

func TestRequireSessao(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	// Rota de sonda no grupo /a (com o middleware de sessão do RegisterRoutes).
	a := r.Group("/a", deps.RequireSessao)
	a.GET("/eco", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"aluno":   c.GetInt64("aluno_api_id"),
			"monitor": c.GetString("monitor_id"),
		})
	})

	// (a) sem cookie: 401.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/a/eco", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão: status %d, quero 401", w.Code)
	}

	// (b) entra e reutiliza o cookie na sonda.
	tok := stDe(t, subTest, midTest, 5*time.Minute)
	wn := httptest.NewRecorder()
	r.ServeHTTP(wn, httptest.NewRequest("GET", "/entrar?st="+tok, nil))
	if wn.Code != http.StatusOK {
		t.Fatalf("entrar: status %d: %s", wn.Code, wn.Body.String())
	}
	cookie := wn.Header().Get("Set-Cookie")
	raw := cookie[strings.Index(cookie, "=")+1:]
	raw = raw[:strings.Index(raw, ";")]

	we := httptest.NewRecorder()
	reqeco := httptest.NewRequest("GET", "/a/eco", nil)
	reqeco.Header.Set("Cookie", sessao.CookieNome+"="+raw)
	r.ServeHTTP(we, reqeco)
	if we.Code != http.StatusOK {
		t.Fatalf("eco: status %d: %s", we.Code, we.Body.String())
	}
	var eco map[string]any
	_ = json.Unmarshal(we.Body.Bytes(), &eco)
	if eco["aluno"].(float64) != subTest || eco["monitor"] != midTest {
		t.Fatalf("contexto do middleware inesperado: %+v", eco)
	}

	// (c) cookie forjado: 401.
	wf := httptest.NewRecorder()
	reqf := httptest.NewRequest("GET", "/a/eco", nil)
	reqf.Header.Set("Cookie",
		sessao.CookieNome+"=34bde8fb21d990b6f1989f4c95939af1")
	r.ServeHTTP(wf, reqf)
	if wf.Code != http.StatusUnauthorized {
		t.Fatalf("cookie forjado: status %d, quero 401", wf.Code)
	}
}
