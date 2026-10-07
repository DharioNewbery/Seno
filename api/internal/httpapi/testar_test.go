package httpapi

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seno-project/seno/api/internal/judge0"
	"github.com/seno-project/seno/api/internal/testar"
)

// fakeJudge0 simula o Judge0 com wait=true: devolve um resultado
// aceito por item do lote.
func fakeJudge0(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/submissions/batch" && req.URL.Query().Get("wait") == "true" {
			var itens []judge0.SubmissaoJudge0
			_ = json.NewDecoder(req.Body).Decode(&itens)
			out := make([]map[string]any, 0, len(itens))
			for range itens {
				out = append(out, map[string]any{
					"status": 3, "stdout": "3\n", "stderr": "",
					"time": "0.001", "memory": "1", "cpu_time": "0.001",
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		http.NotFound(w, req)
	}))
	return srv
}

// TestTestar valida o módulo Execução de teste: só testes públicos,
// janela e entrega bloqueiam, 5/min com espera, histórico de 7 dias e
// "testar" do professor sem histórico.
func TestTestar(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	srv := fakeJudge0(t)
	defer srv.Close()
	jCfg := judge0.CarregarConfigDasEnvLigada(srv.URL, "", "", "http://localhost:5173")
	deps.TestarS = testar.New(deps.Store, deps.Audit, judge0.NewClient(jCfg), jCfg)
	r2 := gin.New()
	RegisterRoutes(r2, deps)
	r = r2

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "paula.tst@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.tst@seno.dev", "senha-paula-t2")
	tokPaula := logar(t, r, "paula.tst@seno.dev", "senha-paula-t2")

	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"TST","nome":"Testar"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":2,"ano":2028}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"T"}`,
		autenticado(tokPaula))
	idTurma := int64(turma["id"].(float64))

	w, tarefa := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Soma","enunciado":".",
		"testes":[{"stdin":"","stdout_esperado":"3","publico":true},
		          {"stdin":"","stdout_esperado":"4","publico":false}]}`,
		autenticado(tokPaula))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[{"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":5,"language":"python"}]}`
	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Ativ T","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":5,"linguagem":"python"}]}`,
		autenticado(tokPaula))
	idAtv := int64(atv["id"].(float64))
	w, atrib := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtv)+`}`,
		autenticado(tokPaula))
	idAtr := int64(atrib["id"].(float64))

	w, _ = chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"carla.tst@seno.dev","first_name":"Carla","last_name":"Souza"}`,
		autenticado(tokPaula))
	tokConvite := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConvite, "senha-carla-t2")
	tokCarla := logar(t, r, "carla.tst@seno.dev", "senha-carla-t2")

	// Sem tentativa → 422.
	w, corpo := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/testar",
		`{"tarefa_id":`+intStr(idTarefa)+`,"codigo":"print(1+2)"}`, autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("testar sem tentativa: %d corpo %v", w.Code, corpo)
	}

	// Abre tentativa → roda (linguagem da atividade, teste público only).
	w, abriu := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("tentativa: %d %v", w.Code, abriu)
	}
	w, exec := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/testar",
		`{"tarefa_id":`+intStr(idTarefa)+`,"codigo":"print(1+2)"}`, autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("testar: %d corpo %v", w.Code, exec)
	}
	if linguagem, _ := exec["linguagem"].(string); linguagem != "python" {
		t.Errorf("linguagem da atividade: %v", exec["linguagem"])
	}

	// Histórico: 1 execução nos últimos 7 dias.
	w, hist := chama(t, r, "GET", "/v1/me/testes?atribuicao_id="+intStr(idAtr),
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK || intOf(hist["total"]) != 1 {
		t.Errorf("histórico: %d corpo %v", w.Code, hist)
	}

	// Professor testa no banco (não gera histórico): aceita.
	w, profTest := chama(t, r, "POST", "/v1/tarefas/"+intStr(idTarefa)+"/testar",
		`{"linguagem":"python","codigo":"print(1+2)"}`, autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("prof testar: %d corpo %v", w.Code, profTest)
	}
	w, hist2 := chama(t, r, "GET", "/v1/me/testes?atribuicao_id="+intStr(idAtr),
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK || intOf(hist2["total"]) != 1 {
		t.Errorf("histórico sem professor: %d corpo %v", w.Code, hist2)
	}

	// Limite 5/min: mais 4 → 6º bloqueado com mensagem de espera.
	for i := 0; i < 4; i++ {
		w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/testar",
			`{"tarefa_id":`+intStr(idTarefa)+`,"codigo":"print(1+2)"}`,
			autenticado(tokCarla))
	}
	w, limitado := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/testar",
		`{"tarefa_id":`+intStr(idTarefa)+`,"codigo":"print(1+2)"}`,
		autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("limite 5/min: %d corpo %v", w.Code, limitado)
	}
	if msg, _ := limitado["erro"].(map[string]any); msg["mensagem"] == nil {
		t.Errorf("mensagem de espera no erro: %v", limitado)
	}

	// Entrega → testar bloqueado (409 "já entregue").
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/submissao",
		`{}`, autenticado(tokCarla))
	w, entregou := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/testar",
		`{"tarefa_id":`+intStr(idTarefa)+`,"codigo":"print(1+2)"}`,
		autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("testar após entrega: %d corpo %v", w.Code, entregou)
	}
}
