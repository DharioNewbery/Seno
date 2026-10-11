package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seno-project/seno/monitor/internal/sessao"
)

// GET/POST direto no roteador com URL completa (Host = localhost:9090).
func reqR(t *testing.T, r actorRouter, method, path, cookie, origin string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRequest(method, "http://localhost:9090"+path, nil)
	w.RemoteAddr = "10.9.0.7:7"
	if cookie != "" {
		w.Header.Set("Cookie", sessao.CookieNome+"="+cookie)
	}
	if origin != "" {
		w.Header.Set("Origin", origin)
	}
	out := httptest.NewRecorder()
	r.ServeHTTP(out, w)
	return out
}

// TestWebuiServidos — a tela da realização e seus assets, sem sessão.
func TestWebuiServidos(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitorJudge0(t, pool)

	// caminhos do navegador: página em /a/app resolve relativos para
	// /a/app.{css,js} (e /a/app/ para /a/app/app.*)
	for _, alvo := range []struct{ path, conter string }{
		{"/a/app", "<dialog"},
		{"/a/app.js", "/abrir"},
		{"/a/app.css", "background"},
		{"/a/app/app.js", "fetch"},
	} {
		w := reqR(t, r, "GET", alvo.path, "", "")
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", alvo.path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), alvo.conter) {
			t.Fatalf("%s sem %q", alvo.path, alvo.conter)
		}
	}
}

// TestEntrarRedirectTela — navegação de navegador (Accept: text/html) no
// /entrar 302 → tela com anid; cliente JSON mantém o contrato.
func TestEntrarRedirectTela(t *testing.T) {
	pool := testeDB(t)
	_, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	w := reqR(t, r, "GET", "/entrar?st="+stDe(t, subTest, midTest, 5*time.Minute),
		"", "")
	// a entrada acima navega: Accept de navegador
	{
		w2 := httptest.NewRequest("GET", "http://localhost:9090/entrar?st="+
			stDe(t, subTest, midTest, 5*time.Minute), nil)
		w2.Header.Set("Accept", "text/html,application/xhtml+xml")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, w2)
		if out.Code != http.StatusFound {
			t.Fatalf("redirect: %d %s", out.Code, out.Body.String())
		}
		loc := out.Header().Get("Location")
		if !strings.HasPrefix(loc, "/a/app?anid="+midTest) || !strings.Contains(loc, "web=") {
			t.Fatalf("Location inesperado: %s", loc)
		}
	}

	// contrato JSON para clientes programáticos (Accept oscuro)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"sessao":"ok"`) {
		t.Fatalf("json: %d %s", w.Code, w.Body.String())
	}
}

// TestGuardaOrigin — mutação /a com Origin divergente é 403; sem Origin
// passa; origem própria e a WebURL configurada passam.
func TestGuardaOrigin(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeDB(pool, "ativa", int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}
	sess := entrarCookie(t, r, subTest, "10.9.0.7:7")

	code := func(origin string) int {
		return reqR(t, r, "POST", "/a/atividade/"+midTest+"/abrir", sess, origin).Code
	}

	if c := code(""); c != 200 {
		t.Fatalf("sem origin: %d", c)
	}
	if c := code("http://localhost:9090"); c != 200 {
		t.Fatalf("origem própria: %d", c)
	}
	if c := code("https://mau.example"); c != http.StatusForbidden {
		t.Fatalf("origin estranha: %d", c)
	}

	deps.Cfg.WebURL = "https://portal" // não altera a guarda (só correcao_url)
	if c := code("https://portal"); c != http.StatusForbidden {
		t.Fatalf("origin de outra base deveria seguir 403: %d", c)
	}
}
