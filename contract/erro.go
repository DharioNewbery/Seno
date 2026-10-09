package contract

// Envelope de erro único da rede, espelhando api/internal/platform/errors.go
// (ARQUITETURA §4.2): {"erro": {"codigo": ..., "mensagem": ..., "detalhe": ...}}.
// Códigos compartilhados entre API e Monitor vivem aqui para os dois serviços
// não divergirem: strings idênticas garantidas em tempo de compilação.

const (
	CodeValidacao       = "VALIDACAO"
	CodeJSONInvalido    = "JSON_INVALIDO"
	CodeMACInvalido     = "MAC_INVALIDO"
	CodeTamanhoExcedido = "TAMANHO_EXCEDIDO"
	CodeInternal        = "ERRO_INTERNO"

	// Flags de aluno no Monitor (Fase 4+):
	CodeConflito        = "CONFLITO_DE_ESTADO" // revisão divergente (409)
	CodeAtualizada      = "ATUALIZADA"         // rev, pré-avaliação desatualizada
	CodeFinalizada      = "FINALIZADA"         // cota de submissões atingida (403)
	CodeInicioFuturo    = "INICIO"             // 409 {erro:"inicio"}
	CodeRateLimited     = "LIMITE_EXCEDIDO"
	CodeSemPermissao    = "SEM_PERMISSAO" // aluno de fora do snapshot (403)
	CodeEncerrada       = "ENCERRADA"     // janela sem entrega atrasada possível (410)
	CodeRevisaoObsoleta = "REVISAO_OBSOLETA" // autosave com revisão antiga (409)
)

// ErroEnvelope é o JSON completo no fio: {"erro": {...}}.
type ErroEnvelope struct {
	Erro ErroDetalhe `json:"erro"`
}

// ErroDetalhe é o objeto dentro de "erro".
type ErroDetalhe struct {
	Codigo   string `json:"codigo"`
	Mensagem string `json:"mensagem"`
	Detalhe  any    `json:"detalhe,omitempty"`
}

// NovoErroEnvelope monta o envelope no fio com um detalhe estruturado.
func NovoErroEnvelope(codigo, mensagem string, detalhe any) ErroEnvelope {
	return ErroEnvelope{Erro: ErroDetalhe{Codigo: codigo, Mensagem: mensagem, Detalhe: detalhe}}
}
