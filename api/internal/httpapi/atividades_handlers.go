// Handlers do banco de atividades (professor+staff; escopo por dono no
// serviço, PROJETO §Atividades).
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/atividades"
	"github.com/seno-project/seno/api/internal/platform"
)

// ListarAtividades: GET /v1/atividades — professor só as próprias.
func (d *Dependencies) ListarAtividades(c *gin.Context) {
	user, _ := currentUser(c)
	prof, err := numParamOpcional(c, "professor_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	pagina, por := numPagina(c)
	lista, err := d.AtividadeS.Listar(
		c.Request.Context(), user, atividades.FiltroAtividades{
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

// CriarAtividade: POST /v1/atividades — cargo professor; dono = ator.
func (d *Dependencies) CriarAtividade(c *gin.Context) {
	user, _ := currentUser(c)
	var cad atividades.CadastroAtividade
	if !bindJSON(c, &cad) {
		return
	}
	atividade, err := d.AtividadeS.Criar(c.Request.Context(), user, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, atividade)
}

// VerAtividade: GET /v1/atividades/:id — 404 fora do escopo.
func (d *Dependencies) VerAtividade(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	atividade, err := d.AtividadeS.Ver(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, atividade)
}

// EditarAtividade: PATCH /v1/atividades/:id — dono|staff; tarefas
// substituem a lista.
func (d *Dependencies) EditarAtividade(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad atividades.CadastroAtividade
	if !bindJSON(c, &cad) {
		return
	}
	atividade, err := d.AtividadeS.Editar(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, atividade)
}

// DuplicarAtividade: POST /v1/atividades/:id/duplicar — dono|staff.
func (d *Dependencies) DuplicarAtividade(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	copia, err := d.AtividadeS.Duplicar(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, copia)
}

// ExcluirAtividade: DELETE /v1/atividades/:id — dono|staff.
func (d *Dependencies) ExcluirAtividade(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.AtividadeS.Excluir(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
