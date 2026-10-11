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

// guardaOrigin valida o cabeçalho Origin das requisições mutantes do /a.
func (d *Dependencies) guardaOrigin(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin == "" {
		c.Next()
		return
	}
	esperada := origem(urlDaRequisicao(c)) // scheme://host da requisição (proxy passa o host público)
	if d.Cfg != nil && d.Cfg.WebURL != "" {
		if b := origem(d.Cfg.WebURL); b != "" {
			esperada = b
		}
	}
	if origin != esperada {
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

func urlDaRequisicao(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if host := c.Request.Host; host != "" {
		return scheme + "://" + host
	}
	return strings.TrimSpace(c.Request.URL.String())
}
