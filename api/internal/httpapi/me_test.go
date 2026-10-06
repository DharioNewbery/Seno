package httpapi

import (
	"net/http"
	"testing"
)

// TestMe valida as visões do aluno (PROJETO §Portal Aluno):
// /v1/me/turmas e /v1/me/atribuicoes — só student; matrícula encerrada
// some; ordenação por prazo no cliente.
func TestMe(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "paula.me@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.me@seno.dev", "senha-paula-m1")
	tokPaula := logar(t, r, "paula.me@seno.dev", "senha-paula-m1")

	// Matéria + período + duas turmas.
	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"ME1","nome":"Marcação"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":2,"ano":2027}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, t1 := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"A"}`,
		autenticado(tokPaula))
	idT1 := int64(t1["id"].(float64))
	w, t2 := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"B"}`,
		autenticado(tokPaula))
	idT2 := int64(t2["id"].(float64))

	// Aluna matriculada nas duas; depois sai da B (matrícula encerrada).
	w, mat := chama(t, r, "POST", "/v1/turmas/"+intStr(idT1)+"/matriculas",
		`{"email":"bruna.me@seno.dev","first_name":"Bruna","last_name":"Lima"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("matrícula A: %d corpo %v", w.Code, mat)
	}
	idMat1 := int64(mat["id"].(float64))
	w, _ = chama(t, r, "POST", "/v1/turmas/"+intStr(idT2)+"/matriculas",
		`{"email":"bruna.me@seno.dev"}`, autenticado(tokPaula))
	tokConvite := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConvite, "senha-bruna-m1")
	tokBruna := logar(t, r, "bruna.me@seno.dev", "senha-bruna-m1")

	// Atividade + atribuições nas duas turmas.
	w, tarefa := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"M1","enunciado":"."}`, autenticado(tokPaula))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[{"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":1,"language":"c"}]}`
	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Lista ME","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":1,"linguagem":"c"}]}`,
		autenticado(tokPaula))
	idAtv := int64(atv["id"].(float64))
	w, _ = chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idT1)+`,"atividade_id":`+intStr(idAtv)+`}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição A: %d", w.Code)
	}

	// Só student: professor → 403.
	w, corpo := chama(t, r, "GET", "/v1/me/turmas", "", autenticado(tokPaula))
	if w.Code != http.StatusForbidden {
		t.Errorf("professor no portal aluno: %d corpo %v", w.Code, corpo)
	}

	// Bruna: vê 2 turmas (A e B), 1 atribuição (só A tem).
	w, corpo = chama(t, r, "GET", "/v1/me/turmas", "", autenticado(tokBruna))
	if w.Code != http.StatusOK || lenOf(corpo, "turmas") != 2 {
		t.Errorf("turmas do aluno: %d corpo %v", w.Code, corpo)
	}
	w, corpo = chama(t, r, "GET", "/v1/me/atribuicoes", "", autenticado(tokBruna))
	if w.Code != http.StatusOK || lenOf(corpo, "atribuicoes") != 1 {
		t.Errorf("atribuições do aluno: %d corpo %v", w.Code, corpo)
	}
	if lista, _ := corpo["atribuicoes"].([]any); len(lista) == 1 {
		item, _ := lista[0].(map[string]any)
		if int64(item["turma_id"].(float64)) != idT1 {
			t.Errorf("atribuição da turma A: %v", item)
		}
		if em, _ := item["em_andamento"].(bool); em {
			t.Errorf("sem tentativa: em_andamento=false; %v", item)
		}
	}

	// Filtro por turma sem atribuições devolve vazio.
	w, corpo = chama(t, r, "GET", "/v1/me/atribuicoes?turma_id="+intStr(idT2),
		"", autenticado(tokBruna))
	if lenOf(corpo, "atribuicoes") != 0 {
		t.Errorf("filtro turma B: %v", corpo["atribuicoes"])
	}

	// Abre tentativa na atribuição A → em_andamento=true.
	w, corpo = chama(t, r, "GET", "/v1/me/atribuicoes", "", autenticado(tokBruna))
	var atribuicaoID int64
	if lista, _ := corpo["atribuicoes"].([]any); len(lista) == 1 {
		item, _ := lista[0].(map[string]any)
		atribuicaoID = int64(item["id"].(float64))
	}
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(atribuicaoID)+"/tentativa",
		"", autenticado(tokBruna))
	w, corpo = chama(t, r, "GET", "/v1/me/atribuicoes", "", autenticado(tokBruna))
	if lista, _ := corpo["atribuicoes"].([]any); len(lista) == 1 {
		item, _ := lista[0].(map[string]any)
		if em, _ := item["em_andamento"].(bool); !em {
			t.Errorf("com tentativa: em_andamento=true; %v", item)
		}
	}

	// Encerrar matrícula da turma A → atribuição some do "me".
	w, _ = chama(t, r, "DELETE", "/v1/matriculas/"+intStr(idMat1), "", autenticado(tokSuper))
	w, corpo = chama(t, r, "GET", "/v1/me/atribuicoes", "", autenticado(tokBruna))
	if lenOf(corpo, "atribuicoes") != 0 {
		t.Errorf("matrícula encerrada esconde atribuição: %v", corpo["atribuicoes"])
	}
	w, corpo = chama(t, r, "GET", "/v1/me/turmas", "", autenticado(tokBruna))
	if lenOf(corpo, "turmas") != 1 {
		t.Errorf("matrícula encerrada esconde turma: %v", corpo["turmas"])
	}
}

func lenOf(body map[string]any, chave string) int {
	lista, _ := body[chave].([]any)
	return len(lista)
}
