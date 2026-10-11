// Package httpapi — tela de realização (Fase 9.4 do PLANO_MONITOR, lado do
// Monitor): interface de prova servida pelo PROPÓRIO Monitor, isolada da
// interface do sistema (web/BFF). Dados em monitor/internal/httpapi/assets,
// embebidos no binário; o app.js fala apenas com as rotas /a via hm_sess.
package httpapi

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed all:assets
var assetsFS embed.FS

// AppServir fornece as páginas estáticas da realização (sem sessão: a
// página só se descobre que tem sessão ao chamar os endpoints /a).
func (d *Dependencies) AppServir(arquivo string) gin.HandlerFunc {
	mimes := map[string]string{
		"index.html": "text/html; charset=utf-8",
		"app.css":    "text/css; charset=utf-8",
		"app.js":     "text/javascript; charset=utf-8",
	}
	return func(c *gin.Context) {
		b, err := assetsFS.ReadFile("assets/" + arquivo)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, mimes[arquivo], b)
	}
}
