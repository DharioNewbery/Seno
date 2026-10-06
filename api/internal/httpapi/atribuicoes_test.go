package httpapi

import (
	"net/http"
	"testing"
	"time"
)

// TestAtribuicoes valida a ligação turma ← atividade (PROJETO
// §Atribuição): dono da turma cria; atividade do mesmo professor;
// duplicado → 409; padrões do autocomplete; turma encerrada bloqueia.
func TestAtribuicoes(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	// Professores: paula (dona de tudo) e hugo (só observa).
	criarProfessor(t, r, tokSuper, "paula.atrib@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.atrib@seno.dev", "senha-paula-x1")
	tokPaula := logar(t, r, "paula.atrib@seno.dev", "senha-paula-x1")

	criarProfessor(t, r, tokSuper, "hugo.atrib@seno.dev", "Hugo", "Dias")
	ativaNovoUsuario(t, r, pool, "hugo.atrib@seno.dev", "senha-hugo-x1")
	tokHugo := logar(t, r, "hugo.atrib@seno.dev", "senha-hugo-x1")

	// Matéria + período + turma + atividade + tarefa (tudo de Paula).
	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"PROG2","nome":"Programação II"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":1,"ano":2027}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"T1"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar turma: %d corpo %v", w.Code, turma)
	}
	idTurma := int64(turma["id"].(float64))
	// Turma alheia (Hugo).
	w, turmaHugo := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`}`,
		autenticado(tokHugo))
	idTurmaHugo := int64(turmaHugo["id"].(float64))

	w, tarefa := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"Hex","enunciado":"Escreva hex."}`, autenticado(tokPaula))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[
	  {"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":10,"language":"python"}]}`
	w, atividade := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Avaliação 1","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":10,"linguagem":"python"}]}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar atividade: %d corpo %v", w.Code, atividade)
	}
	idAtiv := int64(atividade["id"].(float64))

	// -------- Criação com padrão (sem prazo/duração): autocomplete ON --------
	w, atrib := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtiv)+`}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar atribuição: %d corpo %v", w.Code, atrib)
	}
	if aut, _ := atrib["autocomplete"].(bool); !aut {
		t.Errorf("sem prazo/duração: autocomplete ligado: %v", atrib)
	}
	idAtrib := int64(atrib["id"].(float64))

	// Escopo: Hugo não vê nem lista a de Paula.
	w, _ = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtrib), "", autenticado(tokHugo))
	if w.Code != http.StatusNotFound {
		t.Errorf("atribuição alheia: 404 → %d", w.Code)
	}
	w, corpo := chama(t, r, "GET", "/v1/atribuicoes?turma_id="+intStr(idTurma),
		"", autenticado(tokHugo))
	if intOf(corpo["total"]) != 0 {
		t.Errorf("lista alheia escondida: %v", corpo["total"])
	}

	// Duplicado → 409.
	w, _ = chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtiv)+`}`,
		autenticado(tokPaula))
	if w.Code != http.StatusConflict {
		t.Errorf("atribuição duplicada: %d", w.Code)
	}

	// Atividade de outro professor não se atribui a turma de Paula.
	w, tarefaHugo := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"De Hugo","enunciado":"."}`, autenticado(tokHugo))
	idTarefaHugo := int64(tarefaHugo["id"].(float64))
	conteudoHugo := `{"schema_version":1,"components":[
	  {"type":"task_default","id":` + intStr(idTarefaHugo) + `,"value_pts":1,"language":"c"}]}`
	w, atvHugo := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"De Hugo","conteudo":`+conteudoHugo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefaHugo)+`,"ordem":1,"valor_pts":1,"linguagem":"c"}]}`,
		autenticado(tokHugo))
	idAtvHugo := int64(atvHugo["id"].(float64))
	w, falha := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtvHugo)+`}`,
		autenticado(tokPaula))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("atividade de outro professor: %d corpo %v", w.Code, falha)
	}

	// -------- Com prazo: autocomplete OFF --------
	fim := time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	inicio := time.Now().UTC().Format(time.RFC3339Nano)
	w, prova := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idTurmaHugo)+`,"atividade_id":`+intStr(idAtvHugo)+`,
		"inicio":"`+inicio+`","prazo":"`+fim+`","duracao_seg":3600}`,
		autenticado(tokHugo))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição prova: %d corpo %v", w.Code, prova)
	}
	if aut, _ := prova["autocomplete"].(bool); aut {
		t.Errorf("prova (prazo+duração): autocomplete desligado: %v", prova)
	}
	if d, _ := prova["duracao_seg"].(float64); int(d) != 3600 {
		t.Errorf("duracao preservada: %v", prova["duracao_seg"])
	}

	// Validações: prazo anterior ao início.
	antes := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	w, _ = chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtiv)+`,
		"inicio":"`+inicio+`","prazo":"`+antes+`"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("prazo < início: %d", w.Code)
	}

	// -------- Edição: atualiza prazo e pode_atrasado --------
	fim2 := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	w, corpo = chama(t, r, "PATCH", "/v1/atribuicoes/"+intStr(idAtrib),
		`{"prazo":"`+fim2+`","pode_atrasado":true}`, autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("editar atribuição: %d corpo %v", w.Code, corpo)
	}
	if !boolOf(corpo["pode_atrasado"]) {
		t.Errorf("pode_atrasado marcado: %v", corpo)
	}

	// Staff edita/pode ver/excluir.
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtrib), "", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Errorf("staff vê atribuição: %d", w.Code)
	}

	// Turma encerrada bloqueia novas atribuições.
	w, _ = chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/encerrar",
		"", autenticado(tokPaula))
	w, _ = chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtiv)+`}`,
		autenticado(tokPaula))
	if w.Code != http.StatusConflict {
		t.Errorf("atribuição em turma encerrada: %d", w.Code)
	}

	// Excluir.
	w, _ = chama(t, r, "DELETE", "/v1/atribuicoes/"+intStr(idAtrib), "", autenticado(tokPaula))
	if w.Code != http.StatusNoContent {
		t.Errorf("excluir atribuição: %d", w.Code)
	}
	w, _ = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtrib), "", autenticado(tokPaula))
	if w.Code != http.StatusNotFound {
		t.Errorf("excluída é 404: %d", w.Code)
	}
}

func boolOf(v any) bool {
	b, _ := v.(bool)
	return b
}
