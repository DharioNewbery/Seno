package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/platform"
	"github.com/seno-project/seno/monitor/internal/submissoes"
)

// Cancelar — POST /a/atividade/:monitor_id/submissao/cancelar (spec §5.5):
// descarta a pré-avaliação; tentativa continua viva.
func (d *Dependencies) CancelarSubmissao(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	var req struct {
		PreID string `json:"pre_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErro(c, http.StatusBadRequest, contract.CodeJSONInvalido,
			"JSON ilegível no cancelar.", nil)
		return
	}
	if err := d.SubmissaoS.Cancelar(c.Request.Context(), mid, aluno, req.PreID, time.Now()); err != nil {
		if errors.Is(err, submissoes.ErrPreInvalida) {
			respondErro(c, http.StatusConflict, contract.CodeConflito,
				"Pré-avaliação inexistente, expirada ou já usada.", nil)
			return
		}
		slog.Error("monitor: cancelar", "err", err)
		respondErro(c, http.StatusInternalServerError, contract.CodeInternal,
			"Erro interno do servidor.", nil)
		return
	}
	d.Audit.Record(c.Request.Context(), platform.LogEntry{
		Agent:     "aluno:" + strconv.FormatInt(aluno, 10),
		Kind:      "cancelar",
		Descricao: "envio cancelado (pre_id=" + req.PreID + ") em monitor_id=" + mid,
	})
	c.JSON(http.StatusOK, gin.H{"cancelada": true})
}

// Confirmar — POST /a/atividade/:monitor_id/submissao/confirmar (spec §5.5):
// entrega com escolha; toda a validação no §5.5.
func (d *Dependencies) ConfirmarSubmissao(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	var req struct {
		PreID string `json:"pre_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErro(c, http.StatusBadRequest, contract.CodeJSONInvalido,
			"JSON ilegível no confirmar.", nil)
		return
	}
	// prática: pre_id pertence à sessão; entrega usa este mid/aluno.
	entrega, err := d.SubmissaoS.Confirmar(c.Request.Context(), mid, aluno, req.PreID, time.Now())
	switch {
	case errors.Is(err, submissoes.ErrPreInvalida):
		respondErro(c, http.StatusConflict, contract.CodeConflito,
			"Pré-avaliação inexistente, expirada ou já usada.", nil)
	case errors.Is(err, submissoes.ErrAtualizada):
		respondErro(c, http.StatusConflict, contract.CodeAtualizada,
			"Respostas mudaram desde a pré-avaliação: reavalie.", nil)
	case errors.Is(err, submissoes.ErrSemTentativa):
		respondErro(c, http.StatusConflict, contract.CodeConflito,
			"Sem tentativa aberta para entregar.", nil)
	case errors.Is(err, tentativasErrEncerrada()):
		respondErro(c, http.StatusGone, contract.CodeEncerrada,
			"A janela da atividade está encerrada.", nil)
	case err != nil:
		slog.Error("monitor: confirmar", "err", err)
		respondErro(c, http.StatusInternalServerError, contract.CodeInternal,
			"Erro interno do servidor.", nil)
		return
	default:
		d.Audit.Record(c.Request.Context(), platform.LogEntry{
			Agent: "aluno:" + strconv.FormatInt(aluno, 10), Kind: "confirmar",
			Descricao: "submissão n_t=" + strconv.Itoa(entrega.NT) + " em monitor_id=" + mid + " (atrasada=" + strconv.FormatBool(entrega.Atrasada) + ")",
		})
		c.JSON(http.StatusOK, entrega)
	}
}

// BloquearFinalizado — middleware do grupo /a (spec §5.6): cota atingida →
// 403 FINALIZADA em todo endpoint da atividade (correcao_url do portal).
func (d *Dependencies) BloquearFinalizado(c *gin.Context) {
	mid := c.Param("monitor_id")
	aluno := c.GetInt64("aluno_api_id")
	if mid == "" || aluno == 0 { // rotas sem escopo de atividade/lun aluno
		c.Next()
		return
	}
	// BloquearFinalizado é ponto abortável; sem deps (testes antigos) segue.
	if d.SubmissaoS == nil || mid == "" || aluno == 0 {
		c.Next()
		return
	}
	finalizada, apiID, err := d.SubmissaoS.Finalizado(c.Request.Context(), mid, aluno)
	if err != nil {
		slog.Error("monitor: bloqueio finalizado", "err", err)
		c.Next() // falha de leitura NÃO bloqueia aluno: conserva as rotas
		return
	}
	if finalizada {
		respondErro(c, http.StatusForbidden, contract.CodeFinalizada,
			"Atividade encerrada para você.",
			gin.H{"correcao_url": "/aluno/atividades/" + strconv.FormatInt(apiID, 10)})
		return
	}
	c.Next()
}

func tentativasErrEncerrada() error { return submissoes.ErrJanela }
