package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestEnsinoTurmasMatriculas valida o domínio de ensino com foco nas
// permissões do §5.4: professor só as próprias turmas (404/escopo forçado),
// admin/super vê tudo; matrícula cria aluno pendente com convite.
func TestEnsinoTurmasMatriculas(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	// -------- Matérias (staff) --------
	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"prog1","nome":"Programação I"}`, autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar matéria: status %d corpo %v", w.Code, materia)
	}
	if codigo, _ := materia["codigo"].(string); codigo != "PROG1" {
		t.Errorf("código deveria normalizar para PROG1: %v", materia["codigo"])
	}
	idMateria := int64(materia["id"].(float64))

	w, _ = chama(t, r, "POST", "/v1/materias",
		`{"codigo":"PROG1","nome":"Outra"}`, autenticado(tokSuper))
	if w.Code != http.StatusConflict {
		t.Errorf("matéria duplicada: status %d", w.Code)
	}

	w, _ = chama(t, r, "PATCH", "/v1/materias/"+intStr(idMateria),
		`{"codigo":"PROG1","nome":"Programação I (nova grade)"}`, autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Errorf("editar matéria: status %d", w.Code)
	}

	// -------- Professores (nascem pendentes; aceite via convite) --------
	criarProfessor(t, r, tokSuper, "paula.ensino@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.ensino@seno.dev", "senha-paula-1")
	tokPaula := logar(t, r, "paula.ensino@seno.dev", "senha-paula-1")

	criarProfessor(t, r, tokSuper, "bruno.ensino@seno.dev", "Bruno", "Lopes")
	ativaNovoUsuario(t, r, pool, "bruno.ensino@seno.dev", "senha-bruno-1")
	tokBruno := logar(t, r, "bruno.ensino@seno.dev", "senha-bruno-1")

	// -------- Turmas --------
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":1,"ano":2026}`, autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar período: status %d corpo %v", w.Code, periodo)
	}
	idPeriodo := int64(periodo["id"].(float64))

	w, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"T1"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar turma: status %d corpo %v", w.Code, turma)
	}
	if mat, _ := turma["materia"].(map[string]any); mat["nome"] != "Programação I (nova grade)" {
		t.Errorf("materia na turma: %v", turma["materia"])
	}
	idTurma := int64(turma["id"].(float64))

	// Dona: lista 1; Bruno (não-dono): lista 0 e ver → 404.
	w, body := chama(t, r, "GET", "/v1/turmas", "", autenticado(tokPaula))
	if w.Code != http.StatusOK || intOf(body["total"]) != 1 {
		t.Errorf("dona deve listar a própria: %d %v", w.Code, body["total"])
	}
	w, body = chama(t, r, "GET", "/v1/turmas", "", autenticado(tokBruno))
	if w.Code != http.StatusOK || intOf(body["total"]) != 0 {
		t.Errorf("não-dono deve listar 0: %d %v", w.Code, body["total"])
	}
	w, body = chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma), "", autenticado(tokBruno))
	if w.Code != http.StatusNotFound || falhaDoEnvelope(t, body) != "NAO_ENCONTRADO" {
		t.Errorf("turma de outro: 404 (sem revelar): %d %v", w.Code, body)
	}

	// Staff: só a dele... na verdade: staff vê TUDO (não-dono?? staff vê a
	// turma do outro); filtro por professor_id funciona.
	w, body = chama(t, r, "GET", "/v1/turmas", "", autenticado(tokSuper))
	if intOf(body["total"]) != 1 {
		t.Errorf("staff deve ver todas: %v", body["total"])
	}
	w, _ = chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`}`,
		autenticado(tokSuper))
	if w.Code != http.StatusForbidden {
		t.Errorf("admin puro não cria turma: status %d", w.Code)
	}

	// -------- Matrículas --------
	// Professor não pode ser matriculado como aluno: 422.
	w, body = chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"bruno.ensino@seno.dev"}`, autenticado(tokPaula))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("matricular professor: status %d corpo %v", w.Code, body)
	}

	// Aluna inexistente nasce pendente + convite + matrícula.
	w, mat := chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"carla.aluna@seno.dev","first_name":"Carla","last_name":"Souza"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("matricular aluna nova: status %d corpo %v", w.Code, mat)
	}
	idMat := int64(mat["id"].(float64))
	if aluno, _ := mat["aluno"].(map[string]any); aluno["status"] != "pending" {
		t.Errorf("aluna nova pendente: %v", mat["aluno"])
	}
	tokConvite := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConvite, "senha-carla-1")
	tokCarla := logar(t, r, "carla.aluna@seno.dev", "senha-carla-1")

	// Aluna autenticada não lista turmas (rota exige professor/staff).
	w, body = chama(t, r, "GET", "/v1/turmas", "", autenticado(tokCarla))
	if w.Code != http.StatusForbidden || falhaDoEnvelope(t, body) != "SEM_PERMISSAO" {
		t.Errorf("aluna listar turmas: status %d corpo %v", w.Code, body)
	}

	// Dona vê matrículas; staff vê de qualquer turma.
	w, body = chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		"", autenticado(tokPaula))
	if w.Code != http.StatusOK || intOf(body["total"]) != 1 {
		t.Errorf("matriculas da dona: %d %v", w.Code, body["total"])
	}
	w, body = chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		"", autenticado(tokSuper))
	if w.Code != http.StatusOK || intOf(body["total"]) != 1 {
		t.Errorf("staff vê matrículas alheias: %d %v", w.Code, body["total"])
	}

	// Encerrar matrícula (= "remover aluno"): staff ou dona.
	w, _ = chama(t, r, "DELETE", "/v1/matriculas/"+intStr(idMat), "", autenticado(tokSuper))
	if w.Code != http.StatusNoContent {
		t.Errorf("encerrar matrícula: status %d", w.Code)
	}
	w, body = chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		"", autenticado(tokPaula))
	if intOf(body["total"]) != 0 {
		t.Errorf("pós-encerramento: ativas = %v", body["total"])
	}
	w, body = chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma)+"/matriculas?incluir_encerradas=true",
		"", autenticado(tokPaula))
	if intOf(body["total"]) != 1 {
		t.Errorf("com encerradas: total = %v", body["total"])
	}

	// Encerramento e travas da turma.
	w, _ = chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/encerrar", "", autenticado(tokPaula))
	if w.Code != http.StatusNoContent {
		t.Errorf("encerrar turma: status %d", w.Code)
	}
	w, body = chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/encerrar", "", autenticado(tokPaula))
	if w.Code != http.StatusConflict {
		t.Errorf("encerrar duplo: status %d", w.Code)
	}
	w, body = chama(t, r, "PATCH", "/v1/turmas/"+intStr(idTurma),
		`{"titulo":"T2"}`, autenticado(tokPaula))
	if w.Code != http.StatusConflict {
		t.Errorf("editar turma encerrada: status %d", w.Code)
	}
	w, body = chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"carla.aluna@seno.dev"}`, autenticado(tokPaula))
	if w.Code != http.StatusConflict {
		t.Errorf("matricular em turma encerrada: status %d", w.Code)
	}

	// Exclusão: com matrícula encerrada (histórico) ainda é possível; 404 depois.
	w, _ = chama(t, r, "DELETE", "/v1/turmas/"+intStr(idTurma), "", autenticado(tokPaula))
	if w.Code != http.StatusNoContent {
		t.Errorf("excluir turma: status %d", w.Code)
	}
	w, _ = chama(t, r, "GET", "/v1/turmas/"+intStr(idTurma), "", autenticado(tokSuper))
	if w.Code != http.StatusNotFound {
		t.Errorf("turma excluída ainda visível: status %d", w.Code)
	}

	// Ids inválidos → 422.
	w, _ = chama(t, r, "GET", "/v1/turmas/abc", "", autenticado(tokSuper))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("id inválido: status %d", w.Code)
	}
}

