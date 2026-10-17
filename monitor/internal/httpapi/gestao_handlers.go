package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
)

func (d *Dependencies) autenticarInternoHMAC(c *gin.Context) bool {
	corpo, err := readBody(c, d.Cfg.MaxPayloadBytes)
	if err != nil {
		respondErro(c, http.StatusRequestEntityTooLarge, contract.CodeTamanhoExcedido,
			"Corpo maior que SENO_MONITOR_MAX_PAYLOAD_BYTES.", nil)
		return false
	}
	if err := contract.VerificarMAC([]byte(d.Cfg.Secret), corpo,
		c.GetHeader(contract.HeaderMAC), c.GetHeader(contract.HeaderTime),
		d.Cfg.HMACJanela); err != nil {
		respondErro(c, http.StatusUnauthorized, contract.CodeMACInvalido,
			"Assinatura ou tempo inválido.", nil)
		return false
	}
	return true
}

// ReexecutarCorrecao — POST /interna/v1/atividades/:monitor_id/alunos/:aluno_id/reexecutar
func (d *Dependencies) ReexecutarCorrecao(c *gin.Context) {
	if !d.autenticarInternoHMAC(c) {
		return
	}
	mid := c.Param("monitor_id")
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		respondErro(c, http.StatusBadRequest, contract.CodeValidacao, "aluno_id inválido.", nil)
		return
	}
	if err := d.SubmissaoS.Reexecutar(c.Request.Context(), mid, alunoID); err != nil {
		slog.Error("monitor: reexecutar", "err", err)
		respondErro(c, http.StatusUnprocessableEntity, contract.CodeValidacao, err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ExcluirSubmissao — DELETE /interna/v1/atividades/:monitor_id/alunos/:aluno_id/submissao
func (d *Dependencies) ExcluirSubmissao(c *gin.Context) {
	if !d.autenticarInternoHMAC(c) {
		return
	}
	mid := c.Param("monitor_id")
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		respondErro(c, http.StatusBadRequest, contract.CodeValidacao, "aluno_id inválido.", nil)
		return
	}
	if err := d.SubmissaoS.Excluir(c.Request.Context(), mid, alunoID); err != nil {
		slog.Error("monitor: excluir submissao", "err", err)
		respondErro(c, http.StatusUnprocessableEntity, contract.CodeValidacao, err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ColetarTentativa — POST /interna/v1/atividades/:monitor_id/alunos/:aluno_id/coletar
func (d *Dependencies) ColetarTentativa(c *gin.Context) {
	if !d.autenticarInternoHMAC(c) {
		return
	}
	mid := c.Param("monitor_id")
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		respondErro(c, http.StatusBadRequest, contract.CodeValidacao, "aluno_id inválido.", nil)
		return
	}
	entrega, err := d.SubmissaoS.Coletar(c.Request.Context(), mid, alunoID, time.Now())
	if err != nil {
		slog.Error("monitor: coletar tentativa", "err", err)
		respondErro(c, http.StatusUnprocessableEntity, contract.CodeValidacao, err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, entrega)
}
