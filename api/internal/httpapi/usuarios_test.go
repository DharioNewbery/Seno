package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/auth"
)

func TestCrudUsuarios(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	// Bootstrap cria o único super admin (ator da gestão neste teste).
	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	// Lista vazia além do super: apenas ele.
	w, body := chama(t, r, "GET", "/v1/users", "", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Fatalf("listagem: status %d corpo %v", w.Code, body)
	}
	if total, _ := body["total"].(float64); int(total) != 1 {
		t.Errorf("total esperado 1, obtido %v", body["total"])
	}

	// Criação: usuário + pessoa nascem pendentes, SEM cargos.
	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"Paula.Prof@Seno.dev","first_name":"Paula","last_name":"Ribeiro"}`,
		autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar usuário: status %d corpo %v", w.Code, body)
	}
	if email, _ := body["email"].(string); email != "paula.prof@seno.dev" {
		t.Errorf("e-mail deveria normalizar: %v", body["email"])
	}
	if status, _ := body["status"].(string); status != "pending" {
		t.Errorf("usuário deveria nascer pendente: %v", body["status"])
	}
	if cargo, _ := body["roles"].([]any); len(cargo) != 0 {
		t.Errorf("cargos inesperados: %v", body["roles"])
	}
	idProf := int64(body["id"].(float64))
	paulaConvite := tokenDoLog(t, pool) // último e-mail: convite da Paula

	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"ana.admin@seno.dev","first_name":"Ana","last_name":"Costa"}`,
		autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar usuário: status %d corpo %v", w.Code, body)
	}
	anaConvite := tokenDoLog(t, pool) // último e-mail: convite da Ana
	idAna := buscaIDPorEmail(t, r, tokSuper, "ana.admin@seno.dev")

	// Super atribui cargos pela inspeção (PUT /cargos).
	w, body = chama(t, r, "PUT", "/v1/users/"+intStr(idProf)+"/cargos",
		`{"cargos":["professor"]}`, autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Fatalf("cargo da Paula: status %d corpo %v", w.Code, body)
	}
	if cargo, _ := body["roles"].([]any); len(cargo) != 1 || cargo[0] != "professor" {
		t.Errorf("cargo esperado professor: %v", body["roles"])
	}

	// Super transforma a Ana em admin pela inspeção.
	w, body = chama(t, r, "PUT", "/v1/users/"+intStr(idAna)+"/cargos",
		`{"cargos":["admin"]}`, autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Fatalf("cargo da Ana: status %d corpo %v", w.Code, body)
	}

	// Super inspeção: GET /v1/users/:id.
	w, body = chama(t, r, "GET", "/v1/users/"+intStr(idProf), "", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Fatalf("inspeção da Paula: status %d corpo %v", w.Code, body)
	}

	// Convites aceites: Paula e Ana viram ativas com suas senhas.
	aceitarConvite(t, r, paulaConvite, "senha-da-paula-1")
	w, _ = chama(t, r, "POST", "/v1/auth/login",
		`{"email":"paula.prof@seno.dev","senha":"senha-da-paula-1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login da Paula: status %d", w.Code)
	}
	aceitarConvite(t, r, anaConvite, "senha-da-ana-1")
	tokAdmin := logar(t, r, "ana.admin@seno.dev", "senha-da-ana-1")

	// Inspeção do super: ninguém, nem a Ana admin.
	w, body = chama(t, r, "GET", "/v1/auth/me", "", autenticado(tokSuper))
	idSuper := int64(body["id"].(float64))
	w, body = chama(t, r, "GET", "/v1/users/"+intStr(idSuper), "", autenticado(tokAdmin))
	if w.Code != http.StatusForbidden {
		t.Errorf("admin inspeciona super: status %d corpo %v", w.Code, body)
	}
	// O super se inspeciona.
	w, _ = chama(t, r, "GET", "/v1/users/"+intStr(idSuper), "", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Errorf("super se inspeciona: status %d", w.Code)
	}
	// A Ana se inspeciona.
	w, _ = chama(t, r, "GET", "/v1/users/"+intStr(idAna), "", autenticado(tokAdmin))
	if w.Code != http.StatusOK {
		t.Errorf("admin se inspeciona: status %d", w.Code)
	}

	// Admin lista: vê Paula e a si, mas não o super.
	w, body = chama(t, r, "GET", "/v1/users", "", autenticado(tokAdmin))
	if w.Code != http.StatusOK {
		t.Fatalf("admin listar: status %d", w.Code)
	}
	lista, _ := body["usuarios"].([]any)
	if int(body["total"].(float64)) != len(lista) || len(lista) != 2 {
		t.Errorf("admin deveria listar 2 (sem super): %v", body)
	}

	// Admin cria usuários (sem cargos): permitido.
	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"joao.aluno@seno.dev","first_name":"João","last_name":"Lima"}`,
		autenticado(tokAdmin))
	if w.Code != http.StatusCreated {
		t.Fatalf("admin cria usuário: status %d corpo %v", w.Code, body)
	}
	idJoao := int64(body["id"].(float64))

	// Admin não atribui admin; só super.
	w, body = chama(t, r, "PUT", "/v1/users/"+intStr(idJoao)+"/cargos",
		`{"cargos":["admin"]}`, autenticado(tokAdmin))
	if w.Code != http.StatusForbidden {
		t.Errorf("admin atribui admin: status %d corpo %v", w.Code, body)
	}
	// Admin atribui student/professor: OK.
	w, body = chama(t, r, "PUT", "/v1/users/"+intStr(idJoao)+"/cargos",
		`{"cargos":["student","professor"]}`, autenticado(tokAdmin))
	if w.Code != http.StatusOK {
		t.Fatalf("admin atribui student+professor: status %d corpo %v", w.Code, body)
	}
	if cargo, _ := body["roles"].([]any); len(cargo) != 2 {
		t.Errorf("dois cargos esperados: %v", body["roles"])
	}

	// Admin altera cargos de professora; super altera quem quiser.
	w, body = chama(t, r, "PATCH", "/v1/users/"+intStr(idProf),
		`{"first_name":"Paula","last_name":"Souza"}`, autenticado(tokAdmin))
	if w.Code != http.StatusOK {
		t.Fatalf("editar professor: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "GET", "/v1/users?busca=Souza", "", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Fatalf("busca pós-edit: status %d", w.Code)
	}
	lista, _ = body["usuarios"].([]any)
	if len(lista) != 1 {
		t.Errorf("busca por sobrenome editado: %v", body["usuarios"])
	}

	// Ninguém remove o próprio cargo de admin.
	w, body = chama(t, r, "PUT", "/v1/users/"+intStr(idAna)+"/cargos",
		`{"cargos":["professor"]}`, autenticado(tokAdmin))
	if w.Code != http.StatusForbidden {
		t.Errorf("admin remove o próprio admin: status %d corpo %v", w.Code, body)
	}
	// Nem o super remove o próprio super.
	w, body = chama(t, r, "PUT", "/v1/users/"+intStr(idSuper)+"/cargos",
		`{"cargos":["admin"]}`, autenticado(tokSuper))
	if w.Code != http.StatusForbidden {
		t.Errorf("super remove o próprio super: status %d corpo %v", w.Code, body)
	}

	// Admin bloqueia/desbloqueia professora.
	w, _ = chama(t, r, "POST", "/v1/users/"+intStr(idProf)+"/desativar", "", autenticado(tokAdmin))
	if w.Code != http.StatusNoContent {
		t.Errorf("admin bloquear professor: status %d", w.Code)
	}
	w, body = chama(t, r, "POST", "/v1/auth/login",
		`{"email":"paula.prof@seno.dev","senha":"senha-da-paula-1"}`)
	if w.Code != http.StatusForbidden || falhaDoEnvelope(t, body) != "SEM_PERMISSAO" {
		t.Errorf("login bloqueado (senha correta): status %d corpo %v", w.Code, body)
	}
	w, _ = chama(t, r, "POST", "/v1/users/"+intStr(idProf)+"/ativar", "", autenticado(tokAdmin))
	if w.Code != http.StatusNoContent {
		t.Errorf("desbloqueio: status %d", w.Code)
	}

	// Reset de senha: admin sobre professor/aluno; nunca sobre admin.
	w, _ = chama(t, r, "POST", "/v1/users/"+intStr(idProf)+"/resetar-senha", "", autenticado(tokAdmin))
	if w.Code != http.StatusAccepted {
		t.Errorf("resetar-senha professor: status %d", w.Code)
	}
	w, _ = chama(t, r, "POST", "/v1/users/"+intStr(idJoao)+"/resetar-senha", "", autenticado(tokAdmin))
	if w.Code != http.StatusAccepted {
		t.Errorf("resetar-senha aluno: status %d", w.Code)
	}
	w, _ = chama(t, r, "POST", "/v1/users/"+intStr(idAna)+"/resetar-senha", "", autenticado(tokAdmin))
	if w.Code != http.StatusForbidden {
		t.Errorf("admin resetar a própria senha: status %d", w.Code)
	}
	w, _ = chama(t, r, "POST", "/v1/users/"+intStr(idSuper)+"/resetar-senha", "", autenticado(tokSuper))
	if w.Code != http.StatusAccepted {
		t.Errorf("super resetar a própria senha: status %d", w.Code)
	}

	// Super não bloqueia a si; e bloquear super (nãoStaff) nem existe fora do self.
	w, body = chama(t, r, "POST", "/v1/users/"+intStr(idSuper)+"/desativar", "", autenticado(tokSuper))
	if w.Code != http.StatusForbidden {
		t.Errorf("bloquear self/super: status %d corpo %v", w.Code, body)
	}

	// Super atira no próprio pé? Não: sem remover o super (testado antes).

	// Alvo inexistente → 404; id inválido → 422.
	w, _ = chama(t, r, "PATCH", "/v1/users/9999", `{"first_name":"A","last_name":"B"}`, autenticado(tokSuper))
	if w.Code != http.StatusNotFound {
		t.Errorf("patch inexistente: status %d", w.Code)
	}
	w, _ = chama(t, r, "PATCH", "/v1/users/abc", `{"first_name":"A","last_name":"B"}`, autenticado(tokSuper))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("id inválido: status %d", w.Code)
	}
	w, _ = chama(t, r, "GET", "/v1/users/9999", "", autenticado(tokSuper))
	if w.Code != http.StatusNotFound {
		t.Errorf("get inexistente: status %d", w.Code)
	}
}

