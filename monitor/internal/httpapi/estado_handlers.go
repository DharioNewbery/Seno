package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/estado"
)

// EstadoAtividade — GET /interna/v1/atividades/:monitor_id/estado (spec §6,
// Fase 7): quadro consolidado consumido e PERSISTIDO pela API principal via
// job periódico (HMAC; leitura pura: snapshot integral por chamada).
func (d *Dependencies) EstadoAtividade(c *gin.Context) {
	corpo, err := readBody(c, d.Cfg.MaxPayloadBytes)
	if err != nil {
		respondErro(c, http.StatusRequestEntityTooLarge, contract.CodeTamanhoExcedido,
			"Corpo maior que SENO_MONITOR_MAX_PAYLOAD_BYTES.", nil)
		return
	}
	if err := contract.VerificarMAC([]byte(d.Cfg.Secret), corpo,
		c.GetHeader(contract.HeaderMAC), c.GetHeader(contract.HeaderTime),
		d.Cfg.HMACJanela); err != nil {
		respondErro(c, http.StatusUnauthorized, contract.CodeMACInvalido,
			"Assinatura ou tempo inválido.", nil)
		return
	}

	est, err := d.EstadoS.Estado(c.Request.Context(), c.Param("monitor_id"))
	switch {
	case errors.Is(err, estado.ErrNaoEncontrado):
		respondErro(c, http.StatusNotFound, contract.CodeNaoEncontrado,
			"Atividade não encontrada no Monitor.", nil)
	case err != nil:
		slog.Error("monitor: estado", "err", err)
		respondErro(c, http.StatusInternalServerError, contract.CodeInternal,
			"Erro interno do servidor.", nil)
		return
	default:
		c.JSON(http.StatusOK, est)
	}
}
