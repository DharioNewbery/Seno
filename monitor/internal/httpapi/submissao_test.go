package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/seno-project/seno/contract"
)

// TestSubmissaoFluxoFeliz — avaliar → confirmar → entregue n_t=1,
// tentativa apagada, correção e2e com Judge0 real (nota 10).
func TestSubmissaoFluxoFeliz(t *testing.T) {
	if os.Getenv("SENO_JUDGE0_URL") == "" {
		t.Skip("sem judge0 e2e")
	}
	pool := testeDB(t)
	_, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest), 101); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, ipF+"6.")

	// abrir + autosave (respostas) + pré-avaliação: relatório accepted.
	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", w.Code, w.Body.String())
	}
	if w, _ := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{"1":"print(int(input())*2)"}}`); w.Code != http.StatusOK {
		t.Fatalf("autosave: %d %s", w.Code, w.Body.String())
	}
	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("avaliar: %d %s", w.Code, w.Body.String())
	}
	preID := out["pre_id"].(string)

	// confirmar: entregue n_t=1.
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/confirmar",
		`{"pre_id":"`+preID+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("confirmar: %d %s", w.Code, w.Body.String())
	}
	if out["status"] != "entregue" || out["n_t"].(float64) != 1 || out["atrasada"] != false {
		t.Fatalf("entrega inesperada: %+v", out)
	}
	var temTentativa bool
	if err := pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM tentativa WHERE monitor_id = $1 AND aluno_api_id = $2)`,
		midTest, int64(subTest)).Scan(&temTentativa); err != nil || temTentativa {
		t.Fatalf("tentativa deveria ter sido apagada (err=%v tem=%v)", err, temTentativa)
	}

	// rotas agora bloqueadas (cota única): 403 FINALIZADA + correcao_url.
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusForbidden || codigoErro(t, out) != contract.CodeFinalizada {
		t.Fatalf("bloqueio: %d %s", w.Code, w.Body.String())
	}
	if det := out["erro"].(map[string]any)["detalhe"].(map[string]any); det["correcao_url"].(string) != "/aluno/atividades/12" {
		t.Fatalf("correcao_url inesperada: %+v", det)
	}

	// correção e2e real: nota_auto 10 (valor da tarefa), status concluida.
	var status string
	var nota *float64
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(context.Background(),
			`SELECT c.status, c.nota_auto FROM correcao c
			   JOIN submissao sb ON sb.id = c.submissao_id
			   WHERE sb.monitor_id = $1 AND sb.aluno_api_id = $2`,
			midTest, int64(subTest)).Scan(&status, &nota); err == nil {
			if status == "concluida" {
				break
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	if status != "concluida" {
		t.Fatalf("correção: status=%s (não concluiu)", status)
	}
	if nota == nil || *nota != 10 {
		t.Fatalf("nota_auto %#v, quero 10", nota)
	}
}

// TestSubmissaoCancelarEExpiracao — cancelar descarta (tentativa viva);
// confirmar com pre cancelado → 409; pré expirada → 409; revisão mudou → 409.
func TestSubmissaoCancelarEExpiracao(t *testing.T) {
	if os.Getenv("SENO_JUDGE0_URL") == "" {
		t.Skip("sem judge0: cancelar/expirar usam pré-avaliação real")
	}
	pool := testeDB(t)
	_, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, ipF+"7.")

	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir: %d", w.Code)
	}
	if w, _ := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{"1":"print(int(input())*2)"}}`); w.Code != http.StatusOK {
		t.Fatalf("autosave: %d", w.Code)
	}
	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("avaliar: %d %s", w.Code, w.Body.String())
	}
	preID := out["pre_id"].(string)

	// cancelar: tentativa continua viva.
	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/cancelar",
		`{"pre_id":"`+preID+`"}`); w.Code != http.StatusOK {
		t.Fatalf("cancelar: %d %s", w.Code, w.Body.String())
	}
	var temTentativa bool
	if err := pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM tentativa WHERE monitor_id = $1 AND aluno_api_id = $2)`,
		midTest, int64(subTest)).Scan(&temTentativa); err != nil || !temTentativa {
		t.Fatalf("tentativa deveria continuar (err=%v tem=%v)", err, temTentativa)
	}
	// confirmar com a pré cancelada: 409 CONFLITO.
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/confirmar",
		`{"pre_id":"`+preID+`"}`)
	if w.Code != http.StatusConflict || codigoErro(t, out) != contract.CodeConflito {
		t.Fatalf("confirmar com pré cancelada: %d %s", w.Code, w.Body.String())
	}

	// reavaliar (nova pré), mudar a tentativa (autosave) → confirmar: 409 ATUALIZADA.
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("reavaliar: %d %s", w.Code, w.Body.String())
	}
	preID2 := out["pre_id"].(string)
	w, _ = chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":1,"respostas":{"1":"print(int(input())*2) "}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("autosave 2: %d %s", w.Code, w.Body.String())
	}
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/confirmar",
		`{"pre_id":"`+preID2+`"}`)
	if w.Code != http.StatusConflict || codigoErro(t, out) != contract.CodeAtualizada {
		t.Fatalf("confirmar com revisão divergente: %d %s", w.Code, w.Body.String())
	}

	// pré viva, mas expirada (força expira no banco): 409 CONFLITO.
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("avaliar 2: %d", w.Code)
	}
	preID3 := out["pre_id"].(string)
	if _, err := pool.Exec(context.Background(),
		`UPDATE pre_avaliacao SET expira_em = now() - interval '1 s' WHERE id = $1`, preID3); err != nil {
		t.Fatalf("expira: %v", err)
	}
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/confirmar",
		`{"pre_id":"`+preID3+`"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("confirmar expirada: %d %s", w.Code, w.Body.String())
	}
}

