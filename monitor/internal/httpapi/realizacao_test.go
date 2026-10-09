package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/sessao"
)

// entrarCookie emite o st do aluno, faz a entrada e devolve o cookie cru.
func entrarCookie(t *testing.T, r actorRouter, sub int64, ip string) string {
	t.Helper()
	tok := stDe(t, sub, midTest, 5*time.Minute)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/entrar?st="+tok, nil)
	req.RemoteAddr = ip
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("entrar sub=%d: status %d: %s", sub, w.Code, w.Body.String())
	}
	cookie := w.Header().Get("Set-Cookie")
	raw := cookie[strings.Index(cookie, "=")+1:]
	return raw[:strings.Index(raw, ";")]
}

type actorRouter interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// chamaAluno executa uma requisição de aluno com o cookie da sessão.
func chamaAluno(t *testing.T, r actorRouter, sess, method, path, corpo string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	if sess != "" {
		req.Header.Set("Cookie", sessao.CookieNome+"="+sess)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

func TestRealizacaoFluxoCompleto(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest), 101); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.1.0.1:1")

	// (a) abrir: sanitizada com 1 teste público, tentativa zerada.
	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusOK {
		t.Fatalf("abrir: %d: %s", w.Code, w.Body.String())
	}
	ativ := out["atividade"].(map[string]any)
	if len(ativ["tarefas"].([]any)) != 1 {
		t.Fatalf("tarefas inesperadas: %+v", ativ)
	}
	tent := out["tentativa"].(map[string]any)
	if tent["revisao"].(float64) != 0 || tent["segundos_restantes"].(float64) <= 0 {
		t.Fatalf("tentativa inesperada: %+v", tent)
	}

	// (b) abrir de novo: idempotente (mesma revisão, sem nova tentativa).
	w2, out2 := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w2.Code != http.StatusOK || out2["tentativa"].(map[string]any)["revisao"].(float64) != 0 {
		t.Fatalf("idempotência quebrou: %d %s", w2.Code, w2.Body.String())
	}

	// (c) autosave: revisão 0 → 1, com snapshot da gravação anterior.
	w3, out3 := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{"1":"codigo do aluno"}}`)
	if w3.Code != http.StatusOK || out3["revisao"].(float64) != 1 {
		t.Fatalf("autosave: %d %s", w3.Code, w3.Body.String())
	}

	// (d) autosave com revisão antiga: 409 REVISAO_OBSOLETA + revisao_atual.
	w4, out4 := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{"1":"outra aba"}}`)
	if w4.Code != http.StatusConflict {
		t.Fatalf("revisão divergente: %d %s", w4.Code, w4.Body.String())
	}
	if got := codigoErro(t, out4); got != contract.CodeRevisaoObsoleta {
		t.Fatalf("código %q, quero REVISAO_OBSOLETA", got)
	}
	if det := out4["erro"].(map[string]any)["detalhe"].(map[string]any); det["revisao_atual"].(float64) != 1 {
		t.Fatalf("detalhe esperado revisao_atual=1: %+v", det)
	}

	// (e) snapshot registrado (intervalo 0 no cfg de teste → cada gravação).
	var nPontos int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM rascunho_ponto`).Scan(&nPontos); err != nil {
		t.Fatalf("pontos: %v", err)
	}
	if nPontos != 1 {
		t.Fatalf("rascunho_ponto = %d, quero 1 (estado anterior ao primeiro save: '{}')", nPontos)
	}
}

// TestVazamentoTestePrivado — resposta do abrir JAMAIS contém dados privados.
func TestVazamentoTestePrivado(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.2.0.1:1")

	w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	corpo := w.Body.String()
	if strings.Contains(corpo, privStdin) || strings.Contains(corpo, privStdout) {
		t.Fatalf("vazamento de teste privado na resposta: %s", corpo)
	}
	if !strings.Contains(corpo, pubStdout) {
		t.Fatalf("teste público desapareceu da resposta: %s", corpo)
	}
}

// TestAbrirInicioFuturo — 409 INICIO + segundos_para_inicio.
func TestAbrirInicioFuturo(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeJanelaDB(pool, "ativa", contract.Janela{
		Inicio:       time.Now().Add(30 * time.Minute),
		Prazo:        time.Now().Add(time.Hour),
		DuracaoSeg:   intPtr(1800),
		PodeAtrasado: true,
	}, int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.3.0.1:1")

	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, quero 409", w.Code)
	}
	if got := codigoErro(t, out); got != contract.CodeInicioFuturo {
		t.Fatalf("código %q, quero INICIO", got)
	}
	if det := out["erro"].(map[string]any)["detalhe"].(map[string]any); det["segundos_para_inicio"].(float64) <= 0 {
		t.Fatalf("detalhe inesperado: %+v", det)
	}
}

// TestAbrirEncerradaSemAtraso — prazo passado e pode_atrasado=false: 410.
func TestAbrirEncerradaSemAtraso(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeJanelaDB(pool, "ativa", contract.Janela{
		Inicio:       time.Now().Add(-time.Hour),
		Prazo:        time.Now().Add(-10 * time.Minute),
		DuracaoSeg:   intPtr(3600),
		PodeAtrasado: false,
	}, int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.4.0.1:1")

	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusGone {
		t.Fatalf("status %d, quero 410", w.Code)
	}
	if got := codigoErro(t, out); got != contract.CodeEncerrada {
		t.Fatalf("código %q, quero ENCERRADA", got)
	}
}

// TestAbrirFinalizada — 1 submissão em regras únicas: 403 FINALIZADA.
func TestAbrirFinalizada(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	// submissão prévia do aluno
	if _, err := pool.Exec(context.Background(), `INSERT INTO submissao
	    (monitor_id, aluno_api_id, n_t, dados, entregue_em, atrasada)
	    VALUES ($1, $2, 1, '{}'::jsonb, now(), false)`, midTest, int64(subTest)); err != nil {
		t.Fatalf("submissão: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.5.0.1:1")

	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, quero 403", w.Code)
	}
	if got := codigoErro(t, out); got != contract.CodeFinalizada {
		t.Fatalf("código %q, quero FINALIZADA", got)
	}
}

// TestAutosaveSemTentativa — abrir obrigatório antes do autosave: 409.
func TestAutosaveSemTentativa(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitor(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.6.0.1:1")

	w, out := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{}}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, quero 409", w.Code)
	}
	if got := codigoErro(t, out); got != contract.CodeConflito {
		t.Fatalf("código %q, quero CONFLITO_DE_ESTADO", got)
	}
}
