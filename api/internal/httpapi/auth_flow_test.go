package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/api/db"
	"github.com/seno-project/seno/api/internal/atividades"
	"github.com/seno-project/seno/api/internal/auth"
	"github.com/seno-project/seno/api/internal/ensino"
	"github.com/seno-project/seno/api/internal/mail"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/tarefas"
	"github.com/seno-project/seno/api/internal/usuarios"
)

// testeDB conecta ao Postgres de teste (CI ou docker local: banco seno_test).
// Aplica as migrações e limpa as tabelas no início do teste.
func testeDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SENO_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://seno:seno@localhost:5432/seno_test?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	if err := platform.MigrateUp(ctx, pool, db.Migrations); err != nil {
		t.Fatalf("migrações: %v", err)
	}
	_, err = pool.Exec(ctx, `TRUNCATE persons, users, user_roles, sessions,
		one_time_tokens, log_entries, jobs, tarefas, testes_tarefa,
		atividades, atividade_tarefas RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("limpeza do banco: %v", err)
	}
	return pool
}

// montaAPI constrói o roteador completo com serviços reais (sem SMTP).
func montaAPI(t *testing.T, pool *pgxpool.Pool) (*Dependencies, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	audit := platform.NewAudit(pool)
	st := store.New(pool)
	cfg := &platform.Config{WebOrigin: "http://localhost:5173"}
	deps := &Dependencies{
		Cfg:   cfg,
		Pool:  pool,
		Store: st,
		Audit: audit,
		Auth:  auth.New(st, audit, time.Minute, time.Hour, cfg.WebOrigin),
		Cargo: auth.NewRecoverer(st, audit, &mail.LogSender{Audit: audit}, cfg.WebOrigin),
	}
	deps.UsuarioS = usuarios.New(st, audit, deps.Cargo)
	deps.Ensino = ensino.New(st, audit, deps.Cargo)
	deps.TarefaS = tarefas.New(st, audit)
	deps.AtividadeS = atividades.New(st, audit, deps.TarefaS)
	r := gin.New()
	RegisterRoutes(r, deps)
	return deps, r
}

// chama executa uma requisição JSON com a origem do BFF (OriginGuard) e
// headers extras opcionais (ex.: Authorization).
func chama(t *testing.T, r *gin.Engine, method, path, body string, headers ...map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5173")
	for _, hs := range headers {
		for k, v := range hs {
			req.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if len(w.Body.Bytes()) > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("resposta não é JSON (%s %s): %v", method, path, err)
		}
	}
	return w, out
}

// autenticado devolve headers de chamadas autenticadas.
func autenticado(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// falhaDoEnvelope extrai o código de erro do envelope único.
func falhaDoEnvelope(t *testing.T, body map[string]any) string {
	t.Helper()
	erro, ok := body["erro"].(map[string]any)
	if !ok {
		t.Fatalf("envelope de erro ausente: %v", body)
	}
	code, _ := erro["codigo"].(string)
	return code
}

var padraoToken = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// tokenDoLog extrai o token do último e-mail capturado no Log (fallback).
func tokenDoLog(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var detail string
	err := pool.QueryRow(context.Background(),
		`SELECT detail FROM log_entries WHERE kind = 'mail.fallback' ORDER BY id DESC LIMIT 1`,
	).Scan(&detail)
	if err != nil {
		t.Fatalf("e-mail de reset não capturado no Log: %v", err)
	}
	m := padraoToken.FindStringSubmatch(detail)
	if m == nil {
		t.Fatalf("token ausente no e-mail capturado: %q", detail)
	}
	return m[1]
}

func TestAuthFundacao(t *testing.T) {
	pool := testeDB(t)
	_, r := montaAPI(t, pool)
	ctx := context.Background()

	st := store.New(pool)
	audit := platform.NewAudit(pool)

	// Bootstrap cria o super admin e é idempotente (senha nova não sobrescreve).
	if err := auth.BootstrapSuperAdmin(ctx, st, "super@seno.dev", "102938"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := auth.BootstrapSuperAdmin(ctx, st, "super@seno.dev", "outra-senha"); err != nil {
		t.Fatalf("bootstrap idempotente: %v", err)
	}

	// Login correto → 200 {token, pessoa, cargos}.
	w, body := chama(t, r, "POST", "/v1/auth/login", `{"email":"super@seno.dev","senha":"102938"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login correto: status %d corpo %v", w.Code, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("login sem token")
	}
	pessoa, _ := body["pessoa"].(map[string]any)
	if pessoa["email"] != "super@seno.dev" || pessoa["first_name"] != "Super" {
		t.Errorf("pessoa inesperada: %v", pessoa)
	}
	cargos, _ := body["cargos"].([]any)
	if len(cargos) != 1 || cargos[0] != "super" {
		t.Errorf("cargos inesperados: %v", cargos)
	}

	// Login com senha errada e com e-mail desconhecido → 401 genérico.
	w, body = chama(t, r, "POST", "/v1/auth/login", `{"email":"super@seno.dev","senha":"errada"}`)
	if w.Code != http.StatusUnauthorized || falhaDoEnvelope(t, body) != "SESSAO_INVALIDA" {
		t.Errorf("senha errada: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "POST", "/v1/auth/login", `{"email":"ninguem@seno.dev","senha":"x"}`)
	if w.Code != http.StatusUnauthorized || falhaDoEnvelope(t, body) != "SESSAO_INVALIDA" {
		t.Errorf("e-mail desconhecido: status %d corpo %v", w.Code, body)
	}

	// /me sem sessão → 401; com sessão → 200 com cargos revalidados.
	w, _ = chama(t, r, "GET", "/v1/auth/me", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("/me sem token: status %d", w.Code)
	}
	w2, meBody := chama(t, r, "GET", "/v1/auth/me", "", autenticado(token))
	if w2.Code != http.StatusOK {
		t.Fatalf("/me autenticado: status %d corpo %s", w2.Code, w2.Body.String())
	}
	if meBody["email"] != "super@seno.dev" {
		t.Errorf("/me e-mail inesperado: %v", meBody["email"])
	}
	roles, _ := meBody["roles"].([]any)
	if len(roles) != 1 || roles[0] != "super" {
		t.Errorf("/me cargos inesperados: %v", roles)
	}

	// Logout encerra a sessão; token morre na hora.
	w2, _ = chama(t, r, "POST", "/v1/auth/logout", "", autenticado(token))
	if w2.Code != http.StatusNoContent {
		t.Fatalf("logout: status %d corpo %s", w2.Code, w2.Body.String())
	}
	w2, _ = chama(t, r, "GET", "/v1/auth/me", "", autenticado(token))
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("/me pós-logout: status %d", w2.Code)
	}

	// Recuperação: pedido silencioso; token por e-mail (fallback no Log).
	w, body = chama(t, r, "POST", "/v1/auth/password/reset-request", `{"email":"super@seno.dev"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("reset-request: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "POST", "/v1/auth/password/reset-request", `{"email":"fantasma@seno.dev"}`)
	if w.Code != http.StatusAccepted {
		t.Errorf("reset-request silencioso: status %d corpo %v", w.Code, body)
	}
	reset := tokenDoLog(t, pool)

	// Senha fraca → 422; token inválido → 404; troca forte → 200.
	w, body = chama(t, r, "POST", "/v1/auth/password/reset", `{"token":"`+reset+`","senha":"curta"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("senha fraca: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "POST", "/v1/auth/password/reset", `{"token":"invalido","senha":"nova-senha-forte"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("token inválido: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "POST", "/v1/auth/password/reset", `{"token":"`+reset+`","senha":"nova-senha-forte"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reset: status %d corpo %v", w.Code, body)
	}
	// Token é de uso único.
	w, body = chama(t, r, "POST", "/v1/auth/password/reset", `{"token":"`+reset+`","senha":"outra-senha"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("reuso do token: status %d corpo %v", w.Code, body)
	}

	// Senha antiga morre, nova entra; sessões antigas revogadas.
	svc := auth.New(st, audit, time.Minute, time.Hour, "http://localhost:5173")
	if _, _, _, err := svc.Login(ctx, "super@seno.dev", "102938", "ip", "ua"); err == nil {
		t.Error("senha antiga ainda funciona")
	}
	w, body = chama(t, r, "POST", "/v1/auth/login", `{"email":"super@seno.dev","senha":"nova-senha-forte"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login com nova senha: status %d corpo %v", w.Code, body)
	}

	// Brute force: 10 falhas e a próxima tentativa é limitada (429).
	for i := 0; i < 10; i++ {
		chama(t, r, "POST", "/v1/auth/login", `{"email":"bruteforce@seno.dev","senha":"x"}`)
	}
	w, body = chama(t, r, "POST", "/v1/auth/login", `{"email":"bruteforce@seno.dev","senha":"x"}`)
	if w.Code != http.StatusTooManyRequests || falhaDoEnvelope(t, body) != "LIMITE_EXCEDIDO" {
		t.Errorf("rate limit: status %d corpo %v", w.Code, body)
	}
}
