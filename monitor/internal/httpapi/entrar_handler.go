package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/sessao"
)

// Limites de entrada: 10 falhas por IP em 5 min → 429.
const (
	limEntradaMax    = 10
	limEntradaJanela = 5 * time.Minute
)

// Entrar — GET /entrar?st=... (spec §3): valida o token de SSO e concede o
// cookie hm_sess. Falhas de token → 401 genérico (não expõe qual validação
// falhou); aluno de fora → 403; atividade encerrada → 403 FINALIZADA.
func (d *Dependencies) Entrar(c *gin.Context) {
	ip := c.ClientIP()

	if d.Entradas == nil { // bootstrap padrão não fica nulo; protege testes
		d.Entradas = sessao.NovoLimiter(limEntradaMax, limEntradaJanela)
	}
	if bloqueado, quando := d.Entradas.Bloqueado(ip); bloqueado {
		c.Header("Retry-After", strconv.Itoa(int(quando.Seconds())))
		respondErro(c, http.StatusTooManyRequests, contract.CodeRateLimited,
			"Limite de tentativas de entrada excedido.",
			gin.H{"retry_apos": quando.Seconds()})
		return
	}

	claims, err := contract.VerificarST([]byte(d.Cfg.Secret), c.Query("st"),
		contract.SSOAud, time.Now())
	if err != nil {
		d.Entradas.Falha(ip)
		respondErro(c, http.StatusUnauthorized, contract.CodeMACInvalido,
			"Token de entrada ausente ou inválido.", nil)
		return
	}

	token, expira, err := d.SessaoS.Entrar(c.Request.Context(), &claims)
	switch {
	case errors.Is(err, sessao.ErrAtividadeFora):
		respondErro(c, http.StatusForbidden, contract.CodeFinalizada,
			"Atividade encerrada para você.", nil)
	case errors.Is(err, sessao.ErrAlunoFora):
		respondErro(c, http.StatusForbidden, contract.CodeSemPermissao,
			"Você não está matriculado nesta atividade.", nil)
	case err != nil:
		d.Entradas.Falha(ip)
		respondErro(c, http.StatusUnauthorized, contract.CodeMACInvalido,
			"Token de entrada ausente ou inválido.", nil)
	}
	if err != nil {
		return
	}

	// Cookie hm_sess: token cru só no navegador; banco guarda o SHA-256.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessao.CookieNome, token, int(expira.Seconds()), "/",
		"", d.Cfg.CookieSecure, true)

	c.JSON(http.StatusOK, gin.H{
		"sessao":       "ok",
		"monitor_id":   claims.Mid,
		"aluno_api_id": claims.Sub,
		"expira_em":    time.Now().Add(expira),
	})
}
