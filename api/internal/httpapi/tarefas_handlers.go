// Handlers do banco de tarefas (professor+staff; escopo por dono no
// serviço, PROJETO §Tarefas).
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/tarefas"
)

// ListarTarefas: GET /v1/tarefas — professor recebe apenas as próprias
// (filtro forçado no serviço); admin/super podem filtrar por professor.
func (d *Dependencies) ListarTarefas(c *gin.Context) {
	user, _ := currentUser(c)
	prof, err := numParamOpcional(c, "professor_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	pagina, por := numPagina(c)
	lista, err := d.TarefaS.Listar(c.Request.Context(), user, tarefas.FiltroTarefas{
		ProfessorID: prof,
		Pagina:      &pagina,
		Por:         &por,
	})
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, lista)
}

// CriarTarefa: POST /v1/tarefas — cargo professor obrigatório; dono = ator.
func (d *Dependencies) CriarTarefa(c *gin.Context) {
	user, _ := currentUser(c)
	var cad tarefas.CadastroTarefa
	if !bindJSON(c, &cad) {
		return
	}
	tarefa, err := d.TarefaS.Criar(c.Request.Context(), user, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, tarefa)
}

// VerTarefa: GET /v1/tarefas/:id — 404 fora do escopo do professor.
func (d *Dependencies) VerTarefa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	tarefa, testes, err := d.TarefaS.Ver(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tarefa": tarefa, "testes": testes})
}

// EditarTarefa: PATCH /v1/tarefas/:id — dono|staff; testes não-nulos
// substituem a lista inteira.
func (d *Dependencies) EditarTarefa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad tarefas.CadastroTarefa
	if !bindJSON(c, &cad) {
		return
	}
	tarefa, testes, err := d.TarefaS.Editar(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tarefa": tarefa, "testes": testes})
}

// DuplicarTarefa: POST /v1/tarefas/:id/duplicar — dono|staff.
func (d *Dependencies) DuplicarTarefa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	copia, err := d.TarefaS.Duplicar(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, copia)
}

// ExcluirTarefa: DELETE /v1/tarefas/:id — dono|staff.
func (d *Dependencies) ExcluirTarefa(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.TarefaS.Excluir(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
