package httpapi

import (
	"net/http"
	"testing"
)

// TestAnalyticsRotasEPermissoes valida as permissões e retornos dos endpoints de analytics:
// - /v1/turmas/:id/analytics
// - /v1/atribuicoes/:id/analytics
// - /v1/turmas/:id/alunos/:aluno_id/analytics
// - /v1/tarefas/:id/analytics
// - /v1/admin/metricas
func TestAnalyticsRotasEPermissoes(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "prof.a1@seno.dev", "Ana", "Lima")
	ativaNovoUsuario(t, r, pool, "prof.a1@seno.dev", "senha-prof-a1")
	tokProf1 := logar(t, r, "prof.a1@seno.dev", "senha-prof-a1")

	criarProfessor(t, r, tokSuper, "prof.a2@seno.dev", "Bruno", "Rocha")
	ativaNovoUsuario(t, r, pool, "prof.a2@seno.dev", "senha-prof-a2")
	tokProf2 := logar(t, r, "prof.a2@seno.dev", "senha-prof-a2")

	// Criar estrutura da turma para Prof1
	_, mat := chama(t, r, "POST", "/v1/materias", `{"codigo":"AN1","nome":"Analytics 1"}`, autenticado(tokSuper))
	idMat := int64(mat["id"].(float64))
	_, per := chama(t, r, "POST", "/v1/periodos", `{"trimestre":1,"ano":2027}`, autenticado(tokSuper))
	idPer := int64(per["id"].(float64))

	_, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMat)+`,"periodo_id":`+intStr(idPer)+`,"titulo":"Turma Analytics"}`,
		autenticado(tokProf1))
	idTurma := int64(turma["id"].(float64))

	// Tarefa e atividade
	_, tar := chama(t, r, "POST", "/v1/tarefas", `{"nome":"Tarefa An","enunciado":"..."}`, autenticado(tokProf1))
	idTar := int64(tar["id"].(float64))
	conteudo := `{"schema_version":1,"components":[{"type":"task_default","id":` + intStr(idTar) + `,"value_pts":10,"language":"python"}]}`
	_, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Atividade An","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTar)+`,"ordem":1,"valor_pts":10,"linguagem":"python"}]}`,
		autenticado(tokProf1))
	idAtv := int64(atv["id"].(float64))

	_, atr := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtv)+`}`,
		autenticado(tokProf1))
	idAtr := int64(atr["id"].(float64))

	// Matricular aluno
	chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"aluno.an@seno.dev","first_name":"Lucas","last_name":"Melo"}`,
		autenticado(tokProf1))
	tokConv := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConv, "senha-aluno-an")
	tokAluno := logar(t, r, "aluno.an@seno.dev", "senha-aluno-an")

	// 1. GET /v1/turmas/:id/analytics
	// Aluno não pode -> 403
	w, _ := chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma)+"/analytics", "", autenticado(tokAluno))
	if w.Code != http.StatusForbidden {
		t.Errorf("aluno em turma analytics: esperava 403, obteve %d", w.Code)
	}
	// Prof2 (não é dono) -> 404
	w, _ = chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma)+"/analytics", "", autenticado(tokProf2))
	if w.Code != http.StatusNotFound {
		t.Errorf("prof2 em turma de prof1: esperava 404, obteve %d", w.Code)
	}
	// Prof1 (dono) -> 200
	w, corpo := chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma)+"/analytics", "", autenticado(tokProf1))
	if w.Code != http.StatusOK {
		t.Fatalf("prof1 dono: esperava 200, obteve %d: %v", w.Code, corpo)
	}
	if int64(corpo["turma_id"].(float64)) != idTurma || int64(corpo["total_matriculados"].(float64)) != 1 {
		t.Errorf("corpo turma analytics inesperado: %v", corpo)
	}

	// 2. GET /v1/atribuicoes/:id/analytics
	// Prof1 -> 200
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr)+"/analytics", "", autenticado(tokProf1))
	if w.Code != http.StatusOK {
		t.Fatalf("atribuicao analytics: esperava 200, obteve %d: %v", w.Code, corpo)
	}

	// 3. GET /v1/tarefas/:id/analytics
	// Prof1 -> 200
	w, corpo = chama(t, r, "GET", "/v1/tarefas/"+intStr(idTar)+"/analytics", "", autenticado(tokProf1))
	if w.Code != http.StatusOK {
		t.Fatalf("tarefa analytics: esperava 200, obteve %d: %v", w.Code, corpo)
	}

	// 4. GET /v1/admin/metricas
	// Prof1 (não staff) -> 403
	w, _ = chama(t, r, "GET", "/v1/admin/metricas", "", autenticado(tokProf1))
	if w.Code != http.StatusForbidden {
		t.Errorf("professor em admin metricas: esperava 403, obteve %d", w.Code)
	}
	// Super (staff) -> 200
	w, corpo = chama(t, r, "GET", "/v1/admin/metricas", "", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Fatalf("super em admin metricas: esperava 200, obteve %d: %v", w.Code, corpo)
	}
	if _, ok := corpo["usuarios_por_cargo"]; !ok {
		t.Errorf("admin metricas faltando usuarios_por_cargo: %v", corpo)
	}
}
