package httpapi

import (
	"net/http"
	"testing"
)

// TestPublicarLote valida POST /v1/atribuicoes/:id/correcoes/publicar:
// publica de uma vez só as correções confirmadas da atribuição
// (dono|staff); não confirmadas ficam; reexecução é idempotente.
func TestPublicarLote(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "paula.lote@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.lote@seno.dev", "senha-paula-p1")
	tokPaula := logar(t, r, "paula.lote@seno.dev", "senha-paula-p1")
	// Professor alheio (escopo: 404).
	criarProfessor(t, r, tokSuper, "ze.lote@seno.dev", "Zé", "Lopes")
	ativaNovoUsuario(t, r, pool, "ze.lote@seno.dev", "senha-ze-lot")
	tokZe := logar(t, r, "ze.lote@seno.dev", "senha-ze-lot")

	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"LOTE","nome":"Publicação em lote"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":1,"ano":2028}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, t1 := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"P1"}`,
		autenticado(tokPaula))
	idTurma := int64(t1["id"].(float64))

	w, tarefa := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"L1","enunciado":"."}`, autenticado(tokPaula))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[{"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":5,"language":"python"}]}`
	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Ativ L","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":5,"linguagem":"python"}]}`,
		autenticado(tokPaula))
	idAtv := int64(atv["id"].(float64))
	w, atr := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtv)+`}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição: %d corpo %v", w.Code, atr)
	}
	idAtr := int64(atr["id"].(float64))

	criarAluno := func(email string) string {
		t.Helper()
		w, _ := chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
			`{"email":"`+email+`","first_name":"`+email[0:4]+`","last_name":"Al"}`,
			autenticado(tokPaula))
		if w.Code != http.StatusCreated {
			t.Fatalf("matrícula %s: %d", email, w.Code)
		}
		tokConvite := tokenDoLog(t, pool)
		aceitarConvite(t, r, tokConvite, "senha-"+email[0:4]+"-1")
		return logar(t, r, email, "senha-"+email[0:4]+"-1")
	}
	tok1 := criarAluno("ana.lote@seno.dev")
	tok2 := criarAluno("bia.lote@seno.dev")

	entregar := func(tok string) int64 {
		t.Helper()
		w, _ := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/tentativa",
			"", autenticado(tok))
		if w.Code != http.StatusOK {
			t.Fatalf("tentativa: %d", w.Code)
		}
		w, sub := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/submissao",
			`{}`, autenticado(tok))
		if w.Code != http.StatusCreated {
			t.Fatalf("entrega: %d", w.Code)
		}
		return int64(sub["id"].(float64))
	}
	sub1 := entregar(tok1)
	sub2 := entregar(tok2)

	// Professor alheio: 404.
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/correcoes/publicar",
		`{}`, autenticado(tokZe))
	if w.Code != http.StatusNotFound {
		t.Errorf("escopo professor alheio: %d", w.Code)
	}

	// Sem confirmação: publica 0 (nenhuma confirmada).
	w, corpo := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/correcoes/publicar",
		`{}`, autenticado(tokPaula))
	if w.Code != http.StatusOK || int(corpo["publicadas"].(float64)) != 0 {
		t.Errorf("sem confirmadas: %d corpo %v", w.Code, corpo)
	}

	// Confirma somente a primeira → lote publica 1.
	w, _ = chama(t, r, "POST", "/v1/submissoes/"+intStr(sub1)+"/correcao/confirmar",
		`{}`, autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("confirmar: %d", w.Code)
	}
	w, corpo = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/correcoes/publicar",
		`{}`, autenticado(tokPaula))
	if w.Code != http.StatusOK || int(corpo["publicadas"].(float64)) != 1 {
		t.Errorf("lote: %d corpo %v", w.Code, corpo)
	}

	// Idempotência: reexecuções publicam 0.
	w, corpo = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/correcoes/publicar",
		`{}`, autenticado(tokPaula))
	if w.Code != http.StatusOK || int(corpo["publicadas"].(float64)) != 0 {
		t.Errorf("lote idempotente: %d corpo %v", w.Code, corpo)
	}

	// Ana publicada vê a correção (§Visibilidade); Bia não.
	w, visao := chama(t, r, "GET", "/v1/submissoes/"+intStr(sub1)+"/minha",
		"", autenticado(tok1))
	if w.Code != http.StatusOK {
		t.Errorf("aluno publicado: %d", w.Code)
	}
	if pub, _ := visao["publicada"].(bool); !pub {
		t.Errorf("minha: publicada=false %v", visao)
	}
	w, visao = chama(t, r, "GET", "/v1/submissoes/"+intStr(sub2)+"/minha",
		"", autenticado(tok2))
	if pub, _ := visao["publicada"].(bool); pub {
		t.Errorf("não publicada vazou: %v", visao)
	}
}
