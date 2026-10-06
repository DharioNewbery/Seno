package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// TestCorrecoes valida o ciclo (PROJETO §Correção/§Visibilidade): nasce
// na entrega; sem Judge0 fica pendente; callbacks simulados fecham a
// correção com nota automática; professor ajusta, confirma e publica;
// aluno só vê depois de publicar.
func TestCorrecoes(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "paula.corr@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.corr@seno.dev", "senha-paula-c1")
	tokPaula := logar(t, r, "paula.corr@seno.dev", "senha-paula-c1")

	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"CORR","nome":"Correções"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":2,"ano":2028}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"C1"}`,
		autenticado(tokPaula))
	idTurma := int64(turma["id"].(float64))

	// Tarefa A com 2 testes (o 2º reprova) e Tarefa B aberta.
	w, tarefaA := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Somar","enunciado":".",
		"testes":[{"stdin":"","stdout_esperado":"3","publico":true},
		          {"stdin":"","stdout_esperado":"4","publico":false}]}`,
		autenticado(tokPaula))
	idTarefaA := int64(tarefaA["id"].(float64))
	w, tarefaB := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"Reflexo","enunciado":"."}`, autenticado(tokPaula))
	idTarefaB := int64(tarefaB["id"].(float64))

	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Ativ C","conteudo":{"schema_version":1,"components":[
		  {"type":"task_default","id":`+intStr(idTarefaA)+`,"value_pts":7,"language":"python"},
		  {"type":"task_default","id":`+intStr(idTarefaB)+`,"value_pts":3,"language":"python"}]},
		"tarefas":[{"tarefa_id":`+intStr(idTarefaA)+`,"ordem":1,"valor_pts":7,"linguagem":"python"},
		           {"tarefa_id":`+intStr(idTarefaB)+`,"ordem":2,"valor_pts":3,"linguagem":"python"}]}`,
		autenticado(tokPaula))
	idAtv := int64(atv["id"].(float64))
	w, atrib := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtv)+`}`,
		autenticado(tokPaula))
	idAtr := int64(atrib["id"].(float64))

	w, _ = chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"carla.corr@seno.dev","first_name":"Carla","last_name":"Souza"}`,
		autenticado(tokPaula))
	tokConvite := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConvite, "senha-carla-c1")
	tokCarla := logar(t, r, "carla.corr@seno.dev", "senha-carla-c1")

	// Tentativa + entrega → correção nasce pendente (Judge0 desligado).
	w, tenta := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("tentativa: %d corpo %v", w.Code, tenta)
	}
	w, _ = chama(t, r, "PUT", "/v1/atribuicoes/"+intStr(idAtr)+"/tentativa", `{
		"revisao":1,"snapshot":{"tasks":[{"id":`+intStr(idTarefaA)+`,"text":"print(a+b)"}]}}`,
		autenticado(tokCarla))
	w, entrega := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/submissao",
		`{}`, autenticado(tokCarla))
	if w.Code != http.StatusCreated {
		t.Fatalf("entrega: %d corpo %v", w.Code, entrega)
	}
	idSub := int64(entrega["id"].(float64))

	// Visão do professor: pendente, 2 correcao_tarefas.
	w, visaoProf := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub)+"/correcao",
		"", autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("correção professor: %d corpo %v", w.Code, visaoProf)
	}
	cor := visaoProf["correcao"].(map[string]any)
	if cor["status"] != "pendente" {
		t.Errorf("status pendente: %v", cor["status"])
	}
	var ctA, ctB int64
	for _, item := range cor["tarefas"].([]any) {
		te := item.(map[string]any)
		switch te["tarefa_id"].(float64) {
		case float64(idTarefaA):
			ctA = int64(te["id"].(float64))
		case float64(idTarefaB):
			ctB = int64(te["id"].(float64))
		}
	}
	if ctA == 0 || ctB == 0 {
		t.Fatalf("mapeamento correcao_tarefas: %v", cor["tarefas"])
	}

	// Ids dos testes da tarefa A: t1 aceito, t2 reprovado.
	var te1, te2 int64
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM testes_tarefa WHERE tarefa_id = $1 ORDER BY id LIMIT 1",
		idTarefaA).Scan(&te1); err != nil {
		t.Fatalf("teste 1: %v", err)
	}
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM testes_tarefa WHERE tarefa_id = $1 ORDER BY id OFFSET 1 LIMIT 1",
		idTarefaA).Scan(&te2); err != nil {
		t.Fatalf("teste 2: %v", err)
	}

	// Callbacks simulados do Judge0 (cfg de teste sem chave — sem guarda).
	w, corpoCB := chama(t, r, "POST",
		fmt.Sprintf("/v1/judge0/callback?par=%d:%d", ctA, te1),
		`{"status":3,"stdout":"3"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("callback t1: %d corpo %v", w.Code, corpoCB)
	}
	w, corpoCB = chama(t, r, "POST",
		fmt.Sprintf("/v1/judge0/callback?par=%d:%d", ctA, te2),
		`{"status":5}`)
	if w.Code != http.StatusOK {
		t.Fatalf("callback t2: %d corpo %v", w.Code, corpoCB)
	}
	// Com todos os resultados da tarefa A: correção segue pendente?
	// Tarefa B (aberta) não tem resultados → não conclui.
	w, visao2 := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub)+"/correcao",
		"", autenticado(tokPaula))
	for _, item := range visao2["correcao"].(map[string]any)["tarefas"].([]any) {
		te := item.(map[string]any)
		if te["tarefa_id"].(float64) == float64(idTarefaA) {
			if na, _ := te["nota_auto"].(float64); int(na) != 0 {
				t.Errorf("nota auto A = 0 (reprovada): %v", te)
			}
			if res, _ := te["resultados"].([]any); len(res) != 2 {
				t.Errorf("2 resultados na A: %v", te["resultados"])
			}
		}
	}

	// Aluna antes de publicar: apenas estado.
	w, visaoAluna := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub)+"/minha",
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("minha: %d corpo %v", w.Code, visaoAluna)
	}
	if boolOf(visaoAluna["publicada"]) {
		t.Errorf("antes de publicar não há detalhes: %v", visaoAluna)
	}

	// Professor ajusta feedback + notas (B manual) → confirmar → publicar.
	w, corpoEd := chama(t, r, "PATCH", "/v1/submissoes/"+intStr(idSub)+"/correcao", `{
		"feedback":"Boa, mas refaça a soma.",
		"tarefas":[{"id":`+intStr(ctB)+`,"nota_final":3,"feedback":"legível"},
		           {"id":`+intStr(ctA)+`,"nota_final":0}]}`,
		autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("editar: %d corpo %v", w.Code, corpoEd)
	}
	w, _ = chama(t, r, "POST", "/v1/submissoes/"+intStr(idSub)+"/correcao/confirmar",
		"", autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Errorf("confirmar: %d", w.Code)
	}
	w, corpoPub := chama(t, r, "POST", "/v1/submissoes/"+intStr(idSub)+"/correcao/publicar",
		"", autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Errorf("publicar: %d corpo %v", w.Code, corpoPub)
	}

	// Aluna pós-publicação: nota, feedback e testes (incl. privados).
	w, visaoPub := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub)+"/minha",
		"", autenticado(tokCarla))
	if !boolOf(visaoPub["publicada"]) {
		t.Errorf("pós-publicação: publicada=true; %v", visaoPub)
	}
	for _, item := range visaoPub["correcao"].(map[string]any)["tarefas"].([]any) {
		te := item.(map[string]any)
		if te["tarefa_id"].(float64) != float64(idTarefaA) {
			continue
		}
		if nf, _ := te["nota_final"].(float64); int(nf) != 0 {
			t.Errorf("nota final A = 0: %v", te)
		}
		if res, _ := te["resultados"].([]any); len(res) != 2 {
			t.Errorf("resultados da A: %v", te["resultados"])
		}
	}

	// Escopo: outro professor → 404.
	criarProfessor(t, r, tokSuper, "hugo.corr@seno.dev", "Hugo", "Dias")
	ativaNovoUsuario(t, r, pool, "hugo.corr@seno.dev", "senha-hugo-c1")
	tokHugo := logar(t, r, "hugo.corr@seno.dev", "senha-hugo-c1")
	w, _ = chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub)+"/correcao",
		"", autenticado(tokHugo))
	if w.Code != http.StatusNotFound {
		t.Errorf("correção alheia: %d", w.Code)
	}
}