// TestEnsinoFiltros valida encerradas=only/all, periodo_id e a busca de
// alunos (GET /v1/alunos, professor+staff).
func TestEnsinoFiltros(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"GRAF","nome":"Grafos"}`, autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar matéria: status %d corpo %v", w.Code, materia)
	}
	idMateria := int64(materia["id"].(float64))

	criarProfessor(t, r, tokSuper, "hilda.filtros@seno.dev", "Hilda", "Maia")
	ativaNovoUsuario(t, r, pool, "hilda.filtros@seno.dev", "senha-hilda-1")
	tokProf := logar(t, r, "hilda.filtros@seno.dev", "senha-hilda-1")

	// Períodos dos testes: cadastrados pelo admin (super), como na vida real.
	w, per2026 := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":1,"ano":2026}`, autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("período 2026.1: status %d corpo %v", w.Code, per2026)
	}
	idPer2026 := int64(per2026["id"].(float64))
	w, per2025 := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":2,"ano":2025}`, autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("período 2025.2: status %d corpo %v", w.Code, per2025)
	}
	idPer2025 := int64(per2025["id"].(float64))

	// Duas turmas: uma ativa, uma encerrada.
	w, ativa := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPer2026)+`,"titulo":"A"}`,
		autenticado(tokProf))
	if w.Code != http.StatusCreated {
		t.Fatalf("turma ativa: status %d corpo %v", w.Code, ativa)
	}
	idAtiva := int64(ativa["id"].(float64))

	w, encerrada := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPer2025)+`,"titulo":"E"}`,
		autenticado(tokProf))
	if w.Code != http.StatusCreated {
		t.Fatalf("turma a encerrar: status %d corpo %v", w.Code, encerrada)
	}
	idEncerrada := int64(encerrada["id"].(float64))
	w, _ = chama(t, r, "POST", "/v1/turmas/"+intStr(idEncerrada)+"/encerrar",
		"", autenticado(tokProf))
	if w.Code != http.StatusNoContent {
		t.Fatalf("encerrar: status %d", w.Code)
	}

	// Padrão: só ativas (1).
	w, body := chama(t, r, "GET", "/v1/turmas", "", autenticado(tokProf))
	if intOf(body["total"]) != 1 {
		t.Errorf("padrão ativas: %v", body["total"])
	}
	// only: só a encerrada.
	w, body = chama(t, r, "GET", "/v1/turmas?encerradas=only", "", autenticado(tokProf))
	if intOf(body["total"]) != 1 {
		t.Errorf("encerradas=only: %v", body["total"])
	}
	if turmas, _ := body["turmas"].([]any); len(turmas) == 1 {
		item, _ := turmas[0].(map[string]any)
		if id, _ := item["id"].(float64); int64(id) != idEncerrada {
			t.Errorf("only devolve a encerrada: %v", item["id"])
		}
	}
	// all: 2.
	w, body = chama(t, r, "GET", "/v1/turmas?encerradas=all", "", autenticado(tokProf))
	if intOf(body["total"]) != 2 {
		t.Errorf("encerradas=all: %v", body["total"])
	}
	// Valor inválido → 422.
	w, _ = chama(t, r, "GET", "/v1/turmas?encerradas=zanzar", "", autenticado(tokProf))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("encerradas inválido: status %d", w.Code)
	}
	// periodo_id filtra: id do período lido direto do banco.
	var idPeriodo int64
	if err := pool.QueryRow(context.Background(),
		`SELECT periodo_id FROM turmas WHERE id = $1`, idAtiva).Scan(&idPeriodo); err != nil {
		t.Fatalf("periodo: %v", err)
	}
	w, body = chama(t, r, "GET",
		"/v1/turmas?encerradas=all&periodo_id="+intStr(idPeriodo), "", autenticado(tokProf))
	if intOf(body["total"]) != 1 {
		t.Errorf("periodo_id deveria devolver 1 turma: %v", body["total"])
	}

	// Busca de alunos: professor pode buscar (cargo student fixo no filtro).
	w, body = chama(t, r, "GET", "/v1/alunos?busca=aluna", "", autenticado(tokProf))
	if w.Code != http.StatusOK {
		t.Errorf("busca alunos: status %d corpo %v", w.Code, body)
	}

	// Professor não acessa escrita de matérias.
	w, _ = chama(t, r, "POST", "/v1/materias",
		`{"codigo":"X1","nome":"X"}`, autenticado(tokProf))
	if w.Code != http.StatusForbidden {
		t.Errorf("professor cria matéria: status %d", w.Code)
	}
}

// ---------------------------------------------------------------

// intOf converte o JSON numérico do corpo para int (robusto a nil).
func intOf(v any) int {
	if n, ok := v.(float64); ok {
		return int(n)
	}
	return -1
}

// criarProfessor via gestao de usuários (super): nasce pendente + convite.
func criarProfessor(t *testing.T, r *gin.Engine, tokSuper, email, nome, sobrenome string) {
	t.Helper()
	w, body := chama(t, r, "POST", "/v1/users",
		`{"email":"`+email+`","first_name":"`+nome+`","last_name":"`+sobrenome+`","cargo":"professor"}`,
		autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar professor %s: status %d corpo %v", email, w.Code, body)
	}
}

// ativaNovoUsuario consome o convite capturado no Log (da conta criada
// por último).
func ativaNovoUsuario(t *testing.T, r *gin.Engine, pool *pgxpool.Pool, email, senha string) {
	t.Helper()
	tokConvite := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConvite, senha)
}
