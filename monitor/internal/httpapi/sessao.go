package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/sessao"
)

// RequireSessao protege as rotas de aluno (/a/*): cookie hm_sess válido →
// aluno_api_id e monitor_id no contexto; inválido → 401 genérico.
// Janela e cota (finalizada) devem ser revalidadas nos handlers — aqui só
// existência/expiração da sessão.
func (d *Dependencies) RequireSessao(c *gin.Context) {
	raw, err := c.Cookie(sessao.CookieNome)
	if err != nil || raw == "" {
		respondErro(c, http.StatusUnauthorized, contract.CodeMACInvalido,
			"Sessão ausente ou inválida.", nil)
		return
	}

	h := sha256.Sum256([]byte(raw))

	var aluno int64
	var mid string
	if err := d.DB.QueryRow(c.Request.Context(),
		`SELECT aluno_api_id, monitor_id FROM sessao
		   WHERE token_hash = $1 AND expira_em > now()`,
		hex.EncodeToString(h[:])).Scan(&aluno, &mid); err != nil {
		respondErro(c, http.StatusUnauthorized, contract.CodeMACInvalido,
			"Sessão ausente ou inválida.", nil)
		return
	}

	c.Set("aluno_api_id", aluno)
	c.Set("monitor_id", mid)
	c.Next()
}
