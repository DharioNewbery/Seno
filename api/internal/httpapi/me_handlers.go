// Handlers do portal aluno (GET /v1/me/*): turmas e atribuições do
// próprio usuário, com cargo student exigido no serviço.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
)

// MinhasTurmas: GET /v1/me/turmas.
func (d *Dependencies) MinhasTurmas(c *gin.Context) {
	user, _ := currentUser(c)
	turmas, err := d.Ensino.MinhasTurmas(c.Request.Context(), user)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"turmas": turmas})
}

// MinhaTurma: GET /v1/me/turmas/:id.
func (d *Dependencies) MinhaTurma(c *gin.Context) {
	user, _ := currentUser(c)
	turmaID, ok := idDaRota(c)
	if !ok {
		return
	}
	detalhes, err := d.Ensino.ObterTurmaAluno(c.Request.Context(), user, turmaID)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, detalhes)
}

// MinhasAtribuicoes: GET /v1/me/atribuicoes?turma_id.
func (d *Dependencies) MinhasAtribuicoes(c *gin.Context) {
	user, _ := currentUser(c)
	turma, err := numParamOpcional(c, "turma_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	lista, err := d.Ensino.MinhasAtribuicoes(c.Request.Context(), user, turma)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":       len(lista),
		"atribuicoes": lista,
	})
}
