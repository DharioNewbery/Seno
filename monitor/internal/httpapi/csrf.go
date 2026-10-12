// Guarda de Origin das mutações de aluno (plano Fase 3.5, lado do Monitor):
// o navegador fala direto com o Monitor e a sessão é cookie. SameSite=Lax
// já amostra o cookie no cross-site fetch; a guarda fecha a linha do
// replay de referer de OTHER origens legítimas (páginas falsas no domínio
// do usuário). Convention: request sem Origin (curl, testes, verificador)
// passa; Origin presente e divergente → 403.
package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
)

// guardaOrigin valida o cabeçalho Origin das requisições mutantes do /a:
// a página da prova é sempre servida pelo próprio Monitor/podendo estar atrás
// de um proxy (que passa o host público em Host) — a origem legítima é
// scheme://host da requisição. SENO_WEB_URL não entra aqui: só prefixa o
// correcao_url da tela.
func (d *Dependencies) guardaOrigin(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin == "" {
		c.Next() // cliente programático (curl, testes, verificação)
		return
	}
	if origin != origem(urlDaRequisicao(c)) {
		respondErro(c, http.StatusForbidden, contract.CodeSemPermissao,
			"Origem da requisição não permitida.", nil)
		return
	}
	c.Next()
}

// origem extrai "scheme://host[:port]" de um URL ("" se ilegível).
func origem(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Host == "" {
		return ""
	}
	return p.Scheme + "://" + p.Host
}

// urlDaRequisicao monta "scheme://host[:port]". O scheme vem do TLS da
// conexão OU do X-Forwarded-Proto — atrás do Caddy/Nginx (TLS terminado no
// proxy) a conexão interna é http e, sem o header, o Origin https://… do
// navegador nunca casaria e toda mutação de aluno tomaria 403.
func urlDaRequisicao(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	if host := c.Request.Host; host != "" {
		return scheme + "://" + host
	}
	return strings.TrimSpace(c.Request.URL.String())
}
