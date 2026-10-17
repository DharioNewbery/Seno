// Package submissoes — entrega do aluno (spec §5.5–5.6): pré-avaliação tem
// escolha (cancelar mantém a tentativa viva; confirmar fecha a entrega),
// bloqueio final quando a cota zera as rotas e correção automática completa
// (testes públicos E privados, nota por tarefa) na fila do Monitor.
package submissoes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/fila"
	"github.com/seno-project/seno/monitor/internal/judge0"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

// KindCorrecao é o kind do job de correção na fila.
const KindCorrecao = "correcao"

type Service struct {
	pool   *pgxpool.Pool
	janela *tentativas.Service
	j0     *judge0.Client
}

func New(pool *pgxpool.Pool, janela *tentativas.Service, j0 *judge0.Client) *Service {
	return &Service{pool: pool, janela: janela, j0: j0}
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
// respostas, anexa a pré-avaliação, APAGA a tentativa e agenda a correção
// completa (públicos + privados) na fila (§5.5, efeito 1–4).
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
	err = s.pool.QueryRow(ctx, `SELECT revisao, dados FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&revisao, &dados)
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
	    (monitor_id, aluno_api_id, n_t, dados, entregue_em, atrasada)
	    VALUES ($1, $2, $3, $4::jsonb, $5, $6) RETURNING id`,
		mid, aluno, nT, string(dados), agora, est.Atrasada).Scan(&subID); err != nil {
		return nil, fmt.Errorf("submissoes: gravando submissão: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE pre_avaliacao SET submissao_id = $2, confirmada = true
	    WHERE id = $1`, preID, subID); err != nil {
		return nil, fmt.Errorf("submissoes: anexando pré: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM rascunho_ponto
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: rascunhos: %w", err)
	}
	// §5.5 efeito 3: apaga a tentativa (comportamento de hoje).
	if _, err := tx.Exec(ctx, `DELETE FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: apagando tentativa: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO correcao
	    (submissao_id, monitor_id, aluno_api_id, status) VALUES ($1, $2, $3, 'pendente')`,
		subID, mid, aluno); err != nil {
		return nil, fmt.Errorf("submissoes: correcao: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("submissoes: commit: %w", err)
	}

	if _, err := fila.Agenda(ctx, s.pool, KindCorrecao, map[string]any{
		"submissao_id": subID,
	}, 5); err != nil {
		// entrega já registrada: a correção pode ser reexecutada (§5 Fase 8)
		slog.Error("submissoes: enfileirando correção", "err", err)
	}
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

// --- estrutura da correção --------------------------------------------------

type resultadoCOR struct {
	Ordem    int        `json:"ordem"`
	Veredito string     `json:"veredito"`
	Testes   []testeCOR `json:"testes"`
	// Tarefa aberta (sem testes): nil — o professor preenche a nota manual.
	Nota *float64 `json:"nota_auto,omitempty"`
}

type testeCOR struct {
	Ordem   int     `json:"ordem"`
	Status  string  `json:"status"`
	Tempo   float64 `json:"tempo"`
	Memoria int     `json:"memoria"`
	Privado bool    `json:"privado"`
}

// HandlerHub devolve o handler da fila para kind "correcao": processa TODOS
// os testes (públicos + privados) e grava nota por tarefa (nota_auto).
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
			return fmt.Errorf("correcao: carga ilegígível: %w", err)
		}
		respostas := map[string]any{}
		_ = json.Unmarshal(dados, &respostas)

		_, err := s.pool.Exec(ctx, `UPDATE correcao SET status = 'executando', concluida_em = NULL
		    WHERE submissao_id = $1`, p.SubmissaoID)
		if err != nil {
			return fmt.Errorf("correcao: status: %w", err)
		}

		tarefas := make([]resultadoCOR, 0, len(t.Carga.Tarefas))
		notaTotal := 0.0
		for _, tarefa := range t.Carga.Tarefas {
			codigo := ""
			if v, ok := respostas[fmt.Sprint(tarefa.Ordem)]; ok {
				if s, ok := v.(string); ok {
					codigo = s
				}
			}
			idLinguagem, ok := judge0.LinguagemDo(tarefa.Linguagem)
			if !ok {
				return fmt.Errorf("correcao: linguagem %s", tarefa.Linguagem)
			}
			lim := tarefa.Limites
			if lim == nil {
				lim = &contract.Limites{TempoCPUMs: 5000, TempoTotalMs: 10000, MemoriaMB: 128}
			}
			r := resultadoCOR{Ordem: tarefa.Ordem, Veredito: "accepted"}
			acertos, total := 0, 0
			for i, te := range tarefa.Testes {
				total++
				resp, err := s.j0.EnviarSync(ctx, judge0.Submissao{
					LanguageID:     idLinguagem,
					SourceCode:     codigo,
					Stdin:          te.Stdin,
					ExpectedOutput: te.StdoutEsperado,
					CPUTimeSec:     float64(lim.TempoCPUMs) / 1000,
					WallTimeSec:    float64(lim.TempoTotalMs) / 1000,
					MemoriaKB:      lim.MemoriaMB * 1024,
				})
				if err != nil {
					return fmt.Errorf("correcao: judge0 (tarefa %d, teste %d): %w", tarefa.Ordem, i+1, err)
				}
				st := "judge0:" + strconv.Itoa(resp.StatusID())
				if resp.StatusID() == judge0.StatusAcceptedID {
					st = "accepted"
					acertos++
				} else if resp.StatusID() == 1 || resp.StatusID() == 2 {
					// fila do judge0 estágegada: retente (o handler re-executa)
					return fmt.Errorf("correcao: judge0 em fila (tarefa %d)", tarefa.Ordem)
				}
				r.Testes = append(r.Testes, testeCOR{
					Ordem: i + 1, Status: st, Tempo: tempo(resp.Time),
					Memoria: memoriaInt(resp.Memory), Privado: !te.Publico,
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
			return fmt.Errorf("correcao: relatório: %w", err)
		}
		if _, err := s.pool.Exec(ctx, `UPDATE correcao
		    SET status = 'concluida', nota_auto = $2, resultados = $3::jsonb,
		        concluida_em = now()
		    WHERE submissao_id = $1`, p.SubmissaoID, notaTotal, string(resultados)); err != nil {
			return fmt.Errorf("correcao: gravando: %w", err)
		}
		slog.Info("correção concluída", "submissao_id", p.SubmissaoID, "nota", notaTotal)
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
	var entregueEm time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, dados, entregue_em
		FROM submissao
		WHERE monitor_id = $1 AND aluno_api_id = $2
		ORDER BY id DESC LIMIT 1`, mid, aluno).Scan(&subID, &dados, &entregueEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("submissão não encontrada")
	}
	if err != nil {
		return fmt.Errorf("submissoes: buscar submissao: %w", err)
	}

	// Calcula datas para restaurar a tentativa
	var duracaoSeg int64
	var fimPrevisto *time.Time
	_ = s.pool.QueryRow(ctx, `
		SELECT (carga->'regras'->>'duracao_s')::bigint,
		       (carga->'regras'->>'fim')::timestamptz
		FROM atividade WHERE monitor_id = $1`, mid).Scan(&duracaoSeg, &fimPrevisto)

	fimReal := fimPrevisto
	if duracaoSeg > 0 {
		t := entregueEm.Add(time.Duration(duracaoSeg) * time.Second)
		if fimReal == nil || t.Before(*fimReal) {
			fimReal = &t
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
		SET dados = EXCLUDED.dados, revisao = 1`,
		mid, aluno, string(dados), entregueEm, fimReal); err != nil {
		return fmt.Errorf("submissoes: restaurar tentativa: %w", err)
	}

	// Deleta a submissão (correção cai em cascata)
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
	    (monitor_id, aluno_api_id, n_t, dados, entregue_em, atrasada)
	    VALUES ($1, $2, $3, $4::jsonb, $5, true) RETURNING id`,
		mid, aluno, nT, string(dados), agora).Scan(&subID); err != nil {
		return nil, fmt.Errorf("submissoes: gravando submissão: %w", err)
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
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("submissoes: commit: %w", err)
	}

	if _, err := fila.Agenda(ctx, s.pool, KindCorrecao, map[string]any{
		"submissao_id": subID,
	}, 5); err != nil {
		slog.Error("submissoes: enfileirando correção", "err", err)
	}
	return &Entrega{Status: "entregue", NT: nT, Atrasada: true, SegundosAtraso: 0}, nil
}

