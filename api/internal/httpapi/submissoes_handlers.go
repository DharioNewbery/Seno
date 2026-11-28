// Handlers das submissões (fluxo do aluno + entregas do professor).
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/contract"
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
			"submissao": sub,
			"correcao":  map[string]any{"erro": "não iniciada"},
		})
		return
	}
	cor, _ := d.CorrecaoS.VerProfessor(c.Request.Context(), user, sub.ID)
	_ = cor
	c.JSON(http.StatusCreated, sub)
}

// MinhaSubmissao: GET /v1/submissoes/:id — o aluno lê a própria entrega
// (metadados + snapshot); os demais casos são 404 (sem revelar existência).
func (d *Dependencies) MinhaSubmissao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	sub, err := d.SubmissaoS.Minha(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, sub)
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
		"total":      len(lista),
		"submissoes": lista,
	})
}

// ExcluirSubmissao: DELETE /v1/submissoes/:id — professor dono ou staff exclui entrega
// do aluno. Quando a atribuição pertence ao Monitor, exclui primeiro no Monitor para
// impedir estado divergente ou reimportação indevida no próximo poll; se o Monitor
// estiver fora do ar, devolve 503 claro ao professor.
func (d *Dependencies) ExcluirSubmissao(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var atID, alID, profID int64
	var mid *string
	errQ := d.Store.Pool.QueryRow(c.Request.Context(), `
		SELECT s.atribuicao_id, s.aluno_id, a.monitor_id, t.professor_id
		FROM submissoes s
		JOIN atribuicoes a ON a.id = s.atribuicao_id
		JOIN turmas t ON t.id = a.turma_id
		WHERE s.id = $1`, id).Scan(&atID, &alID, &mid, &profID)
	if errQ != nil || (!user.IsStaff() && user.ID != profID) {
		platform.ErrorBody(c, platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Submissão não encontrada."))
		return
	}

	if mid != nil && *mid != "" && d.Monitor != nil && d.Monitor.Ligado() {
		if errMon := d.Monitor.ExcluirSubmissao(c.Request.Context(), *mid, alID); errMon != nil {
			var apiErr *platform.APIError
			// Ignora apenas ausência no Monitor (ex.: submissão só local); se o Monitor estiver fora do ar (503), aborta sem excluir parcialmente.
			if !errors.As(errMon, &apiErr) || (apiErr.Status != http.StatusNotFound && apiErr.Status != http.StatusUnprocessableEntity) {
				platform.ErrorBody(c, errMon)
				return
			}
		}
	}

	if err := d.SubmissaoS.Excluir(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}

	if mid != nil && *mid != "" {
		_, _ = d.Store.Pool.Exec(c.Request.Context(), `
			DELETE FROM monitor_estado_aluno WHERE atribuicao_id = $1 AND aluno_id = $2
		`, atID, alID)
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

	// Se a atribuição está no Monitor e não possui rascunho local na API, opera diretamente no Monitor
	// (garantindo que indisponibilidade do Monitor gere 503 claro em vez de falso 422 local).
	var mid *string
	var profID int64
	var podeAtrasado bool
	var temTentativaLocal bool
	if errQ := d.Store.Pool.QueryRow(c.Request.Context(), `
		SELECT a.monitor_id, t.professor_id, a.pode_atrasado,
		       EXISTS(SELECT 1 FROM tentativas te WHERE te.atribuicao_id = a.id AND te.aluno_id = $2)
		FROM atribuicoes a
		JOIN turmas t ON t.id = a.turma_id
		WHERE a.id = $1`, atribuicaoID, alunoID).Scan(&mid, &profID, &podeAtrasado, &temTentativaLocal); errQ == nil {
		if (user.IsStaff() || user.ID == profID) && !podeAtrasado && !temTentativaLocal && mid != nil && *mid != "" && d.Monitor != nil && d.Monitor.Ligado() {
			if errMon := d.Monitor.ColetarTentativa(c.Request.Context(), *mid, alunoID); errMon != nil {
				platform.ErrorBody(c, errMon)
				return
			}
			_ = d.Monitor.PollEstadoAtribuicao(c.Request.Context(), atribuicaoID, *mid)
			c.JSON(http.StatusOK, gin.H{"ok": true, "status": "coletada_monitor"})
			return
		}
	}

	sub, err := d.SubmissaoS.Coletar(c.Request.Context(), user, atribuicaoID, alunoID)
	if err != nil {
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
	_ = d.Monitor.PollEstadoAtribuicao(c.Request.Context(), atribuicaoID, mid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MonitorExcluir: DELETE /v1/atribuicoes/:id/monitor/alunos/:aluno_id/submissao
func (d *Dependencies) MonitorExcluir(c *gin.Context) {
	user, _ := currentUser(c)
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
	// Remove também eventual espelho/submissão já sincronizada na API para manter os 2 bancos consistentes.
	var subLocalID int64
	if errS := d.Store.Pool.QueryRow(c.Request.Context(),
		`SELECT id FROM submissoes WHERE atribuicao_id = $1 AND aluno_id = $2`,
		atribuicaoID, alunoID).Scan(&subLocalID); errS == nil && subLocalID > 0 {
		_ = d.SubmissaoS.Excluir(c.Request.Context(), user, subLocalID)
	}
	_, _ = d.Store.Pool.Exec(c.Request.Context(),
		`DELETE FROM monitor_estado_aluno WHERE atribuicao_id = $1 AND aluno_id = $2`,
		atribuicaoID, alunoID)
	_ = d.Monitor.PollEstadoAtribuicao(c.Request.Context(), atribuicaoID, mid)
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
	if det != nil && det.Submissao != nil && len(det.Submissao.Arquivos) > 0 {
		for k, arq := range det.Submissao.Arquivos {
			arq.Conteudo = nil
			det.Submissao.Arquivos[k] = arq
		}
	}
	c.JSON(http.StatusOK, det)
}

// MonitorBaixarArquivo: GET /v1/atribuicoes/:id/monitor/alunos/:aluno_id/tarefas/:ordem/arquivo
func (d *Dependencies) MonitorBaixarArquivo(c *gin.Context) {
	atribuicaoID, ok := idDaRota(c)
	if !ok {
		return
	}
	alunoID, err := strconv.ParseInt(c.Param("aluno_id"), 10, 64)
	if err != nil || alunoID < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "VALIDACAO", "mensagem": "aluno_id inválido."}})
		return
	}
	ordemStr := strings.TrimSpace(c.Param("ordem"))
	if ordemStr == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "VALIDACAO", "mensagem": "ordem inválida."}})
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
	if det == nil || det.Submissao == nil || len(det.Submissao.Arquivos) == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": gin.H{"codigo": "NAO_ENCONTRADO", "mensagem": "Arquivo não encontrado."}})
		return
	}
	arq, existe := det.Submissao.Arquivos[ordemStr]
	if !existe || len(arq.Conteudo) == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": gin.H{"codigo": "NAO_ENCONTRADO", "mensagem": "Arquivo não encontrado."}})
		return
	}
	enviarArquivoDownload(c, arq.NomeArquivo, arq.MimeType, arq.Conteudo)
}

