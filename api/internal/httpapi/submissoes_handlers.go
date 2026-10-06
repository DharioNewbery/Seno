// Handlers das submissões (fluxo do aluno + entregas do professor).
package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
)

// EntregarSubmissao: POST /v1/atribuicoes/:id/submissao — aluno entrega
// a atividade (snapshot corrente da tentativa); pode incluir observação.
func (d *Dependencies) EntregarSubmissao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad struct {
		Observacao string `json:"observacao,omitempty"`
	}
	if c.Request.ContentLength > 0 && !bindJSON(c, &cad) {
		return
	}
	sub, err := d.SubmissaoS.Entregar(
		c.Request.Context(), user, id, strings.TrimSpace(cad.Observacao),
	)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, sub)
}

// ListarSubmissoes: GET /v1/atribuicoes/:id/submissoes — dono da turma
// ou staff (quem entregou, atrasadas).
func (d *Dependencies) ListarSubmissoes(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	lista, err := d.SubmissaoS.Listar(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":       len(lista),
		"submissoes": lista,
	})
}
