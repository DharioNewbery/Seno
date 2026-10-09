package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
)

// ReceberAtividade — POST /interna/v1/atividades (spec §2).
// Autenticação HMAC; corpo integral da atividade; idempotente por api_id.
func (d *Dependencies) ReceberAtividade(c *gin.Context) {
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

	var t contract.Transferencia
	if err := json.Unmarshal(corpo, &t); err != nil {
		respondErro(c, http.StatusBadRequest, contract.CodeJSONInvalido,
			"JSON ilegível na transferência.", nil)
		return
	}

	if t.APIID == 0 {
		respondErro(c, http.StatusUnprocessableEntity, contract.CodeValidacao,
			"api_id obrigatório.", nil)
		return
	}
	if len(t.Alunos) == 0 {
		respondErro(c, http.StatusUnprocessableEntity, contract.CodeValidacao,
			"alunos vazio (matrículas ativas do snapshot).", nil)
		return
	}

	monitorID, existente, err := d.AtividadeS.Receber(c.Request.Context(), &t)
	if err != nil {
		respondErro(c, http.StatusInternalServerError, contract.CodeInternal,
			"Erro interno do servidor.", nil)
		return
	}
	status := http.StatusCreated
	if existente {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"monitor_id": monitorID})
}

// readBody lê o corpo bruto com limite (dupla finalidade: bytes para o MAC e
// para o decode). LimitReader de max+1 detecta estouro.
func readBody(c *gin.Context, max int) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(c.Request.Body, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > max {
		return nil, errors.New("body-limite")
	}
	return b, nil
}

// respondErro escreve o envelope único {"erro": {...}} (contract/erro.go).
func respondErro(c *gin.Context, status int, codigo, mensagem string, detalhe any) {
	c.AbortWithStatusJSON(status, contract.NovoErroEnvelope(codigo, mensagem, detalhe))
}
