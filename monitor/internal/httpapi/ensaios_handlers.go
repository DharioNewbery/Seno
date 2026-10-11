package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/ensaios"
	"github.com/seno-project/seno/monitor/internal/judge0"
	"github.com/seno-project/seno/monitor/internal/preavaliacao"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

// Ensaio — POST /a/atividade/:monitor_id/testar (spec §5.3): cria o run na
// fila (assíncrono do Judge0) e devolve 202 {run_id}.
func (d *Dependencies) Ensaio(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	var req struct {
		Ordem     int    `json:"ordem"`
		Linguagem string `json:"linguagem"`
		Codigo    string `json:"codigo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErro(c, http.StatusBadRequest, contract.CodeJSONInvalido,
			"JSON ilegível no ensaio.", nil)
		return
	}

	run, err := d.EnsaiosS.Agendar(c.Request.Context(), mid, aluno, req.Ordem, req.Linguagem, req.Codigo, time.Now())
	switch {
	case errors.Is(err, ensaios.ErrLimite):
		respondErro(c, http.StatusTooManyRequests, contract.CodeRateLimited,
			"Limite de 5 ensaios por minuto excedido.", nil)
	case errors.Is(err, ensaios.ErrSemTarefa):
		respondErro(c, http.StatusNotFound, contract.CodeEncerrada,
			"Tarefa não encontrada na atividade.", nil)
	case errors.Is(err, ensaios.ErrSemPublico):
		respondErro(c, http.StatusUnprocessableEntity, contract.CodeValidacao,
			"Tarefa sem testes públicos.", nil)
	case errors.Is(err, tentativas.ErrSemTentativa):
		respondErro(c, http.StatusConflict, contract.CodeConflito,
			"Abra a atividade antes de ensaiar.", nil)
	case err != nil:
		mapearErroTentativa(c, err)
		return
	default:
		c.JSON(http.StatusAccepted, gin.H{"run_id": run})
	}
}

// EnsaioResultado — GET /a/atividade/:monitor_id/testar/:run_id (spec §5.3):
// 202 enquanto roda, 200 com resultados quando pronto.
func (d *Dependencies) EnsaioResultado(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	run := c.Param("run_id")
	estado, err := d.EnsaiosS.Consultar(c.Request.Context(), mid, aluno, run)
	if err != nil {
		respondErro(c, http.StatusNotFound, contract.CodeEncerrada,
			"Run não encontrado.", nil)
		return
	}
	status := http.StatusOK
	if !estado.Pronto {
		status = http.StatusAccepted
	}
	c.JSON(status, estado)
}

// PreAvaliar — POST /a/atividade/:monitor_id/submissao/avaliar (spec §5.4):
// síncrono; devolve o relatório (spec §5.4) com pre_id e expira_em.
func (d *Dependencies) PreAvaliar(c *gin.Context) {
	mid, aluno, ok := contextoAluno(c)
	if !ok {
		return
	}
	rel, err := d.PreAvaS.Avaliar(c.Request.Context(), mid, aluno, time.Now())
	switch {
	case errors.Is(err, preavaliacao.ErrSemTentativa):
		respondErro(c, http.StatusForbidden, contract.CodeFinalizada,
			"Avaliação exige tentativa aberta e janela válida.", nil)
	case err != nil:
		mapearErroTentativa(c, err)
		return
	default:
		c.JSON(http.StatusOK, gin.H{
			"pre_id":    rel.PreID,
			"resumo":    rel.Resumo,
			"tarefas":   rel.Tarefas,
			"expira_em": strconv.Itoa(int(time.Duration(d.Cfg.PreAvaTTL).Seconds())) + "s",
		})
	}
}

// CallbackJudge0 — PUT /interna/callbacks/judge0 (callback real do Judge0,
// verificado ao vivo; PROBES.md). Autenticaid por ?key=SENO_JUDGE0_CALLBACK_SECRET
// (sem header de auth no Judge0 nesta versão). Corpo base64 conforme probe.
func (d *Dependencies) CallbackJudge0(c *gin.Context) {
	if d.Cfg.Judge0Secret == "" || c.Query("key") != d.Cfg.Judge0Secret {
		respondErro(c, http.StatusUnauthorized, contract.CodeMACInvalido,
			"Callback sem segredo válido.", nil)
		return
	}
	run := c.Query("run")
	testeOrdem, errOrdem := strconv.Atoi(c.Query("t"))
	if run == "" || errOrdem != nil {
		respondErro(c, http.StatusBadRequest, contract.CodeValidacao,
			"Callback sem run/teste.", nil)
		return
	}
	var r judge0.Resposta
	if err := json.NewDecoder(c.Request.Body).Decode(&r); err != nil {
		respondErro(c, http.StatusBadRequest, contract.CodeJSONInvalido,
			"Callback ilegível.", nil)
		return
	}
	if err := d.EnsaiosS.Callback(c.Request.Context(), run, testeOrdem, r); err != nil {
		respondErro(c, http.StatusInternalServerError, contract.CodeInternal,
			"Erro interno do servidor.", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
