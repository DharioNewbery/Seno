// Guarda de Origin das mutações de aluno (plano Fase 3.5, lado do Monitor):
// o navegador fala direto com o Monitor e a sessão é cookie. SameSite=Lax
// já protege o cookie no cross-site fetch; a guarda fecha a superfície de
// requisições cross-origin de páginas no mesmo contexto. Convention: request
// sem Origin (curl, testes, verificador) passa; Origin presente e divergente → 403.
package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
)

// guardaOrigin valida o cabeçalho Origin das requisições mutantes do /a:
// a página da prova é servida pelo próprio Monitor (direto ou atrás de uma
// ou mais camadas de proxy reverso que encaminham Host / X-Forwarded-*).
// A origem legítima é derivada dinamicamente de scheme://host[:port].
// SENO_WEB_URL não entra aqui: só prefixa o correcao_url da tela quando definido.
func (d *Dependencies) guardaOrigin(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin == "" {
		c.Next() // cliente programático (curl, testes, verificação)
		return
	}
	parsedOrigin := origem(origin)
	if parsedOrigin == "" || parsedOrigin != origem(urlDaRequisicao(c)) {
		respondErro(c, http.StatusForbidden, contract.CodeSemPermissao,
			"Origem da requisição não permitida.", nil)
		return
	}
	c.Next()
}

// primeiroValorHeader extrai o primeiro valor não vazio de cabeçalhos
// encadeados por múltiplas camadas de proxy (ex.: "https, http").
func primeiroValorHeader(raw string) string {
	for _, parte := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(parte); v != "" {
			return v
		}
	}
	return ""
}

// origem normaliza "scheme://host[:port]" em minúsculas e remove portas
// padrão redundantes (:443 em https, :80 em http). Retorna "" se ilegível.
func origem(u string) string {
	p, err := url.Parse(strings.TrimSpace(u))
	if err != nil || p.Scheme == "" || p.Host == "" {
		return ""
	}
	scheme := strings.ToLower(p.Scheme)
	host := strings.ToLower(p.Host)
	if h, port, err := net.SplitHostPort(host); err == nil {
		if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
			if strings.Contains(h, ":") {
				host = "[" + h + "]"
			} else {
				host = h
			}
		}
	}
	return scheme + "://" + host
}

// urlDaRequisicao monta "scheme://host[:port]" respeitando múltiplas camadas
// de proxy reverso (X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port).
func urlDaRequisicao(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(primeiroValorHeader(c.GetHeader("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}

	host := primeiroValorHeader(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host != "" {
		if _, _, err := net.SplitHostPort(host); err != nil {
			port := primeiroValorHeader(c.GetHeader("X-Forwarded-Port"))
			if port != "" && !((scheme == "https" && port == "443") || (scheme == "http" && port == "80")) {
				host = net.JoinHostPort(strings.Trim(host, "[]"), port)
			}
		}
		return scheme + "://" + host
	}
	return strings.TrimSpace(c.Request.URL.String())
}
