// Handlers do Execução de teste (aluno + testar do professor).
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/testar"
)

// TestarTarefa: POST /v1/atribuicoes/:id/testar — aluno roda o código
// atual contra os testes públicos (regras no serviço).
func (d *Dependencies) TestarTarefa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad testar.CadastroTeste
	if !bindJSON(c, &cad) {
		return
	}
	exec, err := d.TestarS.TestarTarefa(
		c.Request.Context(), user, id, cad.TarefaID, cad.Codigo,
	)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, exec)
}

// HistoricoTestes: GET /v1/atribuicoes/:id/testes — execuções do
// próprio aluno nos últimos 7 dias (todas ou da atribuição).
func (d *Dependencies) HistoricoTestes(c *gin.Context) {
	user, _ := currentUser(c)
	id, err := numParamOpcional(c, "atribuicao_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	lista, err := d.TestarS.Historico(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": len(lista), "execucoes": lista})
}

// TestarTarefaProfessor: POST /v1/tarefas/:id/testar — banco de
// tarefas, linguagem livre, professor dono|staff, sem histórico.
func (d *Dependencies) TestarTarefaProfessor(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad testar.CadastroTesteProfessor
	if !bindJSON(c, &cad) {
		return
	}
	resultados, err := d.TestarS.TestarTarefaProfessor(
		c.Request.Context(), user, id, cad,
	)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"resultados": resultados})
}