// TestSubmissaoMultiplasComRecarga — submissoes_multiplas: 2 entregas; a
// segunda abrir transporta o conteúdo da última submissão; 3ª tentativa 403.
func TestSubmissaoMultiplasComRecarga(t *testing.T) {
	if os.Getenv("SENO_JUDGE0_URL") == "" {
		t.Skip("sem judge0: recarga + entrega usa pré-avaliação real")
	}
	pool := testeDB(t)
	_, r := montaMonitorJudge0(t, pool)
	// regras: multiplás, max 2 — mono-semeia direto no banco
	if err := semeiaAtividadeRegrasDB(pool, "ativa",
		contract.Regras{SubmissoesMultiplas: true, MaxSubmissoes: 2},
		int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, ipF+"8.")

	entrega := func(pPreID string) (*httptest.ResponseRecorder, map[string]any) {
		return chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/confirmar",
			`{"pre_id":"`+pPreID+`"}`)
	}
	// entrega 1
	if w, _ := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", ""); w.Code != http.StatusOK {
		t.Fatalf("abrir 1: %d", w.Code)
	}
	if w, _ := chamaAluno(t, r, sess, "PUT", "/a/atividade/"+midTest+"/tentativa",
		`{"revisao":0,"respostas":{"1":"print(int(input())*2)"}}`); w.Code != http.StatusOK {
		t.Fatalf("autosave 1: %d", w.Code)
	}
	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	preID := out["pre_id"].(string)
	w, out = entrega(preID)
	if w.Code != http.StatusOK || out["n_t"].(float64) != 1 {
		t.Fatalf("entrega 1: %d %s", w.Code, w.Body.String())
	}

	// abrir de novo: transporta o conteúdo da última entrega (revisão 0).
	w, out2 := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusOK {
		t.Fatalf("abrir 2: %d %s", w.Code, w.Body.String())
	}
	tent := out2["tentativa"].(map[string]any)
	if tent["revisao"].(float64) != 0 {
		t.Fatalf("revisão da recarga %v, quero 0", tent["revisao"])
	}
	if resp := tent["respostas"].(map[string]any); resp["1"] != "print(int(input())*2)" {
		t.Fatalf("recarga não trouxe o conteúdo: %+v", resp)
	}
	// pré avaliar e confirmar: n_t=2
	_, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	preID = out["pre_id"].(string)
	w, out = entrega(preID)
	if w.Code != http.StatusOK || out["n_t"].(float64) != 2 {
		t.Fatalf("entrega 2: %d %s", w.Code, w.Body.String())
	}
	// 3ª tentativa: 403 FINALIZADA (cota max=2)
	w, out = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusForbidden || codigoErro(t, out) != contract.CodeFinalizada {
		t.Fatalf("bloqueio da 3ª: %d %s", w.Code, w.Body.String())
	}
}
