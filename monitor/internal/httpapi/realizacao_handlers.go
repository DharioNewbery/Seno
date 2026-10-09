package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/atividades"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

// abiertaCtx helpers comunes: contexto de rota de aluno.
func contextoAluno(c *gin.Context) (mid string, aluno int64, ok bool) {
	mid = c.Param("monitor_id")
	if mid != c.GetString("monitor_id") {
		// sessão foi concedida para outra atividade (st escopado)
		respondErro(c, http.StatusForbidden, contract.CodeSemPermissao,
			"Esta sessão não vale para esta atividade.", nil)
		return "", 0, false
	}
	aluno = c.GetInt64("aluno_api_id")
	return mid, aluno, true
}

// AbrirAtividade — POST /a/atividade/:monitor_id/abrir (spec §5.1, idempotente).
func (d *Dependencies) AbrirAtividade(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	aberta, err := d.TentativaS.Abrir(c.Request.Context(), mid, aluno, time.Now())
	if err != nil {
		mapearErroTentativa(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"atividade": atividades.Sanitizada(aberta.Carga),
		"tentativa": gin.H{
			"fim_real":           aberta.FimReal,
			"segundos_restantes": aberta.Restantes,
			"revisao":            aberta.Revisao,
			"respostas":          aberta.Respostas,
		},
	})
}

// SalvarTentativa — PUT /a/atividade/:monitor_id/tentativa (spec §5.2):
// autosave com revisão otimista e snapshot por intervalo.
func (d *Dependencies) SalvarTentativa(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	corpo, err := readBody(c, d.Cfg.MaxSnapshotBytes)
	if err != nil {
		respondErro(c, http.StatusRequestEntityTooLarge, contract.CodeTamanhoExcedido,
			"Corpo maior que SENO_MONITOR_MAX_SNAPSHOT_BYTES.", nil)
		return
	}
	var req struct {
		Revisao   int            `json:"revisao"`
		Respostas map[string]any `json:"respostas"`
	}
	if err := json.Unmarshal(corpo, &req); err != nil {
		respondErro(c, http.StatusBadRequest, contract.CodeJSONInvalido,
			"JSON ilegível no autosave.", nil)
		return
	}
	if req.Respostas == nil {
		req.Respostas = map[string]any{}
	}

	nova, err := d.TentativaS.Salvar(c.Request.Context(), mid, aluno, req.Respostas, req.Revisao, time.Now())
	switch {
	case errors.Is(err, tentativas.ErrRevisao):
		atual, _ := d.TentativaS.RevisaoAtual(c.Request.Context(), mid, aluno)
		respondErro(c, http.StatusConflict, contract.CodeRevisaoObsoleta,
			"A tentativa foi gravada por outra aba.", gin.H{"revisao_atual": atual})
		return
	case err != nil:
		mapearErroTentativa(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"revisao": nova})
}

// mapearErroTentativa converte os erros de negócio do service em respostas.
func mapearErroTentativa(c *gin.Context, err error) {
	var inicio *tentativas.ErroInicio
	switch {
	case errors.As(err, &inicio):
		respondErro(c, http.StatusConflict, contract.CodeInicioFuturo,
			"A atividade ainda não está aberta.",
			gin.H{"segundos_para_inicio": inicio.Segundos})
	case errors.Is(err, tentativas.ErrEncerrada):
		respondErro(c, http.StatusGone, contract.CodeEncerrada,
			"A janela da atividade está encerrada.", nil)
	case errors.Is(err, tentativas.ErrFinalizada):
		respondErro(c, http.StatusForbidden, contract.CodeFinalizada,
			"Atividade encerrada para você.", nil)
	case errors.Is(err, tentativas.ErrSemTentativa):
		respondErro(c, http.StatusConflict, contract.CodeConflito,
			"Abra a atividade antes de continuar.", nil)
	default:
		respondErro(c, http.StatusInternalServerError, contract.CodeInternal,
			"Erro interno do servidor.", nil)
	}
}
