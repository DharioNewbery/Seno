// Handlers das tentativas (fluxo do aluno, PROJETO §Tentativa).
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/tentativas"
)

// AbrirTentativa: POST /v1/atribuicoes/:id/tentativa — idempotente;
// cria na 1ª vez (comecou_em), devolve o rascunho nas seguintes.
func (d *Dependencies) AbrirTentativa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	tentativa, err := d.TentativaS.Abrir(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, tentativa)
}

// LerTentativa: GET /v1/atribuicoes/:id/tentativa — do próprio aluno.
func (d *Dependencies) LerTentativa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	tentativa, err := d.TentativaS.Ler(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, tentativa)
}

// GravarTentativa: PUT /v1/atribuicoes/:id/tentativa — autosave com
// revisão; revisão antiga → 409 (duas abas).
func (d *Dependencies) GravarTentativa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad tentativas.CadastroGravacao
	if !bindJSON(c, &cad) {
		return
	}
	tentativa, err := d.TentativaS.Gravar(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, tentativa)
}
