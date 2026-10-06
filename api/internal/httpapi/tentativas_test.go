package httpapi

import (
	"net/http"
	"testing"
)

// TestTentativas valida o rascunho do aluno (PROJETO §Tentativa):
// única por (aluno, atribuição), comecou_em, revisão anti duas-abas
// (409 em gravação antiga), escopo (matrícula ativa; professor 403).
func TestTentativas(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "paula.tent@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.tent@seno.dev", "senha-paula-t1")
	tokPaula := logar(t, r, "paula.tent@seno.dev", "senha-paula-t1")

	// Matéria + período + turma + atividade + atribuição.
	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"BD1","nome":"Banco de Dados"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":3,"ano":2027}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"T3"}`,
		autenticado(tokPaula))
	idTurma := int64(turma["id"].(float64))
	w, tarefa := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"SQL1","enunciado":"Escreva SELECT."}`, autenticado(tokPaula))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[
	  {"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":5,"language":"python"}]}`
	w, atividade := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Lista 1","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":5,"linguagem":"python"}]}`,
		autenticado(tokPaula))
	idAtiv := int64(atividade["id"].(float64))
	w, atrib := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtiv)+`}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição: %d corpo %v", w.Code, atrib)
	}
	idAtrib := int64(atrib["id"].(float64))

	// Aluna Carla (matriculada) e Bruno (sem matrícula).
	w, mat := chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"carla.tent@seno.dev","first_name":"Carla","last_name":"Souza"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("matrícula: %d corpo %v", w.Code, mat)
	}
	tokConvite := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConvite, "senha-carla-t1")
	tokCarla := logar(t, r, "carla.tent@seno.dev", "senha-carla-t1")

	criarProfessor(t, r, tokSuper, "hugo.tent@seno.dev", "Hugo", "Dias")
	ativaNovoUsuario(t, r, pool, "hugo.tent@seno.dev", "senha-hugo-t1")
	tokHugo := logar(t, r, "hugo.tent@seno.dev", "senha-hugo-t1")

	// GET antes de abrir: 404.
	w, _ = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusNotFound {
		t.Errorf("sem tentativa: 404 → %d", w.Code)
	}

	// Não-alunos: professor (dono da turma) → 403.
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		"", autenticado(tokPaula))
	if w.Code != http.StatusForbidden {
		t.Errorf("professor abre tentativa: %d", w.Code)
	}
	// Aluno sem matrícula → 404 (escopo).
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		"", autenticado(tokHugo))
	if w.Code != http.StatusForbidden {
		t.Errorf("professor alheio abre tentativa: %d", w.Code)
	}

	// Abrir: cria com revisão 1 e comecou_em.
	w, tento := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("abrir tentativa: %d corpo %v", w.Code, tento)
	}
	if intOf(tento["revisao"]) != 1 {
		t.Errorf("revisão inicial 1: %v", tento)
	}
	idTent := int64(tento["id"].(float64))

	// Idempotente: reabrir devolve a mesma tentativa.
	w, tento2 := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		"", autenticado(tokCarla))
	if int64(tento2["id"].(float64)) != idTent || intOf(tento2["revisao"]) != 1 {
		t.Errorf("reabrir idempotente: %v", tento2)
	}

	// 1ª gravação (autosave, revisão 1) → 2.
	w, gravou := chama(t, r, "PUT", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		`{"revisao":1,"snapshot":{"tasks":[{"id":`+intStr(idTarefa)+`,"text":"print(5)"}]}}`,
		autenticado(tokCarla))
	if w.Code != http.StatusOK || intOf(gravou["revisao"]) != 2 {
		t.Errorf("gravar revisão 1 → 2: %d %v", w.Code, gravou)
	}

	// GET agora devolve o rascunho atualizado.
	w, corpo := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		"", autenticado(tokCarla))
	if intOf(corpo["revisao"]) != 2 {
		t.Errorf("rascunho revisão 2: %v", corpo)
	}

	// Gravação com revisão antiga → 409 (duas abas).
	w, corpoGrav := chama(t, r, "PUT", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		`{"revisao":1,"snapshot":{"tasks":[]}}`, autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("revisão antiga: %d corpo %v", w.Code, corpoGrav)
	}
	// Revisão à frente (sem existir) também → 409.
	w, _ = chama(t, r, "PUT", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		`{"revisao":5,"snapshot":{"tasks":[]}}`, autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("revisão à frente: %d", w.Code)
	}

	// Próxima gravação da revisão corrente (2) → 3.
	w, gravou = chama(t, r, "PUT", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		`{"revisao":2,"snapshot":{"tasks":[{"id":`+intStr(idTarefa)+`,"text":"print(6)"}]}}`,
		autenticado(tokCarla))
	if w.Code != http.StatusOK || intOf(gravou["revisao"]) != 3 {
		t.Errorf("gravar revisão 2 → 3: %d %v", w.Code, gravou)
	}

	// Snapshot não-objeto → 422.
	w, _ = chama(t, r, "PUT", "/v1/atribuicoes/"+intStr(idAtrib)+"/tentativa",
		`{"revisao":3,"snapshot":"texto"}`, autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("snapshot inválido: %d", w.Code)
	}

	// Visão do aluno: material completo; professor → 403.
	w, visao := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtrib)+"/aluno",
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("visão do aluno: %d corpo %v", w.Code, visao)
	}
	if tarefas, _ := visao["atividade"].(map[string]any)["tarefas"].([]any); len(tarefas) != 1 {
		t.Errorf("visão com 1 tarefa: %v", visao)
	}
	if _, ok := visao["tentativa"].(map[string]any); !ok {
		t.Errorf("rascunho aberto aparece na visão: %v", visao)
	}
	w, _ = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtrib)+"/aluno",
		"", autenticado(tokPaula))
	if w.Code != http.StatusForbidden {
		t.Errorf("professor usa visão do aluno: %d", w.Code)
	}
}
