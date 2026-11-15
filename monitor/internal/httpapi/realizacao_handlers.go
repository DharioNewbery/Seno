package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/atividades"
	"github.com/seno-project/seno/monitor/internal/platform"
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
	if idempotente := aberta.Revisao == 0; idempotente { // primeira abertura é escrita; reabrir não é
		d.Audit.Record(c.Request.Context(), platform.LogEntry{
			Agent:     "aluno:" + strconv.FormatInt(aluno, 10),
			Kind:      "abrir",
			Descricao: "tentativa aberta para monitor_id=" + mid,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"atividade": atividades.Sanitizada(aberta.Carga),
		"regras": gin.H{
			"autocomplete": aberta.Regras.Autocomplete,
		},
		"tentativa": gin.H{
			"fim_real":           aberta.FimReal,
			"segundos_restantes": aberta.Restantes,
			"revisao":            aberta.Revisao,
			"respostas":          aberta.Respostas,
			"arquivos":           aberta.Arquivos,
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

// EnviarArquivoTarefa — POST /a/atividade/:monitor_id/tarefas/:ordem/arquivo:
// recebe o arquivo de uma tarefa do tipo arquivo_submissao via multipart/form-data
// (campo "arquivo") ou JSON ({"nome_arquivo":"...","conteudo":"<base64>"}).
func (d *Dependencies) EnviarArquivoTarefa(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	ordem, err := strconv.Atoi(c.Param("ordem"))
	if err != nil || ordem < 1 {
		respondErro(c, http.StatusBadRequest, contract.CodeValidacao,
			"Ordem da tarefa inválida.", nil)
		return
	}

	maxPadrao := int64(contract.DefaultArquivoMaxBytes)
	if d.Cfg != nil && d.Cfg.ArquivoMaxBytes > 0 {
		maxPadrao = d.Cfg.ArquivoMaxBytes
	}

	var nomeArquivo string
	var dados []byte
	ct := strings.ToLower(c.ContentType())
	if strings.HasPrefix(ct, "multipart/") {
		fh, err := c.FormFile("arquivo")
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				respondErro(c, http.StatusRequestEntityTooLarge, contract.CodeTamanhoExcedido,
					contract.ErrArquivoTamanhoExcedido.Error(), nil)
				return
			}
			respondErro(c, http.StatusBadRequest, contract.CodeValidacao,
				"Envie o arquivo no campo 'arquivo' (multipart/form-data).", nil)
			return
		}
		f, err := fh.Open()
		if err != nil {
			respondErro(c, http.StatusBadRequest, contract.CodeValidacao,
				"Falha ao abrir arquivo enviado.", nil)
			return
		}
		defer f.Close()
		dados, err = io.ReadAll(io.LimitReader(f, contract.MaxArquivoMaxBytes+1))
		if err != nil {
			respondErro(c, http.StatusBadRequest, contract.CodeValidacao,
				"Falha ao ler arquivo enviado.", nil)
			return
		}
		nomeArquivo = fh.Filename
	} else {
		var req struct {
			NomeArquivo string `json:"nome_arquivo"`
			Conteudo    []byte `json:"conteudo"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				respondErro(c, http.StatusRequestEntityTooLarge, contract.CodeTamanhoExcedido,
					contract.ErrArquivoTamanhoExcedido.Error(), nil)
				return
			}
			respondErro(c, http.StatusBadRequest, contract.CodeJSONInvalido,
				"Corpo inválido para upload de arquivo.", nil)
			return
		}
		nomeArquivo = req.NomeArquivo
		dados = req.Conteudo
	}

	meta, novaRevisao, err := d.TentativaS.SalvarArquivo(
		c.Request.Context(), mid, aluno, ordem, nomeArquivo, dados, maxPadrao, time.Now(),
	)
	switch {
	case errors.Is(err, contract.ErrArquivoTamanhoExcedido):
		respondErro(c, http.StatusRequestEntityTooLarge, contract.CodeTamanhoExcedido,
			err.Error(), nil)
	case errors.Is(err, contract.ErrArquivoExtensaoNegada),
		errors.Is(err, contract.ErrArquivoConteudoNegado),
		errors.Is(err, contract.ErrArquivoVazio),
		errors.Is(err, contract.ErrArquivoNomeInvalido),
		errors.Is(err, tentativas.ErrTipoNaoArquivo):
		respondErro(c, http.StatusUnprocessableEntity, contract.CodeValidacao,
			err.Error(), nil)
	case errors.Is(err, tentativas.ErrSemTarefa):
		respondErro(c, http.StatusNotFound, contract.CodeEncerrada,
			"Tarefa não encontrada na atividade.", nil)
	case err != nil:
		mapearErroTentativa(c, err)
	default:
		c.JSON(http.StatusOK, gin.H{
			"arquivo": meta,
			"revisao": novaRevisao,
		})
	}
}

// BaixarArquivoTarefa — GET /a/atividade/:monitor_id/tarefas/:ordem/arquivo:
// devolve o arquivo anexado pelo próprio aluno na tarefa.
func (d *Dependencies) BaixarArquivoTarefa(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	ordem, err := strconv.Atoi(c.Param("ordem"))
	if err != nil || ordem < 1 {
		respondErro(c, http.StatusBadRequest, contract.CodeValidacao,
			"Ordem da tarefa inválida.", nil)
		return
	}
	meta, dados, err := d.TentativaS.ObterArquivo(c.Request.Context(), mid, aluno, ordem)
	if err != nil {
		if errors.Is(err, tentativas.ErrArquivoInexistente) {
			respondErro(c, http.StatusNotFound, contract.CodeEncerrada,
				"Nenhum arquivo anexado nesta tarefa.", nil)
			return
		}
		mapearErroTentativa(c, err)
		return
	}
	nome, _ := contract.SanitizarNomeArquivo(meta.NomeArquivo)
	if nome == "" {
		nome = "arquivo.bin"
	}
	mimeType := meta.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename=%q; filename*=UTF-8''%s`, nome, url.PathEscape(nome)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Length", strconv.Itoa(len(dados)))
	c.Data(http.StatusOK, mimeType, dados)
}

// RemoverArquivoTarefa — DELETE /a/atividade/:monitor_id/tarefas/:ordem/arquivo:
// remove o arquivo anexado pelo aluno durante a tentativa.
func (d *Dependencies) RemoverArquivoTarefa(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	ordem, err := strconv.Atoi(c.Param("ordem"))
	if err != nil || ordem < 1 {
		respondErro(c, http.StatusBadRequest, contract.CodeValidacao,
			"Ordem da tarefa inválida.", nil)
		return
	}
	novaRevisao, err := d.TentativaS.RemoverArquivo(c.Request.Context(), mid, aluno, ordem, time.Now())
	if err != nil {
		if errors.Is(err, tentativas.ErrArquivoInexistente) {
			respondErro(c, http.StatusNotFound, contract.CodeEncerrada,
				"Nenhum arquivo anexado nesta tarefa.", nil)
			return
		}
		mapearErroTentativa(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "revisao": novaRevisao})
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
		slog.Error("monitor: erro inesperado em rota de aluno", "err", err)
		respondErro(c, http.StatusInternalServerError, contract.CodeInternal,
			"Erro interno do servidor.", nil)
	}
}
