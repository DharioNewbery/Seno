package httpapi

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOrigemNormalizacao(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://seno.ufpa.br", "https://seno.ufpa.br"},
		{"HTTPS://SENO.UFPA.BR:443", "https://seno.ufpa.br"},
		{"http://192.168.100.161:80", "http://192.168.100.161"},
		{"https://192.168.100.161:8443", "https://192.168.100.161:8443"},
		{"https://[::1]:443", "https://[::1]"},
		{"https://[::1]:8443", "https://[::1]:8443"},
		{"invalido", ""},
	}
	for _, tc := range cases {
		if got := origem(tc.in); got != tc.want {
			t.Errorf("origem(%q) = %q, quero %q", tc.in, got, tc.want)
		}
	}
}

func TestGuardaOriginMultiProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	deps := &Dependencies{}
	r := gin.New()
	r.POST("/a/teste", deps.guardaOrigin, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	chamar := func(host, origin string, hdrs map[string]string, withTLS bool) int {
		req := httptest.NewRequest(http.MethodPost, "http://"+host+"/a/teste", nil)
		req.Host = host
		if withTLS {
			req.TLS = &tls.ConnectionState{}
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		for k, v := range hdrs {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// 1. Sem Origin (cliente programático): passa
	if code := chamar("localhost:8081", "", nil, false); code != http.StatusOK {
		t.Fatalf("sem origin: %d", code)
	}

	// 2. Direto HTTP / HTTPS com porta não padrão
	if code := chamar("192.168.1.50:8081", "http://192.168.1.50:8081", nil, false); code != http.StatusOK {
		t.Fatalf("http direto: %d", code)
	}
	if code := chamar("192.168.1.50:8443", "https://192.168.1.50:8443", nil, true); code != http.StatusOK {
		t.Fatalf("https direto: %d", code)
	}

	// 3. Proxy único (Caddy)
	if code := chamar("monitor:8081", "https://seno.ufpa.br", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "seno.ufpa.br",
	}, false); code != http.StatusOK {
		t.Fatalf("proxy único: %d", code)
	}

	// 4. Múltiplas camadas de proxy (ex.: Proxy SETIC -> Caddy -> Monitor)
	if code := chamar("monitor:8081", "https://seno.ufpa.br", map[string]string{
		"X-Forwarded-Proto": "https, http",
		"X-Forwarded-Host":  "seno.ufpa.br, 10.0.0.5:8443",
	}, false); code != http.StatusOK {
		t.Fatalf("multi-proxy: %d", code)
	}

	// 5. Porta customizada via X-Forwarded-Host ou X-Forwarded-Port
	if code := chamar("monitor:8081", "https://192.168.100.161:8443", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "192.168.100.161:8443",
	}, false); code != http.StatusOK {
		t.Fatalf("x-forwarded-host com porta: %d", code)
	}
	if code := chamar("monitor:8081", "https://192.168.100.161:8443", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "192.168.100.161",
		"X-Forwarded-Port":  "8443",
	}, false); code != http.StatusOK {
		t.Fatalf("x-forwarded-port: %d", code)
	}

	// 6. Normalização de porta padrão (:443 em https)
	if code := chamar("monitor:8081", "https://seno.ufpa.br:443", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "seno.ufpa.br",
	}, false); code != http.StatusOK {
		t.Fatalf("porta padrão 443: %d", code)
	}

	// 7. Origem divergente → 403
	if code := chamar("monitor:8081", "https://evil.example", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "seno.ufpa.br",
	}, false); code != http.StatusForbidden {
		t.Fatalf("origin divergente: %d, quero 403", code)
	}
}
