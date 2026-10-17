// Handlers de analytics e métricas (PROJETO §Analytics).
// Visão professor: turmas, atribuições, tarefas e alunos.
// Visão admin: métricas globais da plataforma.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
)

// TurmaAnalytics: GET /v1/turmas/:id/analytics (professor ou staff).
func (d *Dependencies) TurmaAnalytics(c *gin.Context) {
	user, _ := currentUser(c)
	turmaID, ok := idDaRota(c)
	if !ok {
		return
	}
	res, err := d.Analytics.TurmaAnalytics(c.Request.Context(), user, turmaID)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// AtribuicaoAnalytics: GET /v1/atribuicoes/:id/analytics (professor ou staff).
func (d *Dependencies) AtribuicaoAnalytics(c *gin.Context) {
	user, _ := currentUser(c)
	atribuicaoID, ok := idDaRota(c)
	if !ok {
		return
	}
	res, err := d.Analytics.AtribuicaoAnalytics(c.Request.Context(), user, atribuicaoID)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// AlunoTurmaAnalytics: GET /v1/turmas/:id/alunos/:aluno_id/analytics (professor ou staff).
func (d *Dependencies) AlunoTurmaAnalytics(c *gin.Context) {
	user, _ := currentUser(c)
	turmaID, ok := idDaRota(c)
	if !ok {
		return
	}
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID <= 0 {
		platform.ErrorBody(c, platform.NewAPIError(http.StatusBadRequest, platform.CodeUnprocessable, "aluno_id inválido."))
		return
	}
	res, err := d.Analytics.AlunoTurmaAnalytics(c.Request.Context(), user, turmaID, alunoID)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// TarefaAnalytics: GET /v1/tarefas/:id/analytics (professor ou staff).
func (d *Dependencies) TarefaAnalytics(c *gin.Context) {
	user, _ := currentUser(c)
	tarefaID, ok := idDaRota(c)
	if !ok {
		return
	}
	res, err := d.Analytics.TarefaAnalytics(c.Request.Context(), user, tarefaID)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// AdminMetricas: GET /v1/admin/metricas (apenas staff: admin/super).
func (d *Dependencies) AdminMetricas(c *gin.Context) {
	user, _ := currentUser(c)
	res, err := d.Analytics.AdminMetricas(c.Request.Context(), user)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
