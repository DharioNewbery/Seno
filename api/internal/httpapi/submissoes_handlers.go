// Handlers das submissões (fluxo do aluno + entregas do professor).
package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
)

// EntregarSubmissao: POST /v1/atribuicoes/:id/submissao — aluno entrega
// a atividade (snapshot corrente da tentativa); pode incluir observação.
func (d *Dependencies) EntregarSubmissao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad struct {
		Observacao string `json:"observacao,omitempty"`
	}
	if c.Request.ContentLength > 0 && !bindJSON(c, &cad) {
		return
	}
	sub, err := d.SubmissaoS.Entregar(
		c.Request.Context(), user, id, strings.TrimSpace(cad.Observacao),
	)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	// Correção nasce junto com a submissão (PROJETO §Correção: "gerada
	// automaticamente"). Falha no despacho não bloqueia a entrega.
	if _, corErr := d.CorrecaoS.Iniciar(
		c.Request.Context(), sub.ID, user.ID, user.Email,
	); corErr != nil {
		c.JSON(http.StatusCreated, gin.H{
			"submissao":   sub,
			"correcao":    map[string]any{"erro": "não iniciada"},
		})
		return
	}
	cor, _ := d.CorrecaoS.VerProfessor(c.Request.Context(), user, sub.ID)
	_ = cor
	c.JSON(http.StatusCreated, sub)
}

// ListarSubmissoes: GET /v1/atribuicoes/:id/submissoes — dono da turma
// ou staff (quem entregou, atrasadas).
func (d *Dependencies) ListarSubmissoes(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	lista, err := d.SubmissaoS.Listar(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":       len(lista),
		"submissoes": lista,
	})
}

// ExcluirSubmissao: DELETE /v1/submissoes/:id — professor dono ou staff exclui entrega
// do aluno. A tentativa do aluno é restaurada para nova entrega com o comecou_em original.
func (d *Dependencies) ExcluirSubmissao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.SubmissaoS.Excluir(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ColetarRascunho: POST /v1/atribuicoes/:id/coletar/:aluno_id — professor coleta rascunho de aluno
// cujo prazo expirou em atribuição sem atraso permitido.
func (d *Dependencies) ColetarRascunho(c *gin.Context) {
	user, _ := currentUser(c)
	atribuicaoID, ok := idDaRota(c)
	if !ok {
		return
	}
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"erro": gin.H{"codigo": "VALIDACAO", "mensagem": "ID de aluno inválido."},
		})
		return
	}
	sub, err := d.SubmissaoS.Coletar(c.Request.Context(), user, atribuicaoID, alunoID)
	if err != nil {
		// Se for atribuição do Monitor, tenta coletar lá:
		var mid *string
		if errQ := d.Store.Pool.QueryRow(c.Request.Context(),
			`SELECT monitor_id FROM atribuicoes WHERE id = $1`, atribuicaoID).Scan(&mid); errQ == nil && mid != nil && *mid != "" {
			if errMon := d.Monitor.ColetarTentativa(c.Request.Context(), *mid, alunoID); errMon == nil {
				_ = d.Monitor.PollEstado(c.Request.Context())
				c.JSON(http.StatusOK, gin.H{"ok": true, "status": "coletada_monitor"})
				return
			}
		}
		platform.ErrorBody(c, err)
		return
	}
	// Inicia a correção automática
	if _, corErr := d.CorrecaoS.Iniciar(
		c.Request.Context(), sub.ID, user.ID, user.Email,
	); corErr != nil {
		c.JSON(http.StatusCreated, gin.H{
			"submissao": sub,
			"correcao":  map[string]any{"erro": "não iniciada"},
		})
		return
	}
	c.JSON(http.StatusCreated, sub)
}

func (d *Dependencies) obterMonitorAtribuicao(c *gin.Context, atribuicaoID int64) (string, bool) {
	user, _ := currentUser(c)
	var turmaProfID int64
	var mid *string
	err := d.Store.Pool.QueryRow(c.Request.Context(),
		`SELECT t.professor_id, a.monitor_id FROM atribuicoes a JOIN turmas t ON t.id = a.turma_id WHERE a.id = $1`, atribuicaoID).Scan(&turmaProfID, &mid)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": gin.H{"codigo": "NAO_ENCONTRADO", "mensagem": "Atribuição não encontrada."}})
		return "", false
	}
	if !user.IsStaff() && user.ID != turmaProfID {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": gin.H{"codigo": "NAO_ENCONTRADO", "mensagem": "Atribuição não encontrada."}})
		return "", false
	}
	if mid == nil || *mid == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "VALIDACAO", "mensagem": "Atribuição não está no Monitor."}})
		return "", false
	}
	return *mid, true
}

// MonitorReexecutar: POST /v1/atribuicoes/:id/monitor/alunos/:aluno_id/reexecutar
func (d *Dependencies) MonitorReexecutar(c *gin.Context) {
	atribuicaoID, ok := idDaRota(c)
	if !ok {
		return
	}
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "VALIDACAO", "mensagem": "aluno_id inválido."}})
		return
	}
	mid, ok := d.obterMonitorAtribuicao(c, atribuicaoID)
	if !ok {
		return
	}
	if err := d.Monitor.Reexecutar(c.Request.Context(), mid, alunoID); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	_ = d.Monitor.PollEstado(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MonitorExcluir: DELETE /v1/atribuicoes/:id/monitor/alunos/:aluno_id/submissao
func (d *Dependencies) MonitorExcluir(c *gin.Context) {
	atribuicaoID, ok := idDaRota(c)
	if !ok {
		return
	}
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "VALIDACAO", "mensagem": "aluno_id inválido."}})
		return
	}
	mid, ok := d.obterMonitorAtribuicao(c, atribuicaoID)
	if !ok {
		return
	}
	if err := d.Monitor.ExcluirSubmissao(c.Request.Context(), mid, alunoID); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	_ = d.Monitor.PollEstado(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MonitorObterSubmissao: GET /v1/atribuicoes/:id/monitor/alunos/:aluno_id/submissao
func (d *Dependencies) MonitorObterSubmissao(c *gin.Context) {
	atribuicaoID, ok := idDaRota(c)
	if !ok {
		return
	}
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "VALIDACAO", "mensagem": "aluno_id inválido."}})
		return
	}
	mid, ok := d.obterMonitorAtribuicao(c, atribuicaoID)
	if !ok {
		return
	}
	det, err := d.Monitor.ObterSubmissao(c.Request.Context(), mid, alunoID)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, det)
}



