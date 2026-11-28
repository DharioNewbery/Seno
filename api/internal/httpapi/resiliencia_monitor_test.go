package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/seno-project/seno/api/internal/monitor"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/contract"
)

// TestAPIResilienciaQuandoMonitorForaDoAr valida:
//  1. CriarAtribuicao com o Monitor fora do ar NÃO perde os dados: grava a atribuição e enfileira monitor_transferir;
//  2. EntrarMonitor com o Monitor fora do ar devolve 503 SERVICO_INDISPONIVEL sem marcar falsamente 'aberta';
//  3. Quando o Monitor volta ao ar, EntrarMonitor conclui a transferência pendente na hora e emite o token st;
//  4. ExcluirSubmissao quando o Monitor volta a cair devolve 503 SERVICO_INDISPONIVEL sem apagar localmente (evitando inconsistência).
func TestAPIResilienciaQuandoMonitorForaDoAr(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)
	seg := "segredo-resiliencia-api-monitor-32-bytes!!"

	var monitorOnline atomic.Bool
	monSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !monitorOnline.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if req.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		if req.Method == http.MethodPost && req.URL.Path == "/interna/v1/atividades" {
			_, _ = io.ReadAll(req.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"monitor_id":"atv_recuperada_1"}`))
			return
		}
		if req.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, req)
	}))
	defer monSrv.Close()

	st := store.New(pool)
	deps.Monitor = monitor.New(st, deps.Audit, monSrv.URL, monSrv.URL, seg)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "prof.resil@seno.dev", "Rita", "Nunes")
	ativaNovoUsuario(t, r, pool, "prof.resil@seno.dev", "senha-rita-1")
	tokRita := logar(t, r, "prof.resil@seno.dev", "senha-rita-1")

	_, mat := chama(t, r, "POST", "/v1/materias", `{"codigo":"RES1","nome":"Resiliência"}`, autenticado(tokSuper))
	idMat := int64(mat["id"].(float64))
	_, per := chama(t, r, "POST", "/v1/periodos", `{"trimestre":1,"ano":2028}`, autenticado(tokSuper))
	idPer := int64(per["id"].(float64))
	_, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMat)+`,"periodo_id":`+intStr(idPer)+`,"titulo":"TR"}`, autenticado(tokRita))
	idTurma := int64(turma["id"].(float64))

	// Matricula e ativa o aluno antes da atribuição.
	chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"aluno.resil@seno.dev","first_name":"Beto","last_name":"Luz"}`, autenticado(tokRita))
	tokConv := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConv, "senha-beto-1")
	tokBeto := logar(t, r, "aluno.resil@seno.dev", "senha-beto-1")
	_, meBeto := chama(t, r, "GET", "/v1/auth/me", "", autenticado(tokBeto))
	idBeto := int64(meBeto["id"].(float64))

	_, tar := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Dobro","enunciado":"Imprima o dobro.",
		"testes":[{"stdin":"2","stdout_esperado":"4","publico":true}]
	}`, autenticado(tokRita))
	idTar := int64(tar["id"].(float64))
	_, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Prova Resiliente",
		"conteudo":{"schema_version":1,"components":[]},
		"tarefas":[{"tarefa_id":`+intStr(idTar)+`,"ordem":1,"valor_pts":10,"linguagem":"python"}]
	}`, autenticado(tokRita))
	idAtv := int64(atv["id"].(float64))

	// 1. Monitor FORA DO AR: CriarAtribuicao salva a atribuição (201) e agenda job na fila!
	monitorOnline.Store(false)
	prazo := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)
	w, atrib := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtv)+`,"prazo":"`+prazo+`"}`,
		autenticado(tokRita))
	if w.Code != http.StatusCreated {
		t.Fatalf("CriarAtribuicao com Monitor offline deveria preservar e enfileirar (201), obteve %d (%v)", w.Code, atrib)
	}
	idAtr := int64(atrib["id"].(float64))

	var jobsAgendados int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM jobs WHERE kind = $1 AND status = 'scheduled'`, monitor.KindTransferir).Scan(&jobsAgendados); err != nil || jobsAgendados == 0 {
		t.Fatalf("esperava job %s agendado na fila; qtd=%d err=%v", monitor.KindTransferir, jobsAgendados, err)
	}

	// 2. Aluno tenta entrar com Monitor ainda FORA DO AR: recebe 503 SERVICO_INDISPONIVEL e NÃO grava estado 'aberta'.
	w, errEntrar := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr)+"/entrar", "", autenticado(tokBeto))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Entrar com Monitor offline deveria retornar 503, obteve %d (%v)", w.Code, errEntrar)
	}
	if erroObj, _ := errEntrar["erro"].(map[string]any); erroObj["codigo"] != platform.CodeServiceUnavailable {
		t.Fatalf("código esperado %s, obtido %v", platform.CodeServiceUnavailable, errEntrar)
	}
	var marcouAberta bool
	_ = pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM monitor_estado_aluno WHERE atribuicao_id = $1 AND aluno_id = $2)`,
		idAtr, idBeto).Scan(&marcouAberta)
	if marcouAberta {
		t.Fatal("não deveria ter marcado atividade como 'aberta' quando o Monitor estava fora do ar")
	}

	// 3. Monitor VOLTA AO AR: EntrarMonitor transfere automaticamente a atribuição pendente e devolve URL válida!
	monitorOnline.Store(true)
	w, okEntrar := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr)+"/entrar", "", autenticado(tokBeto))
	if w.Code != http.StatusOK || okEntrar["url"] == nil {
		t.Fatalf("Entrar após retorno do Monitor falhou: %d (%v)", w.Code, okEntrar)
	}

	// 4. Simula submissão ingerida do Monitor e queda subsequente do Monitor durante tentativa de exclusão pelo professor.
	agora := time.Now().UTC()
	pac := contract.PacoteSubmissaoMonitor{
		MonitorID:       "atv_recuperada_1",
		AtribuicaoAPIID: idAtr,
		AlunoAPIID:      idBeto,
		Submissao: &contract.SubmissaoMonitor{
			ID: 1, AlunoID: idBeto, NT: 1, ComecouEm: agora.Add(-time.Minute), EntregueEm: agora,
			Respostas: map[string]string{"1": "print(4)"},
		},
		Correcao: &contract.CorrecaoMonitor{Status: "concluida"},
	}
	corpoPac, _ := json.Marshal(pac)
	mac, ts := contract.AssinarMAC([]byte(seg), time.Now(), corpoPac)
	reqIng := httptest.NewRequest("POST", "/v1/interna/monitor/submissoes", bytes.NewReader(corpoPac))
	reqIng.Header.Set("Content-Type", "application/json")
	reqIng.Header.Set(contract.HeaderMAC, mac)
	reqIng.Header.Set(contract.HeaderTime, ts)
	wIng := httptest.NewRecorder()
	r.ServeHTTP(wIng, reqIng)
	if wIng.Code != http.StatusOK {
		t.Fatalf("ingestão falhou: %d %s", wIng.Code, wIng.Body.String())
	}
	var ingOut map[string]any
	_ = json.Unmarshal(wIng.Body.Bytes(), &ingOut)
	idSub := int64(ingOut["submissao_id"].(float64))

	// Monitor cai antes do DELETE -> API deve recusar com 503 e preservar a submissão local sem divergência!
	monitorOnline.Store(false)
	w, errDel := chama(t, r, "DELETE", "/v1/submissoes/"+intStr(idSub), "", autenticado(tokRita))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("ExcluirSubmissao com Monitor offline deveria retornar 503, obteve %d (%v)", w.Code, errDel)
	}
	var aindaExiste bool
	_ = pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM submissoes WHERE id = $1)`, idSub).Scan(&aindaExiste)
	if !aindaExiste {
		t.Fatal("submissão local não deveria ter sido apagada quando a exclusão no Monitor falhou")
	}

	// Monitor volta -> DELETE conclui nos dois lados!
	monitorOnline.Store(true)
	w, _ = chama(t, r, "DELETE", "/v1/submissoes/"+intStr(idSub), "", autenticado(tokRita))
	if w.Code != http.StatusOK {
		t.Fatalf("ExcluirSubmissao após retorno do Monitor falhou: %d", w.Code)
	}
}