// ingerirSubmissaoMonitor: POST /v1/interna/monitor/submissoes — ingestão assíncrona/direta do Monitor.
func (d *Dependencies) ingerirSubmissaoMonitor(c *gin.Context) {
	if d.Monitor == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"erro": gin.H{"codigo": "MONITOR_DESLIGADO", "mensagem": "Integração com Monitor desligada."}})
		return
	}
	mac := c.GetHeader(contract.HeaderMAC)
	ts := c.GetHeader(contract.HeaderTime)
	maxBody := int64(64 << 20)
	if d.Cfg != nil && d.Cfg.ArquivoMaxBytes*3 > maxBody {
		maxBody = d.Cfg.ArquivoMaxBytes * 3
	}
	corpo, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBody))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "CORPO_INVALIDO", "mensagem": "Falha ao ler corpo da requisição."}})
		return
	}
	if err := d.Monitor.VerificarMAC(corpo, mac, ts); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"erro": gin.H{"codigo": "SEM_PERMISSAO", "mensagem": "Assinatura HMAC inválida ou expirada."}})
		return
	}
	var pacote contract.PacoteSubmissaoMonitor
	if err := json.Unmarshal(corpo, &pacote); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": gin.H{"codigo": "JSON_INVALIDO", "mensagem": "JSON inválido."}})
		return
	}
	subID, err := d.Monitor.IngerirSubmissao(c.Request.Context(), &pacote)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": gin.H{"codigo": "ERRO_INTERNO", "mensagem": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "submissao_id": subID})
}
