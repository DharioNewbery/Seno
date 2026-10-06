// Handlers da correção (professor|staff) e do callback do Judge0.
package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/correcoes"
	"github.com/seno-project/seno/api/internal/judge0"
	"github.com/seno-project/seno/api/internal/platform"
)

// verCorrecao: GET /v1/submissoes/:id/correcao — visão completa do
// professor (dono da turma da submissão|staff).
func (d *Dependencies) verCorrecao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	visao, err := d.CorrecaoS.VerProfessor(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, visao)
}

// editarCorrecao: PATCH /v1/submissoes/:id/correcao — feedback geral +
// notas/feedback por tarefa (dono|staff).
func (d *Dependencies) editarCorrecao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad correcoes.CadastroEdicao
	if !bindJSON(c, &cad) {
		return
	}
	visao, err := d.CorrecaoS.Editar(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, visao)
}

// confirmarCorrecao: POST /v1/submissoes/:id/correcao/confirmar.
func (d *Dependencies) confirmarCorrecao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	visao, err := d.CorrecaoS.Confirmar(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, visao)
}

// publicarCorrecao: POST /v1/submissoes/:id/correcao/publicar.
func (d *Dependencies) publicarCorrecao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	visao, err := d.CorrecaoS.Publicar(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, visao)
}

// minhaCorrecao: GET /v1/submissoes/:id/minha — aluno vê a correção
// publicada (§Visibilidade); antes disso, apenas estado.
func (d *Dependencies) minhaCorrecao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	visao, err := d.CorrecaoS.VerAluno(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, visao)
}

// callbackJudge0: POST /v1/judge0/callback?key&par=ct:te — Judge0 envia
// o veredito (callback_url de cada submissão do lote). Validado pela
// chave compartilhada; espera objeto ou array (lote).
func (d *Dependencies) callbackJudge0(c *gin.Context) {
	if d.Cfg.Judge0Secret != "" && c.Query("key") != d.Cfg.Judge0Secret {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"erro": gin.H{"codigo": "SEM_PERMISSAO", "mensagem": "Chave inválida."},
		})
		return
	}
	par := c.Query("par")
	partes := strings.SplitN(par, ":", 2)
	if len(partes) != 2 {
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	ctID, err1 := strconv.ParseInt(partes[0], 10, 64)
	teID, err2 := strconv.ParseInt(partes[1], 10, 64)
	if err1 != nil || err2 != nil || ctID < 1 || teID < 1 {
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	bruto, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var itens []judge0.RespostaJudge0
	// Judge0 manda objeto único por callback; toleramos array também.
	if err := json.Unmarshal(bruto, &itens); err != nil {
		var uno judge0.RespostaJudge0
		if err := json.Unmarshal(bruto, &uno); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		itens = []judge0.RespostaJudge0{uno}
	}
	for _, item := range itens {
		if err := d.CorrecaoS.AplicarResultado(
			c.Request.Context(), ctID, teID, item,
		); err != nil {
			platform.ErrorBody(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"aplicados": len(itens)})
}
