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

	// Criação: professor e admin nascem pendentes com convite por e-mail.
	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"Paula.Prof@Seno.dev","first_name":"Paula","last_name":"Ribeiro","cargo":"professor"}`,
		autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar professor: status %d corpo %v", w.Code, body)
	}
	if email, _ := body["email"].(string); email != "paula.prof@seno.dev" {
		t.Errorf("e-mail deveria normalizar: %v", body["email"])
	}
	if status, _ := body["status"].(string); status != "pending" {
		t.Errorf("professor deveria nascer pendente: %v", body["status"])
	}
	if cargo, _ := body["roles"].([]any); len(cargo) != 1 || cargo[0] != "professor" {
		t.Errorf("cargos inesperados: %v", body["roles"])
	}
	idProf := int64(body["id"].(float64))
	paulaConvite := tokenDoLog(t, pool) // último e-mail: convite da Paula

	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"ana.admin@seno.dev","first_name":"Ana","last_name":"Costa","cargo":"admin"}`,
		autenticado(tokSuper))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar admin: status %d corpo %v", w.Code, body)
	}
	anaConvite := tokenDoLog(t, pool) // último e-mail: convite da Ana

	// Cargo inválidos nesta fase: student (com turmas depois) e super (único).
	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"x@seno.dev","first_name":"X","last_name":"Y","cargo":"student"}`,
		autenticado(tokSuper))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("student nesta fase: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"x@seno.dev","first_name":"X","last_name":"Y","cargo":"super"}`,
		autenticado(tokSuper))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("super extra: status %d corpo %v", w.Code, body)
	}

	// Convites aceites: Paula e Ana viram ativas com suas senhas.
	aceitarConvite(t, r, paulaConvite, "senha-da-paula-1")
	w, _ = chama(t, r, "POST", "/v1/auth/login", `{"email":"paula.prof@seno.dev","senha":"senha-da-paula-1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login da Paula ativa: status %d", w.Code)
	}
	aceitarConvite(t, r, anaConvite, "senha-da-ana-1")
	tokAdmin := logar(t, r, "ana.admin@seno.dev", "senha-da-ana-1")

	// Admin lista, mas não cria admin e não opera sobre admin.
	w, _ = chama(t, r, "GET", "/v1/users", "", autenticado(tokAdmin))
	if w.Code != http.StatusOK {
		t.Errorf("admin listar: status %d", w.Code)
	}
	w, body = chama(t, r, "POST", "/v1/users",
		`{"email":"n2@seno.dev","first_name":"N","last_name":"C","cargo":"admin"}`,
		autenticado(tokAdmin))
	if w.Code != http.StatusForbidden {
		t.Errorf("admin criar admin: status %d corpo %v", w.Code, body)
	}
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

	// Edição de dados: admin edita professora; super edita quem quiser.
	w, body = chama(t, r, "PATCH", "/v1/users/"+intStr(idProf),
		`{"first_name":"Paula","last_name":"Souza"}`, autenticado(tokAdmin))
	if w.Code != http.StatusOK {
		t.Fatalf("editar professor: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "GET", "/v1/users?busca=Souza", "", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Fatalf("busca pós-edit: status %d", w.Code)
	}
	lista, _ := body["usuarios"].([]any)
	if len(lista) != 1 {
		t.Errorf("busca por sobrenome editado: %v", body["usuarios"])
	}

	// Reset de senha pelo admin: 202 silencioso.
	w, _ = chama(t, r, "POST", "/v1/users/"+intStr(idProf)+"/resetar-senha", "", autenticado(tokAdmin))
	if w.Code != http.StatusAccepted {
		t.Errorf("resetar-senha: status %d", w.Code)
	}

	// Super não bloqueia a si (super é singleton e irreversível).
	w, body = chama(t, r, "GET", "/v1/auth/me", "", autenticado(tokSuper))
	idSuper := int64(body["id"].(float64))
	w, body = chama(t, r, "POST", "/v1/users/"+intStr(idSuper)+"/desativar", "", autenticado(tokSuper))
	if w.Code != http.StatusForbidden {
		t.Errorf("bloquear self/super: status %d corpo %v", w.Code, body)
	}

	// Alvo inexistente → 404; id inválido → 422.
	w, _ = chama(t, r, "PATCH", "/v1/users/9999", `{"first_name":"A","last_name":"B"}`, autenticado(tokSuper))
	if w.Code != http.StatusNotFound {
		t.Errorf("patch inexistente: status %d", w.Code)
	}
	w, _ = chama(t, r, "PATCH", "/v1/users/abc", `{"first_name":"A","last_name":"B"}`, autenticado(tokSuper))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("id inválido: status %d", w.Code)
	}
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
