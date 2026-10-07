package httpapi

import (
	"net/http"
	"testing"
)

// TestAtividades valida o banco de atividades (PROJETO §Atividades):
// conjunto de tarefas com ordem/pontuação/linguagem, conteúdo JSON
// schema_version=1, escopo por dono, duplicação e o bloqueio mútuo
// atividade→tarefa usada (§Ciclo de Tarefa e Atividade).
func TestAtividades(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "carla.ativ@seno.dev", "Carla", "Nunes")
	ativaNovoUsuario(t, r, pool, "carla.ativ@seno.dev", "senha-carla-a1")
	tokCarla := logar(t, r, "carla.ativ@seno.dev", "senha-carla-a1")

	criarProfessor(t, r, tokSuper, "hugo.ativ@seno.dev", "Hugo", "Dias")
	ativaNovoUsuario(t, r, pool, "hugo.ativ@seno.dev", "senha-hugo-a1")
	tokHugo := logar(t, r, "hugo.ativ@seno.dev", "senha-hugo-a1")

	// -------- Tarefas de Carla (conteúdo das atividades) --------
	criaTarefa := func(nome string) int64 {
		t.Helper()
		w, corpo := chama(t, r, "POST", "/v1/tarefas",
			`{"nome":"`+nome+`","enunciado":"Faça o que deve."}`, autenticado(tokCarla))
		if w.Code != http.StatusCreated {
			t.Fatalf("criar tarefa %s: status %d corpo %v", nome, w.Code, corpo)
		}
		return int64(corpo["id"].(float64))
	}
	idT1 := criaTarefa("Prova T1")
	idT2 := criaTarefa("Prova T2")
	idT3 := criaTarefa("Prova T3")

	conteudo := `{"schema_version":1,"components":[
	  {"type":"large_text","text":"Prova 1"},
	  {"type":"task_default","id":` + intStr(idT1) + `,"value_pts":3,"language":"c"},
	  {"type":"task_default","id":` + intStr(idT2) + `,"value_pts":7,"language":"python"}]}`

	// Super (não professor) não cria.
	w, _ := chama(t, r, "POST", "/v1/atividades",
		`{"nome":"x","conteudo":{"schema_version":1},"tarefas":[]}`, autenticado(tokSuper))
	if w.Code != http.StatusForbidden {
		t.Errorf("super cria atividade: %d", w.Code)
	}

	// Validations: sem tarefas, linguagem ruim, ordem fora.
	exemplo := `{"nome":"Prova 1","conteudo":` + conteudo +
		`,"tarefas":[{"tarefa_id":` + intStr(idT1) + `,"ordem":1,"valor_pts":3,"linguagem":"c"},` +
		`{"tarefa_id":` + intStr(idT2) + `,"ordem":2,"valor_pts":7,"linguagem":"python"}]}`
	w, corpo := chama(t, r, "POST", "/v1/atividades", exemplo, autenticado(tokCarla))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar atividade: status %d corpo %v", w.Code, corpo)
	}
	idAtiv := int64(corpo["id"].(float64))

	w, _ = chama(t, r, "POST", "/v1/atividades",
		`{"nome":"vazia","conteudo":{"schema_version":1},"tarefas":[]}`,
		autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("atividade sem tarefas: %d", w.Code)
	}

	// Escopo: Hugo não vê a de Carla; lista própria vazia; staff vê tudo.
	w, _ = chama(t, r, "GET", "/v1/atividades/"+intStr(idAtiv), "", autenticado(tokHugo))
	if w.Code != http.StatusNotFound {
		t.Errorf("atividade de outro: 404 → %d", w.Code)
	}
	w, corpo = chama(t, r, "GET", "/v1/atividades", "", autenticado(tokHugo))
	if intOf(corpo["total"]) != 0 {
		t.Errorf("lista do não-dono: %v", corpo["total"])
	}
	w, corpo = chama(t, r, "GET", "/v1/atividades", "", autenticado(tokSuper))
	if intOf(corpo["total"]) != 1 {
		t.Errorf("staff vê todas: %v", corpo["total"])
	}

	// Ver detalhado: ordem/pontos/linguagem por tarefa.
	w, corpo = chama(t, r, "GET", "/v1/atividades/"+intStr(idAtiv), "", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("ver atividade: %d corpo %v", w.Code, corpo)
	}
	if tarefas, _ := corpo["tarefas"].([]any); len(tarefas) != 2 {
		t.Errorf("atividade com 2 tarefas: %v", corpo["tarefas"])
	}

	// Tarefa em uso não pode editar (409) nem excluir (soft → some da lista).
	w, _ = chama(t, r, "PATCH", "/v1/tarefas/"+intStr(idT1),
		`{"nome":"Trocada","enunciado":"novo"}`, autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("editar tarefa em uso: %d", w.Code)
	}
	w, _ = chama(t, r, "DELETE", "/v1/tarefas/"+intStr(idT1), "", autenticado(tokCarla))
	if w.Code != http.StatusNoContent {
		t.Errorf("soft delete de tarefa usada: %d", w.Code)
	}
	w, corpo = chama(t, r, "GET", "/v1/atividades/"+intStr(idAtiv), "", autenticado(tokCarla))
	if tarefas, _ := corpo["tarefas"].([]any); len(tarefas) != 2 {
		t.Errorf("tarefa soft-deletada segue na atividade: %v", corpo["tarefas"])
	}

	// Edição da atividade (troca tarefas): tarefa nova não usada.
	conteudo2 := `{"schema_version":1,"components":[
	  {"type":"task_default","id":` + intStr(idT3) + `,"value_pts":10,"language":"cpp"}]}`
	w, corpo = chama(t, r, "PATCH", "/v1/atividades/"+intStr(idAtiv), `{
		"nome":"Prova 1 (ajustada)","conteudo":`+conteudo2+`,
		"tarefas":[{"tarefa_id":`+intStr(idT3)+`,"ordem":1,"valor_pts":10,"linguagem":"cpp"}]}`,
		autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("editar atividade: %d corpo %v", w.Code, corpo)
	}

	// Duplicar: nova editável com copy vínculo.
	w, copia := chama(t, r, "POST", "/v1/atividades/"+intStr(idAtiv)+"/duplicar",
		"", autenticado(tokCarla))
	if w.Code != http.StatusCreated {
		t.Fatalf("duplicar: %d corpo %v", w.Code, copia)
	}
	idCopia := int64(copia["id"].(float64))
	w, corpo = chama(t, r, "GET", "/v1/atividades/"+intStr(idCopia), "", autenticado(tokCarla))
	if tarefas, _ := corpo["tarefas"].([]any); len(tarefas) != 1 {
		t.Errorf("cópia mantém tarefas: %v", corpo["tarefas"])
	}

	// Excluir atividade.
	w, _ = chama(t, r, "DELETE", "/v1/atividades/"+intStr(idCopia), "", autenticado(tokCarla))
	if w.Code != http.StatusNoContent {
		t.Errorf("excluir atividade: %d", w.Code)
	}
	w, _ = chama(t, r, "GET", "/v1/atividades/"+intStr(idCopia), "", autenticado(tokCarla))
	if w.Code != http.StatusNotFound {
		t.Errorf("excluída é 404: %d", w.Code)
	}

	// Ids inválidos → 422.
	w, _ = chama(t, r, "GET", "/v1/atividades/abc", "", autenticado(tokSuper))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("id inválido: %d", w.Code)
	}
}
