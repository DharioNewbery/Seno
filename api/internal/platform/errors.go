package platform

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Códigos de erro do envelope único da API (ARQUITETURA §4.2).
const (
	CodeUnauthorized   = "SESSAO_INVALIDA"
	CodeForbidden      = "SEM_PERMISSAO"
	CodeNotFound       = "NAO_ENCONTRADO"
	CodeConflict       = "CONFLITO_DE_ESTADO"
	CodeRateLimited    = "LIMITE_EXCEDIDO"
	CodeUnprocessable  = "VALIDACAO"
	CodeInternal       = "ERRO_INTERNO"
	CodeWrongTimestamp = "ORIGEM_INVALIDA"
)

// APIError é o tipo de erro dominó da camada de aplicação. Handlers
// devolvem-no e o framework serializa no envelope único.
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"codigo"`
	Message string `json:"mensagem"`
	Detail  any    `json:"detalhe,omitempty"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// NewAPIError cria um APIError com status, código e mensagem.
func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// WithDetail anexa o detalhe estruturado ao erro (ex.: revisao_atual).
func (e *APIError) WithDetail(detail any) *APIError {
	e.Detail = detail
	return e
}

// ErrorSeq constrói a partir de um erro de negócio o envelope único.
// Erros desconhecidos viram 500 sem expor detalhes internos.
func ErrorBody(c *gin.Context, err error) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		slog.Error("erro interno", "err", err)
		apiErr = NewAPIError(http.StatusInternalServerError, CodeInternal, "Erro interno do servidor.")
	}
	c.AbortWithStatusJSON(apiErr.Status, gin.H{"erro": apiErr})
}

// AbortWithError envia o envelope e encerra a cadeia de handlers.
func AbortWithError(c *gin.Context, err error) {
	ErrorBody(c, err)
}
