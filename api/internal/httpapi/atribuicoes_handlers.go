// Handlers das atribuições (professor+staff; escopo por dono da turma
// no serviço, PROJETO §Atribuição).
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/atribuicoes"
	"github.com/seno-project/seno/api/internal/monitor"
	"github.com/seno-project/seno/api/internal/platform"
)

// ListarAtribuicoes: GET /v1/atribuicoes?turma_id&atividade_id —
// professor só as de turmas próprias; staff tudo.
func (d *Dependencies) ListarAtribuicoes(c *gin.Context) {
	user, _ := currentUser(c)
	turma, err := numParamOpcional(c, "turma_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	atividade, err := numParamOpcional(c, "atividade_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	pagina, por := numPagina(c)
	lista, err := d.AtribuicaoS.Listar(
		c.Request.Context(), user, atribuicoes.FiltroAtribuicoes{
			TurmaID:     turma,
			AtividadeID: atividade,
			Pagina:      &pagina,
			Por:         &por,
		})
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, lista)
}

// CriarAtribuicao: POST /v1/atribuicoes — dono da turma; atividade do
// mesmo professor.
func (d *Dependencies) CriarAtribuicao(c *gin.Context) {
	user, _ := currentUser(c)
	var cad atribuicoes.CadastroAtribuicao
	if !bindJSON(c, &cad) {
		return
	}
	atribuicao, err := d.AtribuicaoS.Criar(c.Request.Context(), user, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	if err := d.Monitor.Transferir(c.Request.Context(), atribuicao.ID); err != nil {
		if delErr := d.AtribuicaoS.Excluir(c.Request.Context(), user, atribuicao.ID); delErr != nil {
			slog.Error("atribuicao: rollback após falha no Monitor", "atribuicao", atribuicao.ID, "err", delErr)
		}
		slog.Error("atribuicao: falha ao disponibilizar no Monitor", "atribuicao", atribuicao.ID, "err", err)
		switch {
		case errors.Is(err, monitor.ErrSemAlunos):
			platform.ErrorBody(c, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"A turma precisa ter pelo menos um aluno ativo para atribuir a atividade."))
		case errors.Is(err, monitor.ErrSemTarefas):
			platform.ErrorBody(c, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"A atividade precisa ter pelo menos uma tarefa para ser atribuída."))
		default:
			platform.ErrorBody(c, platform.NewAPIError(
				http.StatusServiceUnavailable, platform.CodeServiceUnavailable,
				"Não foi possível disponibilizar a atividade no Monitor. Tente novamente."))
		}
		return
	}
	c.JSON(http.StatusCreated, atribuicao)
}

// VerAtribuicao: GET /v1/atribuicoes/:id — 404 fora do escopo.
func (d *Dependencies) VerAtribuicao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	atribuicao, err := d.AtribuicaoS.Ver(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, atribuicao)
}

// EditarAtribuicao: PATCH /v1/atribuicoes/:id — marcações (nil preserva).
func (d *Dependencies) EditarAtribuicao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad atribuicoes.CadastroAtribuicao
	if !bindJSON(c, &cad) {
		return
	}
	atribuicao, err := d.AtribuicaoS.Editar(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, atribuicao)
}

// ViewAluno: GET /v1/atribuicoes/:id/aluno — material do aluno
// (exige student + matrícula ativa, validado no serviço).
func (d *Dependencies) ViewAluno(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	visao, err := d.AtribuicaoS.VerParaAluno(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, visao)
}

// EntrarMonitor: GET /v1/atribuicoes/:id/entrar — emite `st` + URL do
// Monitor (Fase 9.1; regras no serviço — student + matrícula + transferida).
func (d *Dependencies) EntrarMonitor(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	url, err := d.Monitor.Entrar(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

// ExcluirAtribuicao: DELETE /v1/atribuicoes/:id — dono|staff.
func (d *Dependencies) ExcluirAtribuicao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.AtribuicaoS.Excluir(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
