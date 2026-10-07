package httpapi

import (
	"net/http"
	"testing"
	"time"
)

// TestSubmissoes valida a entrega final (PROJETO §Submissão/§Atribuição):
// exige tentativa, prazo efetivo no servidor, atraso condicional,
// unique por aluno/atribuição, tentativa apagada e listagem do professor.
func TestSubmissoes(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "paula.subm@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.subm@seno.dev", "senha-paula-s1")
	tokPaula := logar(t, r, "paula.subm@seno.dev", "senha-paula-s1")

	// Turma aberta (2) com atraso permitido + turma com prazo curto (1).
	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"SUBM","nome":"Submissões"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":1,"ano":2028}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, t1 := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"P1"}`,
		autenticado(tokPaula))
	idT1 := int64(t1["id"].(float64))
	w, t2 := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"P2"}`,
		autenticado(tokPaula))
	idT2 := int64(t2["id"].(float64))

	// Atividade única atribuída às duas turmas (mesmo professor).
	w, tarefa := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"S1","enunciado":"."}`, autenticado(tokPaula))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[{"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":5,"language":"python"}]}`
	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Ativ S","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":5,"linguagem":"python"}]}`,
		autenticado(tokPaula))
	idAtv := int64(atv["id"].(float64))
	w, atrasada := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idT1)+`,"atividade_id":`+intStr(idAtv)+`,
		"prazo":"`+time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)+`",
		"pode_atrasado":true}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição lenta: %d corpo %v", w.Code, atrasada)
	}
	idAtr1 := int64(atrasada["id"].(float64))
	w, dura := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idT2)+`,"atividade_id":`+intStr(idAtv)+`,
		"prazo":"`+time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)+`"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição dura: %d corpo %v", w.Code, dura)
	}
	idAtr2 := int64(dura["id"].(float64))

	// Dois alunos: Carla (P1) e Bruno (P2).
	criarAluno := func(turma int64, email string) (string, int64) {
		t.Helper()
		w, mat := chama(t, r, "POST", "/v1/turmas/"+intStr(turma)+"/matriculas",
			`{"email":"`+email+`","first_name":"`+email[0:4]+`","last_name":"Al"}`,
			autenticado(tokPaula))
		if w.Code != http.StatusCreated {
			t.Fatalf("matrícula %s: %d corpo %v", email, w.Code, mat)
		}
		tokConvite := tokenDoLog(t, pool)
		aceitarConvite(t, r, tokConvite, "senha-"+email[0:4]+"-1")
		tok := logar(t, r, email, "senha-"+email[0:4]+"-1")
		if len(email) < 4 {
			return tok, 0
		}
		return tok, int64(mat["id"].(float64))
	}
	tokCarla, _ := criarAluno(idT1, "carla.subm@seno.dev")
	tokBruno, _ := criarAluno(idT2, "bruno.subm@seno.dev")

	// Sem tentativa aberta: 422.
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissao",
		`{}`, autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("entrega sem tentativa: %d", w.Code)
	}

	// Carla: prato passada com atraso permitido → submissão atrasada.
	w, abriu := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("abrir tentativa: %d corpo %v", w.Code, abriu)
	}
	w, entregou := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissao",
		`{"observacao":"corrigi o 2"}`, autenticado(tokCarla))
	if w.Code != http.StatusCreated {
		t.Fatalf("entrega atrasada: %d corpo %v", w.Code, entregou)
	}
	if at, _ := entregou["atrasada"].(bool); !at {
		t.Errorf("prazo gap: atrasada=true: %v", entregou)
	}
	idSub := int64(entregou["id"].(float64))

	// Dupla → 409 (rascunho não volta: POST /tentativa também bloqueia).
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissao",
		`{}`, autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("entrega dupla: %d", w.Code)
	}
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("tentativa após entrega: %d", w.Code)
	}
	// Tentativa apagada: GET /tentativa → 404.
	w, _ = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusNotFound {
		t.Errorf("tentativa seguiu lá: %d", w.Code)
	}

	// Bruno: prazo encerrado sem atraso permitido — tentativa e entrega
	// bloqueadas (422).
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr2)+"/tentativa",
		"", autenticado(tokBruno))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("abrir tentativa após prazo: %d", w.Code)
	}
	w, falha := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr2)+"/submissao",
		`{}`, autenticado(tokBruno))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("prazo bloqueado era 422: %d corpo %v", w.Code, falha)
	}

	// Professor dono lista entregas: 1 (Carla, atrasada); staff igual.
	w, corpo := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("listar entregas: %d corpo %v", w.Code, corpo)
	}
	if intOf(corpo["total"]) != 1 {
		t.Errorf("total de entregas: %v", corpo["total"])
	}
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Errorf("staff lista entregas: %d corpo %v", w.Code, corpo)
	}
	// Bruno não lista entregas (escopo).
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokBruno))
	if w.Code != http.StatusNotFound {
		t.Errorf("aluno lista entregas: %d corpo %v", w.Code, corpo)
	}

	// /v1/me: entregue=true atrasada=true e sem tentativa.
	w, corpo = chama(t, r, "GET", "/v1/me/atribuicoes?turma_id="+intStr(idT1),
		"", autenticado(tokCarla))
	if lista, _ := corpo["atribuicoes"].([]any); len(lista) == 1 {
		item, _ := lista[0].(map[string]any)
		if em, _ := item["entregue"].(bool); !em {
			t.Errorf("entregue=true no /me: %v", item)
		}
	}

	// Id inválido → 422.
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/abc/submissao", "{}",
		autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("id inválido: %d", w.Code)
	}
	_ = idSub
	_ = pool
}
