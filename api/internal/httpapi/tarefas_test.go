package httpapi

import (
	"net/http"
	"testing"
)

// TestTarefas valida o banco de tarefas (PROJETO §Tarefas/§Teste):
// permissões (professor dono; staff tudo; admin puro não cria),
// CRUD com testes públicos/privados, duplicação e exclusão.
func TestTarefas(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "pedro.tarefas@seno.dev", "Pedro", "Alves")
	ativaNovoUsuario(t, r, pool, "pedro.tarefas@seno.dev", "senha-pedro-1")
	tokPedro := logar(t, r, "pedro.tarefas@seno.dev", "senha-pedro-1")

	// -------- Criação com testes (público + privado) --------
	w, tarefa := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Fibonacci","enunciado":"Imprima o n-ésimo termo.",
		"tempo_cpu_ms":1000,"tempo_total_ms":2000,"memoria_mb":64,
		"testes":[{"stdin":"5","stdout_esperado":"55","publico":true},
		          {"stdin":"10","stdout_esperado":"55","publico":false}]}`,
		autenticado(tokPedro))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar tarefa: status %d corpo %v", w.Code, tarefa)
	}
	idTarefa := int64(tarefa["id"].(float64))

	// Ver: dono vê testes (público e privado); fora do escopo → 404.
	w, corpo := chama(t, r, "GET", "/v1/tarefas/"+intStr(idTarefa), "", autenticado(tokPedro))
	if w.Code != http.StatusOK {
		t.Fatalf("ver tarefa: status %d corpo %v", w.Code, corpo)
	}
	if testes, _ := corpo["testes"].([]any); len(testes) != 2 {
		t.Errorf("deveria ter 2 testes: %v", corpo["testes"])
	}

	// Outro professor: cria a própria e não vê a de Pedro.
	criarProfessor(t, r, tokSuper, "rafa.tarefas@seno.dev", "Rafaela", "Costa")
	ativaNovoUsuario(t, r, pool, "rafa.tarefas@seno.dev", "senha-rafa-1")
	tokRafa := logar(t, r, "rafa.tarefas@seno.dev", "senha-rafa-1")

	w, _ = chama(t, r, "GET", "/v1/tarefas/"+intStr(idTarefa), "", autenticado(tokRafa))
	if w.Code != http.StatusNotFound {
		t.Errorf("tarefa de outro: 404 (sem revelar) → %d", w.Code)
	}
	w, corpo = chama(t, r, "GET", "/v1/tarefas", "", autenticado(tokRafa))
	if intOf(corpo["total"]) != 0 {
		t.Errorf("lista do não-dono deve ser vazia: %v", corpo["total"])
	}
	w, corpo = chama(t, r, "GET", "/v1/tarefas", "", autenticado(tokPedro))
	if intOf(corpo["total"]) != 1 {
		t.Errorf("lista do dono: %v", corpo["total"])
	}
	// Staff vê tudo.
	w, corpo = chama(t, r, "GET", "/v1/tarefas", "", autenticado(tokSuper))
	if intOf(corpo["total"]) != 1 {
		t.Errorf("staff vê todas: %v", corpo["total"])
	}

	// Admin puro não cria (dono precisa ser professor).
	w, _ = chama(t, r, "POST", "/v1/tarefas", `{"nome":"x","enunciado":"y"}`, autenticado(tokSuper))
	if w.Code != http.StatusForbidden {
		t.Errorf("super não cria tarefa: status %d", w.Code)
	}

	// Validações.
	w, falha := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"","enunciado":"sem nome"}`, autenticado(tokPedro))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("nome vazio: status %d corpo %v", w.Code, falha)
	}
	w, falha = chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"x","enunciado":"y","tempo_cpu_ms":-5}`, autenticado(tokPedro))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("tempo negativo: status %d", w.Code)
	}

	// Edição substitui os testes quando informados.
	w, corpo = chama(t, r, "PATCH", "/v1/tarefas/"+intStr(idTarefa), `{
		"nome":"Fibonacci (ajustada)","enunciado":"Imprima o n-ésimo termo.",
		"testes":[{"stdin":"7","stdout_esperado":"13","publico":true}]}`,
		autenticado(tokPedro))
	if w.Code != http.StatusOK {
		t.Fatalf("editar tarefa: status %d corpo %v", w.Code, corpo)
	}
	if testes, _ := corpo["testes"].([]any); len(testes) != 1 {
		t.Errorf("testes substituídos (1): %v", corpo["testes"])
	}

	// Duplicação: nova tarefa com testes copiados e nome de cópia.
	w, copia := chama(t, r, "POST", "/v1/tarefas/"+intStr(idTarefa)+"/duplicar",
		"", autenticado(tokPedro))
	if w.Code != http.StatusCreated {
		t.Fatalf("duplicar: status %d corpo %v", w.Code, copia)
	}
	idCopia := int64(copia["id"].(float64))
	if idCopia == idTarefa {
		t.Errorf("cópia precisa de id próprio")
	}
	w, corpo = chama(t, r, "GET", "/v1/tarefas/"+intStr(idCopia), "", autenticado(tokPedro))
	if testes, _ := corpo["testes"].([]any); len(testes) != 1 {
		t.Errorf("cópia mantém testes: %v", corpo["testes"])
	}

	// Exclusão: dono; depois 404.
	w, _ = chama(t, r, "DELETE", "/v1/tarefas/"+intStr(idCopia), "", autenticado(tokPedro))
	if w.Code != http.StatusNoContent {
		t.Errorf("excluir: status %d", w.Code)
	}
	w, _ = chama(t, r, "GET", "/v1/tarefas/"+intStr(idCopia), "", autenticado(tokPedro))
	if w.Code != http.StatusNotFound {
		t.Errorf("excluída é 404: %d", w.Code)
	}

	// Ids inválidos → 422.
	w, _ = chama(t, r, "GET", "/v1/tarefas/abc", "", autenticado(tokSuper))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("id inválido: status %d", w.Code)
	}
}
