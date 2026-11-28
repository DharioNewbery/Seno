// Package submissoes — entrega do aluno (spec §5.5–5.6): pré-avaliação tem
// escolha (cancelar mantém a tentativa viva; confirmar fecha a entrega),
// bloqueio final quando a cota zera as rotas e correção automática completa
// (testes públicos E privados, nota por tarefa) na fila do Monitor.
package submissoes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/fila"
	"github.com/seno-project/seno/monitor/internal/judge0"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

// KindCorrecao e KindNotificarAPI são os kinds dos jobs persistentes na fila do Monitor.
const (
	KindCorrecao     = "correcao"
	KindNotificarAPI = "notificar_api"
)

type Service struct {
	pool   *pgxpool.Pool
	janela *tentativas.Service
	j0     *judge0.Client
	apiURL string
	secret []byte
	http   *http.Client
}

func New(pool *pgxpool.Pool, janela *tentativas.Service, j0 *judge0.Client) *Service {
	return &Service{
		pool:   pool,
		janela: janela,
		j0:     j0,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// WithAPIPush configura o envio imediato (push) de submissões e correções para a API principal.
func (s *Service) WithAPIPush(apiURL, secret string) *Service {
	s.apiURL = strings.TrimSuffix(strings.TrimSpace(apiURL), "/")
	s.secret = []byte(secret)
	return s
}

// Erros mapeados pelos handlers (§5.5).
var (
	ErrPreInvalida  = errors.New("pré-avaliação inexistente, expirada ou inválida")
	ErrAtualizada   = errors.New("tentativa atualizada após a pré-avaliação")
	ErrSemTentativa = errors.New("sem tentativa aberta")
	ErrJanela       = tentativas.ErrEncerrada // entrega fora de janela (410)
)

// Entrega é a resposta do confirmar (§5.5).
type Entrega struct {
	Status         string `json:"status"`
	NT             int    `json:"n_t"`
	Atrasada       bool   `json:"atrasada"`
	SegundosAtraso int64  `json:"segundos_atraso"`
}

type preAvaliacaoRow struct {
	id         string
	monitorID  string
	alunoID    int64
	revisaoRef int
	expirada   bool
	confirmada bool
	cancelada  bool
}

// Cancelar descarta a pré-avaliação; a tentativa continua viva (§5.5).
func (s *Service) Cancelar(ctx context.Context, mid string, aluno int64, preID string, agora time.Time) error {
	ct, err := s.pool.Exec(ctx, `UPDATE pre_avaliacao SET cancelada = true
	    WHERE id = $1 AND monitor_id = $2 AND aluno_api_id = $3
	      AND expira_em > $4 AND NOT confirmada AND NOT cancelada`,
		preID, mid, aluno, agora)
	if err != nil {
		return fmt.Errorf("submissoes: cancelar: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrPreInvalida
	}
	return nil
}

// Confirmar fecha a entrega: valida pre_id vivo + revisao_ref; congela as
// respostas, limpa os dados temporários de realização (tentativa, rascunhos,
// pré-avaliações, ensaios), agenda a correção completa (públicos + privados)
// na fila (§5.5, efeito 1–4) e transfere a submissão para a API.
func (s *Service) Confirmar(ctx context.Context, mid string, aluno int64, preID string, agora time.Time) (*Entrega, error) {
	// pré-avaliação: viva, não confirmada, não cancelada (409 senão).
	var pre preAvaliacaoRow
	err := s.pool.QueryRow(ctx, `SELECT id, monitor_id, aluno_api_id, revisao_ref,
	    expira_em < $4, confirmada, cancelada
	  FROM pre_avaliacao WHERE id = $1 AND monitor_id = $2 AND aluno_api_id = $3`,
		preID, mid, aluno, agora).Scan(
		&pre.id, &pre.monitorID, &pre.alunoID, &pre.revisaoRef,
		&pre.expirada, &pre.confirmada, &pre.cancelada)
	if err != nil {
		return nil, ErrPreInvalida
	}
	if pre.expirada || pre.confirmada || pre.cancelada {
		return nil, ErrPreInvalida
	}

	// revisão da tentativa = revisao_ref (409 ATUALIZADA senão).
	var revisao int
	var dados []byte
	var abertaEm time.Time
	err = s.pool.QueryRow(ctx, `SELECT revisao, dados, aberta_em FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&revisao, &dados, &abertaEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSemTentativa
	}
	if err != nil {
		return nil, fmt.Errorf("submissoes: tentativa: %w", err)
	}
	if revisao != pre.revisaoRef {
		return nil, ErrAtualizada
	}

	// janela recalculada: entrega possível? (encerrada definitiva → 410)
	est, err := s.janela.VerificarJanela(ctx, mid, aluno, agora)
	if err != nil {
		return nil, err // ErroInicio/ErrEncerrada
	}
	segundosAtraso := int64(0)
	if est.Atrasada {
		segundosAtraso = int64(agora.Sub(est.Prazo) / time.Second)
		if segundosAtraso < 0 {
			segundosAtraso = 0
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("submissoes: tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM submissao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&count); err != nil {
		return nil, fmt.Errorf("submissoes: contador: %w", err)
	}
	nT := count + 1

	var subID int64
	if err := tx.QueryRow(ctx, `INSERT INTO submissao
	    (monitor_id, aluno_api_id, n_t, dados, comecou_em, entregue_em, atrasada)
	    VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7) RETURNING id`,
		mid, aluno, nT, string(dados), abertaEm, agora, est.Atrasada).Scan(&subID); err != nil {
		return nil, fmt.Errorf("submissoes: gravando submissão: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO submissao_arquivo
	    (submissao_id, tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, criado_em)
	    SELECT $1, tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, $4
	    FROM tentativa_arquivo
	    WHERE monitor_id = $2 AND aluno_api_id = $3`, subID, mid, aluno, agora); err != nil {
		return nil, fmt.Errorf("submissoes: copiando arquivos da tentativa: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tentativa_arquivo
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: limpando tentativa_arquivo: %w", err)
	}
	// Limpa dados temporários gerados durante a realização da atividade.
	if _, err := tx.Exec(ctx, `DELETE FROM pre_avaliacao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: limpando pre_avaliacao: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM execucao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: limpando execucao: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM rascunho_ponto
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: rascunhos: %w", err)
	}
	// §5.5 efeito 3: apaga a tentativa.
	if _, err := tx.Exec(ctx, `DELETE FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: apagando tentativa: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO correcao
	    (submissao_id, monitor_id, aluno_api_id, status) VALUES ($1, $2, $3, 'pendente')`,
		subID, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: correcao: %w", err)
	}
	// Transactional Outbox: agenda a correção na mesma transação da entrega,
	// garantindo zero perda caso o processo reinicie logo após o commit.
	if _, err := fila.AgendaTx(ctx, tx, KindCorrecao, payloadCorrecao{
		SubmissaoID: subID,
	}, 5); err != nil {
		return nil, fmt.Errorf("submissoes: agendando correcao: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("submissoes: commit: %w", err)
	}

	// Envia imediatamente para a API com timeout curto (2s) e, caso a API esteja
	// fora do ar, grava job persistente `notificar_api` na fila para retentativa automática.
	s.notificarImediatoOuEnfileirar(mid, aluno)
	return &Entrega{Status: "entregue", NT: nT, Atrasada: est.Atrasada, SegundosAtraso: segundosAtraso}, nil
}

// Finalizado diz se a cota do aluno esgotou (§5.6): todo endpoint então
// devolve 403 FINALIZADA. Devolve também o api_id da atividade (a tela do
// portal onde acompanhar a correção).
func (s *Service) Finalizado(ctx context.Context, mid string, aluno int64) (bool, int64, error) {
	var count int
	var apiID int64
	var regrasJSON []byte
	err := s.pool.QueryRow(ctx, `SELECT
	    (SELECT count(*) FROM submissao WHERE monitor_id = $1 AND aluno_api_id = $2),
	    (carga->'carga'->'atividade'->>'api_id')::bigint,
	    carga->'regras'
	  FROM atividade WHERE monitor_id = $1`, mid, aluno).
		Scan(&count, &apiID, &regrasJSON)
	if err != nil {
		return false, 0, fmt.Errorf("submissoes: finalizado: %w", err)
	}
	var regras contract.Regras
	_ = json.Unmarshal(regrasJSON, &regras)
	if count > 0 {
		if !regras.SubmissoesMultiplas {
			return true, apiID, nil
		}
		if regras.MaxSubmissoes > 0 && count >= regras.MaxSubmissoes {
			return true, apiID, nil
		}
	}
	return false, apiID, nil
}

// ---- correção na fila ------------------------------------------------------

type payloadCorrecao struct {
	SubmissaoID int64 `json:"submissao_id"`
}

type payloadNotificarAPI struct {
	MonitorID  string `json:"monitor_id"`
	AlunoAPIID int64  `json:"aluno_api_id"`
}

func (s *Service) apiPushLigado() bool {
	return s.apiURL != "" && len(s.secret) > 0
}

// --- estrutura da correção --------------------------------------------------

type resultadoCOR struct {
	Ordem       int        `json:"ordem"`
	TarefaAPIID int64      `json:"tarefa_api_id,omitempty"`
	Veredito    string     `json:"veredito"`
	Testes      []testeCOR `json:"testes"`
	// Tarefa aberta (sem testes): nil — o professor preenche a nota manual.
	Nota *float64 `json:"nota_auto,omitempty"`
}

type testeCOR struct {
	Ordem         int     `json:"ordem"`
	TesteAPIID    int64   `json:"teste_api_id,omitempty"`
	Status        string  `json:"status"`
	Stdout        string  `json:"stdout,omitempty"`
	Stderr        string  `json:"stderr,omitempty"`
	CompileOutput string  `json:"compile_output,omitempty"`
	Tempo         float64 `json:"tempo"`
	CPU           float64 `json:"cpu,omitempty"`
	Memoria       int     `json:"memoria"`
	Privado       bool    `json:"privado"`
}

// Handler devolve o handler da fila para kind "correcao": processa TODOS
// os testes (públicos + privados), grava nota por tarefa (nota_auto) e
// transfere o pacote consolidado para a API principal.
func (s *Service) Handler() fila.Handler {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p payloadCorrecao
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("correcao: payload: %w", err)
		}

		// submissão + carga integral (PRIVADOS incluídos, §5.5).
		var mid string
		var aluno int64
		var dados []byte
		var carga []byte
		if err := s.pool.QueryRow(ctx, `SELECT sb.monitor_id, sb.aluno_api_id, sb.dados,
		    a.carga FROM submissao sb JOIN atividade a USING (monitor_id)
		    WHERE sb.id = $1`, p.SubmissaoID).Scan(&mid, &aluno, &dados, &carga); err != nil {
			return fmt.Errorf("correcao: submissão %d: %w", p.SubmissaoID, err)
		}
		var t struct {
			Carga contract.Carga `json:"carga"`
		}
		if err := json.Unmarshal(carga, &t); err != nil {
			return fmt.Errorf("correcao: carga ilegível: %w", err)
		}
		respostas := map[string]any{}
		_ = json.Unmarshal(dados, &respostas)

		precisaJudge0 := false
		for _, tarefa := range t.Carga.Tarefas {
			if contract.NormalizarTipoTarefa(tarefa.Tipo) != contract.TipoTarefaArquivo && len(tarefa.Testes) > 0 {
				precisaJudge0 = true
				break
			}
		}
		if precisaJudge0 && (s.j0 == nil || !s.j0.Ligada()) {
			// Sem Judge0 configurado: mantém correção 'pendente' (avaliação manual na API)
			// e garante que o pacote da submissão seja sincronizado com a API.
			s.enfileirarOuNotificarAPI(ctx, mid, aluno)
			return nil
		}

		_, err := s.pool.Exec(ctx, `UPDATE correcao SET status = 'executando', concluida_em = NULL
		    WHERE submissao_id = $1`, p.SubmissaoID)
		if err != nil {
			return fmt.Errorf("correcao: status: %w", err)
		}
		reverterPendente := func() {
			revCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = s.pool.Exec(revCtx, `UPDATE correcao SET status = 'pendente'
			    WHERE submissao_id = $1 AND status = 'executando'`, p.SubmissaoID)
		}

		tarefas := make([]resultadoCOR, 0, len(t.Carga.Tarefas))
		notaTotal := 0.0
		for _, tarefa := range t.Carga.Tarefas {
			if contract.NormalizarTipoTarefa(tarefa.Tipo) == contract.TipoTarefaArquivo {
				var existeArq bool
				_ = s.pool.QueryRow(ctx, `SELECT EXISTS(
					SELECT 1 FROM submissao_arquivo WHERE submissao_id = $1 AND tarefa_ordem = $2
				)`, p.SubmissaoID, tarefa.Ordem).Scan(&existeArq)
				veredito := "sem_arquivo"
				if existeArq {
					veredito = "arquivo_entregue"
				}
				tarefas = append(tarefas, resultadoCOR{
					Ordem:       tarefa.Ordem,
					TarefaAPIID: tarefa.TarefaAPIID,
					Veredito:    veredito,
				})
				continue
			}
			if len(tarefa.Testes) == 0 {
				// Tarefa aberta (sem casos de teste): nota manual pelo professor.
				tarefas = append(tarefas, resultadoCOR{
					Ordem:       tarefa.Ordem,
					TarefaAPIID: tarefa.TarefaAPIID,
					Veredito:    "manual",
				})
				continue
			}
			codigo := ""
			if v, ok := respostas[fmt.Sprint(tarefa.Ordem)]; ok {
				if s, ok := v.(string); ok {
					codigo = s
				}
			}
			idLinguagem, ok := judge0.LinguagemDo(tarefa.Linguagem)
			if !ok {
				reverterPendente()
				return fmt.Errorf("correcao: linguagem %s", tarefa.Linguagem)
			}
			lim := tarefa.Limites
			if lim == nil {
				lim = &contract.Limites{TempoCPUMs: 5000, TempoTotalMs: 10000, MemoriaMB: 128}
			}
			itens := make([]judge0.Submissao, len(tarefa.Testes))
			for i, te := range tarefa.Testes {
				itens[i] = judge0.Submissao{
					LanguageID:     idLinguagem,
					SourceCode:     codigo,
					Stdin:          te.Stdin,
					ExpectedOutput: te.StdoutEsperado,
					CPUTimeSec:     float64(lim.TempoCPUMs) / 1000,
					WallTimeSec:    float64(lim.TempoTotalMs) / 1000,
					MemoriaKB:      lim.MemoriaMB * 1024,
				}
			}
			callCtx, cancelCall := context.WithTimeout(ctx, 30*time.Second)
			resps, err := s.j0.EnviarBatchSincrono(callCtx, itens)
			cancelCall()
			if err != nil {
				reverterPendente()
				return fmt.Errorf("correcao: judge0 (tarefa %d): %w", tarefa.Ordem, err)
			}

			r := resultadoCOR{
				Ordem:       tarefa.Ordem,
				TarefaAPIID: tarefa.TarefaAPIID,
				Veredito:    "accepted",
			}
			acertos, total := 0, 0
			for i, te := range tarefa.Testes {
				total++
				resp := resps[i]
				st := "judge0:" + strconv.Itoa(resp.StatusID())
				if resp.StatusID() == judge0.StatusAcceptedID {
					st = "accepted"
					acertos++
				} else if resp.StatusID() == 1 || resp.StatusID() == 2 {
					reverterPendente()
					return fmt.Errorf("correcao: judge0 em fila (tarefa %d)", tarefa.Ordem)
				}
				cpuVal := tempo(resp.CPUTime)
				if cpuVal == 0 {
					cpuVal = tempo(resp.Time)
				}
				r.Testes = append(r.Testes, testeCOR{
					Ordem:         i + 1,
					TesteAPIID:    te.TesteAPIID,
					Status:        st,
					Stdout:        resp.Stdout,
					Stderr:        resp.Stderr,
					CompileOutput: resp.CompileOutput,
					Tempo:         tempo(resp.Time),
					CPU:           cpuVal,
					Memoria:       memoriaInt(resp.Memory),
					Privado:       !te.Publico,
				})
			}
			if total > 0 {
				nota := (float64(acertos) / float64(total)) * tarefa.ValorPts
				notaTotal += nota
				r.Nota = &nota
			}
			if acertos != total {
				r.Veredito = "judge0:partial"
			}
			tarefas = append(tarefas, r)
		}

		resultados, err := json.Marshal(map[string]any{
			"tarefas": tarefas, "nota_auto": notaTotal,
		})
		if err != nil {
			reverterPendente()
			return fmt.Errorf("correcao: relatório: %w", err)
		}
		if _, err := s.pool.Exec(ctx, `UPDATE correcao
		    SET status = 'concluida', nota_auto = $2, resultados = $3::jsonb,
		        concluida_em = now()
		    WHERE submissao_id = $1`, p.SubmissaoID, notaTotal, string(resultados)); err != nil {
			return fmt.Errorf("correcao: gravando: %w", err)
		}
		slog.Info("correção concluída", "submissao_id", p.SubmissaoID, "nota", notaTotal)

		// Tenta envio imediato para a API; se a API estiver fora do ar, agenda job
		// persistente para retentativa sem precisar reexecutar os testes no Judge0.
		s.enfileirarOuNotificarAPI(ctx, mid, aluno)
		return nil
	}
}

func tempo(t judge0.NumStr) float64 {
	var f float64
	if _, err := fmt.Sscan(string(t), &f); err == nil {
		return f
	}
	return 0
}

func memoriaInt(m judge0.NumStr) int {
	var n int
	if _, err := fmt.Sscan(string(m), &n); err == nil {
		return n
	}
	return 0
}

// Reexecutar reinicia a avaliação automática da submissão do aluno no Monitor.
func (s *Service) Reexecutar(ctx context.Context, mid string, aluno int64) error {
	var subID int64
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT s.id, c.status
		FROM submissao s
		JOIN correcao c ON c.submissao_id = s.id
		WHERE s.monitor_id = $1 AND s.aluno_api_id = $2
		ORDER BY s.id DESC LIMIT 1`, mid, aluno).Scan(&subID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("submissão não encontrada")
	}
	if err != nil {
		return fmt.Errorf("submissoes: buscar correcao: %w", err)
	}
	if status == "executando" {
		return errors.New("correção já está em execução")
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE correcao
		SET status = 'pendente', nota_auto = NULL, resultados = NULL, concluida_em = NULL
		WHERE submissao_id = $1`, subID); err != nil {
		return fmt.Errorf("submissoes: resetar correcao: %w", err)
	}

	if _, err := fila.Agenda(ctx, s.pool, KindCorrecao, map[string]any{
		"submissao_id": subID,
	}, 5); err != nil {
		return fmt.Errorf("submissoes: reenfileirar correcao: %w", err)
	}
	return nil
}

// Excluir apaga a submissão no Monitor e restaura a tentativa pré-preenchida.
func (s *Service) Excluir(ctx context.Context, mid string, aluno int64) error {
	var subID int64
	var dados []byte
	var comecouEm *time.Time
	var entregueEm time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, dados, comecou_em, entregue_em
		FROM submissao
		WHERE monitor_id = $1 AND aluno_api_id = $2
		ORDER BY id DESC LIMIT 1`, mid, aluno).Scan(&subID, &dados, &comecouEm, &entregueEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("submissão não encontrada")
	}
	if err != nil {
		return fmt.Errorf("submissoes: buscar submissao: %w", err)
	}

	inicioReal := entregueEm
	if comecouEm != nil && !comecouEm.IsZero() {
		inicioReal = *comecouEm
	}

	// Calcula datas para restaurar a tentativa
	var duracaoSeg int64
	var fimPrevisto *time.Time
	_ = s.pool.QueryRow(ctx, `
		SELECT (carga->'janela'->>'duracao_seg')::bigint,
		       (carga->'janela'->>'prazo')::timestamptz
		FROM atividade WHERE monitor_id = $1`, mid).Scan(&duracaoSeg, &fimPrevisto)

	var fimReal time.Time
	if fimPrevisto != nil {
		fimReal = *fimPrevisto
	} else {
		fimReal = inicioReal.Add(24 * time.Hour)
	}
	if duracaoSeg > 0 {
		t := inicioReal.Add(time.Duration(duracaoSeg) * time.Second)
		if t.Before(fimReal) {
			fimReal = t
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("submissoes: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Restaura a tentativa com os dados da submissão excluída
	if _, err := tx.Exec(ctx, `
		INSERT INTO tentativa (monitor_id, aluno_api_id, revisao, dados, aberta_em, fim_real)
		VALUES ($1, $2, 1, $3::jsonb, $4, $5)
		ON CONFLICT (monitor_id, aluno_api_id) DO UPDATE
		SET dados = EXCLUDED.dados, revisao = 1, aberta_em = EXCLUDED.aberta_em, fim_real = EXCLUDED.fim_real`,
		mid, aluno, string(dados), inicioReal, fimReal); err != nil {
		return fmt.Errorf("submissoes: restaurar tentativa: %w", err)
	}

	// Restaura os arquivos submetidos de volta para tentativa_arquivo
	if _, err := tx.Exec(ctx, `
		INSERT INTO tentativa_arquivo (monitor_id, aluno_api_id, tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, atualizado_em)
		SELECT $1, $2, tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, now()
		FROM submissao_arquivo
		WHERE submissao_id = $3
		ON CONFLICT (monitor_id, aluno_api_id, tarefa_ordem) DO UPDATE SET
			tarefa_api_id = EXCLUDED.tarefa_api_id,
			nome_arquivo = EXCLUDED.nome_arquivo,
			mime_type = EXCLUDED.mime_type,
			tamanho_bytes = EXCLUDED.tamanho_bytes,
			sha256 = EXCLUDED.sha256,
			conteudo = EXCLUDED.conteudo,
			atualizado_em = EXCLUDED.atualizado_em`,
		mid, aluno, subID); err != nil {
		return fmt.Errorf("submissoes: restaurar tentativa_arquivo: %w", err)
	}

	// Deleta a submissão (correção e submissao_arquivo caem em cascata)
	if _, err := tx.Exec(ctx, `DELETE FROM submissao WHERE id = $1`, subID); err != nil {
		return fmt.Errorf("submissoes: delete submissao: %w", err)
	}

	// Limpa pré-avaliação
	if _, err := tx.Exec(ctx, `
		DELETE FROM pre_avaliacao WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return fmt.Errorf("submissoes: delete pre_avaliacao: %w", err)
	}

	return tx.Commit(ctx)
}

// Coletar encerra manualmente a tentativa do aluno atrasado sem entrega.
func (s *Service) Coletar(ctx context.Context, mid string, aluno int64, agora time.Time) (*Entrega, error) {
	var revisao int
	var dados []byte
	var abertaEm time.Time
	var fimReal *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT revisao, dados, aberta_em, fim_real FROM tentativa
		WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&revisao, &dados, &abertaEm, &fimReal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSemTentativa
	}
	if err != nil {
		return nil, fmt.Errorf("submissoes: buscar tentativa: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("submissoes: tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM submissao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&count); err != nil {
		return nil, fmt.Errorf("submissoes: contador: %w", err)
	}
	nT := count + 1

	var subID int64
	if err := tx.QueryRow(ctx, `INSERT INTO submissao
	    (monitor_id, aluno_api_id, n_t, dados, comecou_em, entregue_em, atrasada)
	    VALUES ($1, $2, $3, $4::jsonb, $5, $6, true) RETURNING id`,
		mid, aluno, nT, string(dados), abertaEm, agora).Scan(&subID); err != nil {
		return nil, fmt.Errorf("submissoes: gravando submissão: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO submissao_arquivo
	    (submissao_id, tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, criado_em)
	    SELECT $1, tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, $4
	    FROM tentativa_arquivo
	    WHERE monitor_id = $2 AND aluno_api_id = $3`, subID, mid, aluno, agora); err != nil {
		return nil, fmt.Errorf("submissoes: copiando arquivos da tentativa na coleta: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tentativa_arquivo
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: limpando tentativa_arquivo na coleta: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM pre_avaliacao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: limpando pre_avaliacao: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM execucao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: limpando execucao: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM rascunho_ponto
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: rascunhos: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: apagando tentativa: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO correcao
	    (submissao_id, monitor_id, aluno_api_id, status) VALUES ($1, $2, $3, 'pendente')`,
		subID, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: correcao: %w", err)
	}
	if _, err := fila.AgendaTx(ctx, tx, KindCorrecao, payloadCorrecao{
		SubmissaoID: subID,
	}, 5); err != nil {
		return nil, fmt.Errorf("submissoes: agendando correcao na coleta: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("submissoes: commit: %w", err)
	}

	s.notificarImediatoOuEnfileirar(mid, aluno)
	return &Entrega{Status: "entregue", NT: nT, Atrasada: true, SegundosAtraso: 0}, nil
}

// ObterDetalhes busca a submissão, correção e metadados das tarefas da atividade
// no formato contract.PacoteSubmissaoMonitor (compatível tanto com a sincronização
// para a API quanto com a visualização detalhada).
func (s *Service) ObterDetalhes(ctx context.Context, mid string, aluno int64) (*contract.PacoteSubmissaoMonitor, error) {
	var apiID *int64
	var cargaJSON []byte
	if err := s.pool.QueryRow(ctx, `SELECT api_id, carga FROM atividade WHERE monitor_id = $1`, mid).Scan(&apiID, &cargaJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("atividade não encontrada no Monitor")
		}
		return nil, err
	}

	var envelope struct {
		APIID int64          `json:"api_id"`
		Carga contract.Carga `json:"carga"`
	}
	_ = json.Unmarshal(cargaJSON, &envelope)
	atribID := envelope.APIID
	if atribID == 0 && apiID != nil {
		atribID = *apiID
	}

	det := &contract.PacoteSubmissaoMonitor{
		MonitorID:       mid,
		AtribuicaoAPIID: atribID,
		AlunoAPIID:      aluno,
		Atividade: contract.AtividadeInfoMonitor{
			Nome:    envelope.Carga.Atividade.Nome,
			Tarefas: make([]contract.TarefaInfoMonitor, 0, len(envelope.Carga.Tarefas)),
		},
	}
	ordemParaTarefaID := make(map[int]int64, len(envelope.Carga.Tarefas))
	for _, t := range envelope.Carga.Tarefas {
		ordemParaTarefaID[t.Ordem] = t.TarefaAPIID
		det.Atividade.Tarefas = append(det.Atividade.Tarefas, contract.TarefaInfoMonitor{
			Ordem:         t.Ordem,
			TarefaAPIID:   t.TarefaAPIID,
			Nome:          t.Nome,
			Enunciado:     t.Enunciado,
			Tipo:          contract.NormalizarTipoTarefa(t.Tipo),
			ConfigArquivo: t.ConfigArquivo,
			Linguagem:     t.Linguagem,
			ValorPts:      t.ValorPts,
		})
	}

	var (
		subID      int64
		nT         int
		dados      []byte
		comecouEm  *time.Time
		entregueEm time.Time
		atrasada   bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, n_t, dados, comecou_em, entregue_em, atrasada
		FROM submissao
		WHERE monitor_id = $1 AND aluno_api_id = $2
		ORDER BY n_t DESC LIMIT 1`, mid, aluno).Scan(&subID, &nT, &dados, &comecouEm, &entregueEm, &atrasada)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("obter submissao: %w", err)
	}

	if err == nil {
		respostas := map[string]string{}
		var rawMap map[string]any
		if json.Unmarshal(dados, &rawMap) == nil {
			for k, v := range rawMap {
				if str, ok := v.(string); ok {
					respostas[k] = str
				} else if b, err := json.Marshal(v); err == nil {
					respostas[k] = string(b)
				}
			}
		}
		inicioSub := entregueEm
		if comecouEm != nil && !comecouEm.IsZero() {
			inicioSub = *comecouEm
		}

		det.Submissao = &contract.SubmissaoMonitor{
			ID:         subID,
			AlunoID:    aluno,
			NT:         nT,
			ComecouEm:  inicioSub,
			EntregueEm: entregueEm,
			Atrasada:   atrasada,
			Respostas:  respostas,
			Arquivos:   s.carregarArquivosSubmissao(ctx, subID, ordemParaTarefaID),
		}

		var (
			corStatus   string
			notaAuto    *float64
			concluidaEm *time.Time
			resultados  []byte
		)
		errCor := s.pool.QueryRow(ctx, `
			SELECT status, nota_auto, concluida_em, resultados
			FROM correcao
			WHERE submissao_id = $1`, subID).Scan(&corStatus, &notaAuto, &concluidaEm, &resultados)
		if errCor == nil {
			var parsed struct {
				Tarefas []contract.TarefaCorrecaoMonitor `json:"tarefas"`
			}
			if len(resultados) > 0 {
				_ = json.Unmarshal(resultados, &parsed)
				for i := range parsed.Tarefas {
					if parsed.Tarefas[i].TarefaAPIID == 0 {
						parsed.Tarefas[i].TarefaAPIID = ordemParaTarefaID[parsed.Tarefas[i].Ordem]
					}
				}
			}
			det.Correcao = &contract.CorrecaoMonitor{
				Status:      corStatus,
				NotaAuto:    notaAuto,
				ConcluidaEm: concluidaEm,
				Resultados:  resultados,
				Tarefas:     parsed.Tarefas,
			}
		}
	} else {
		// Verificar se há tentativa aberta (rascunho)
		var (
			revisao int
			dadosT  []byte
		)
		if errT := s.pool.QueryRow(ctx, `SELECT revisao, dados FROM tentativa WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&revisao, &dadosT); errT == nil {
			respostas := map[string]string{}
			var rawMap map[string]any
			if json.Unmarshal(dadosT, &rawMap) == nil {
				for k, v := range rawMap {
					if str, ok := v.(string); ok {
						respostas[k] = str
					}
				}
			}
			det.Submissao = &contract.SubmissaoMonitor{
				AlunoID:   aluno,
				Respostas: respostas,
				Arquivos:  s.carregarArquivosTentativa(ctx, mid, aluno, ordemParaTarefaID),
			}
		}
	}

	return det, nil
}

func (s *Service) carregarArquivosSubmissao(ctx context.Context, subID int64, ordemParaTarefaID map[int]int64) map[string]contract.ArquivoSubmetidoMonitor {
	rows, err := s.pool.Query(ctx, `
		SELECT tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, criado_em
		FROM submissao_arquivo
		WHERE submissao_id = $1
		ORDER BY tarefa_ordem`, subID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out map[string]contract.ArquivoSubmetidoMonitor
	for rows.Next() {
		var a contract.ArquivoSubmetidoMonitor
		var criadoEm time.Time
		if err := rows.Scan(&a.TarefaOrdem, &a.TarefaAPIID, &a.NomeArquivo, &a.MimeType, &a.TamanhoBytes, &a.Sha256, &a.Conteudo, &criadoEm); err == nil {
			if a.TarefaAPIID == 0 {
				a.TarefaAPIID = ordemParaTarefaID[a.TarefaOrdem]
			}
			a.EnviadoEm = criadoEm.UTC().Format(time.RFC3339)
			if out == nil {
				out = make(map[string]contract.ArquivoSubmetidoMonitor)
			}
			out[strconv.Itoa(a.TarefaOrdem)] = a
		}
	}
	return out
}

func (s *Service) carregarArquivosTentativa(ctx context.Context, mid string, aluno int64, ordemParaTarefaID map[int]int64) map[string]contract.ArquivoSubmetidoMonitor {
	rows, err := s.pool.Query(ctx, `
		SELECT tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, atualizado_em
		FROM tentativa_arquivo
		WHERE monitor_id = $1 AND aluno_api_id = $2
		ORDER BY tarefa_ordem`, mid, aluno)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out map[string]contract.ArquivoSubmetidoMonitor
	for rows.Next() {
		var a contract.ArquivoSubmetidoMonitor
		var atualizadoEm time.Time
		if err := rows.Scan(&a.TarefaOrdem, &a.TarefaAPIID, &a.NomeArquivo, &a.MimeType, &a.TamanhoBytes, &a.Sha256, &a.Conteudo, &atualizadoEm); err == nil {
			if a.TarefaAPIID == 0 {
				a.TarefaAPIID = ordemParaTarefaID[a.TarefaOrdem]
			}
			a.EnviadoEm = atualizadoEm.UTC().Format(time.RFC3339)
			if out == nil {
				out = make(map[string]contract.ArquivoSubmetidoMonitor)
			}
			out[strconv.Itoa(a.TarefaOrdem)] = a
		}
	}
	return out
}

// NotificarAPIHandler devolve o worker da fila para sincronizar pacotes pendentes
// de submissão/correção com a API principal (com retentativa automática se a API estiver fora do ar).
func (s *Service) NotificarAPIHandler() fila.Handler {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p payloadNotificarAPI
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("notificar_api: payload: %w", err)
		}
		if !s.apiPushLigado() || p.MonitorID == "" || p.AlunoAPIID == 0 {
			return nil
		}
		return s.enviarPacoteAPI(ctx, p.MonitorID, p.AlunoAPIID)
	}
}

// MarcarFalhaJob garante que uma correção cujas retentativas da fila esgotaram
// transicione para 'falhou' (nunca ficando presa em 'pendente'/'executando') e notifique a API.
func (s *Service) MarcarFalhaJob(ctx context.Context, payload json.RawMessage) {
	var p payloadCorrecao
	if err := json.Unmarshal(payload, &p); err != nil || p.SubmissaoID == 0 {
		return
	}
	var mid string
	var aluno int64
	err := s.pool.QueryRow(ctx, `UPDATE correcao
	    SET status = 'falhou'
	    WHERE submissao_id = $1 AND status <> 'concluida'
	    RETURNING monitor_id, aluno_api_id`, p.SubmissaoID).Scan(&mid, &aluno)
	if err == nil && s.apiPushLigado() {
		_, _ = fila.Agenda(ctx, s.pool, KindNotificarAPI, payloadNotificarAPI{
			MonitorID:  mid,
			AlunoAPIID: aluno,
		}, 6)
	}
}

// ReconciliarPendentes garante auto-recuperação após quedas prolongadas do Judge0 ou da API:
//  1. Destrava correções 'executando' há mais de 12 min sem job ativo;
//  2. Reenfileira correções 'pendente' órfãs ou 'falhou' quando o Judge0 volta a responder;
//  3. Reenfileira jobs 'notificar_api' que esgotaram tentativas quando a API volta ao ar.
func (s *Service) ReconciliarPendentes(ctx context.Context) {
	// 1. Correções 'executando' órfãs (> 12 min sem job executando).
	_, _ = s.pool.Exec(ctx, `UPDATE correcao c
	    SET status = 'pendente'
	    WHERE c.status = 'executando'
	      AND c.criada_em < now() - interval '12 minutes'
	      AND NOT EXISTS (
	          SELECT 1 FROM jobs j
	          WHERE j.kind = $1 AND j.status IN ('agendado', 'executando')
	            AND (j.payload->>'submissao_id')::bigint = c.submissao_id
	      )`, KindCorrecao)

	// 2. Se Judge0 estiver saudável, reenfileira correções pendentes sem job ou que falharam por queda do Judge0.
	if s.j0 != nil && s.j0.Ligada() && s.j0.Saude(ctx) == nil {
		rows, err := s.pool.Query(ctx, `SELECT c.submissao_id
		    FROM correcao c
		    WHERE c.status IN ('pendente', 'falhou')
		      AND NOT EXISTS (
		          SELECT 1 FROM jobs j
		          WHERE j.kind = $1 AND j.status IN ('agendado', 'executando')
		            AND (j.payload->>'submissao_id')::bigint = c.submissao_id
		      )
		    ORDER BY c.criada_em ASC LIMIT 10`, KindCorrecao)
		if err == nil {
			var ids []int64
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				_, _ = s.pool.Exec(ctx, `UPDATE correcao SET status = 'pendente' WHERE submissao_id = $1 AND status = 'falhou'`, id)
				_, _ = fila.Agenda(ctx, s.pool, KindCorrecao, payloadCorrecao{SubmissaoID: id}, 4)
			}
		}
	}

	// 3. Se API estiver saudável, reativa jobs de sincronização que esgotaram tentativas durante queda longa da API.
	if s.apiPushLigado() && s.apiSaudavel(ctx) {
		_, _ = s.pool.Exec(ctx, `UPDATE jobs
		    SET status = 'agendado', attempts = 0, run_after = now(), updated_at = now()
		    WHERE kind = $1 AND status = 'falhou'`, KindNotificarAPI)
	}
}

func (s *Service) apiSaudavel(ctx context.Context) bool {
	if !s.apiPushLigado() {
		return false
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pingCtx, http.MethodGet, s.apiURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Service) notificarImediatoOuEnfileirar(mid string, aluno int64) {
	if !s.apiPushLigado() {
		return
	}
	pushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.enviarPacoteAPI(pushCtx, mid, aluno); err != nil {
		slog.Warn("monitor: API indisponível na entrega; agendando sincronização persistente",
			"monitor_id", mid, "aluno", aluno, "err", err)
		qCtx, qCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer qCancel()
		_, _ = fila.Agenda(qCtx, s.pool, KindNotificarAPI, payloadNotificarAPI{
			MonitorID:  mid,
			AlunoAPIID: aluno,
		}, 6)
	}
}

func (s *Service) enfileirarOuNotificarAPI(ctx context.Context, mid string, aluno int64) {
	if !s.apiPushLigado() {
		return
	}
	if err := s.enviarPacoteAPI(ctx, mid, aluno); err != nil {
		slog.Warn("monitor: API indisponível no push imediato; agendando retentativa persistente",
			"monitor_id", mid, "aluno", aluno, "err", err)
		_, _ = fila.Agenda(ctx, s.pool, KindNotificarAPI, payloadNotificarAPI{
			MonitorID:  mid,
			AlunoAPIID: aluno,
		}, 6)
	}
}

// enviarPacoteAPI envia o pacote da submissão e correção para a API principal via HMAC.
// Devolve erro quando a API está inacessível ou responde erro transitório, permitindo retentativa na fila.
func (s *Service) enviarPacoteAPI(ctx context.Context, mid string, aluno int64) error {
	if !s.apiPushLigado() {
		return nil
	}
	pacote, err := s.ObterDetalhes(ctx, mid, aluno)
	if err != nil {
		return err
	}
	if pacote == nil || pacote.Submissao == nil || pacote.Submissao.ID == 0 {
		return nil
	}
	corpo, err := json.Marshal(pacote)
	if err != nil {
		return err
	}
	mac, ts := contract.AssinarMAC(s.secret, time.Now(), corpo)
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost,
		s.apiURL+"/v1/interna/monitor/submissoes", bytes.NewReader(corpo))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, ts)
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("monitor push api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("monitor push api: status %d", resp.StatusCode)
	}
	return nil
}