// buscaIDPorEmail lista com busca e devolve o ID do primeiro resultado.
func buscaIDPorEmail(t *testing.T, r *gin.Engine, token, busca string) int64 {
	t.Helper()
	w, body := chama(t, r, "GET", "/v1/users?busca="+busca, "", autenticado(token))
	if w.Code != http.StatusOK {
		t.Fatalf("busca por %s: status %d corpo %v", busca, w.Code, body)
	}
	usuarios, _ := body["usuarios"].([]any)
	if len(usuarios) == 0 {
		t.Fatalf("busca por %s sem resultados: %v", busca, body)
	}
	return int64(usuarios[0].(map[string]any)["id"].(float64))
}

// bootstrapSuper cria o super admin padrão do banco de teste.
func bootstrapSuper(t *testing.T, deps *Dependencies) error {
	t.Helper()
	return auth.BootstrapSuperAdmin(
		context.Background(), deps.Store, "super@seno.dev", "102938")
}

// logar entra com credenciais e devolve o token de sessão.
func logar(t *testing.T, r *gin.Engine, email, senha string) string {
	t.Helper()
	w, body := chama(t, r, "POST", "/v1/auth/login",
		`{"email":"`+email+`","senha":"`+senha+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: status %d corpo %v", email, w.Code, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("login sem token")
	}
	return token
}

// aceitarConvite consome o link de convite com a senha indicada.
func aceitarConvite(t *testing.T, r *gin.Engine, token, senha string) {
	t.Helper()
	w, body := chama(t, r, "POST", "/v1/auth/invite/accept",
		`{"token":"`+token+`","senha":"`+senha+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("aceite: status %d corpo %v", w.Code, body)
	}
}

// intStr formata ids para a rota.
func intStr(id int64) string { return strconv.FormatInt(id, 10) }
