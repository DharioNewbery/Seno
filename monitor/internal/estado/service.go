// Package estado — consolida o quadro de uma atividade para a API principal
// (spec §6, Fase 7 do plano): leitura pura, snapshot integral por chamada.
// A API chama por job periódico (HMAC) e PERSISTE o snapshot local — o
// Monitor não mantém nada além do fluxo de realização; ao Monitor offline
// a listagem do aluno continua da própria API.
package estado

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
)

var ErrNaoEncontrado = errors.New("atividade não encontrada")

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Estado devolve o snapshot consolidado da atividade (§6).
func (s *Service) Estado(ctx context.Context, mid string) (*contract.EstadoAtividade, error) {
	var status string
	var carga []byte
	err := s.pool.QueryRow(ctx, `SELECT status, carga FROM atividade WHERE monitor_id = $1`,
		mid).Scan(&status, &carga)
	if err != nil {
		return nil, ErrNaoEncontrado
	}

	out := &contract.EstadoAtividade{MonitorID: mid, Status: status}

	// Sem corrigir? Tudo via um único SELECT com LATERAL para o estado do
	// aluno: tentativa (aberta), última submissão (entregue/atrasada) e a
	// correção da última submissão (nota/status) — as 4 fontes da §6.
	rows, err := s.pool.Query(ctx, `SELECT
	    aa.aluno_api_id,
	    (t.monitor_id IS NOT NULL),
	    t.fim_real,
	    sb.n_t,
	    sb.entregue_em,
	    COALESCE(sb.atrasada, false),
	    c.status,
	    c.nota_auto
	  FROM atividade_aluno aa
	  LEFT JOIN tentativa t
	    ON t.monitor_id = aa.monitor_id AND t.aluno_api_id = aa.aluno_api_id
	  LEFT JOIN LATERAL (
	    SELECT id, n_t, entregue_em, atrasada FROM submissao s
	    WHERE s.monitor_id = aa.monitor_id AND s.aluno_api_id = aa.aluno_api_id
	    ORDER BY n_t DESC LIMIT 1
	  ) sb ON true
	  LEFT JOIN correcao c ON c.submissao_id = sb.id
	  WHERE aa.monitor_id = $1
	  ORDER BY aa.aluno_api_id`, mid)
	if err != nil {
		return nil, fmt.Errorf("estado: alunos: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var alunoAPI int64
		var temTentativa bool
		var fimReal *time.Time
		var nT *int
		var entregueEm *time.Time
		var atrasada bool
		var corrStatus *string
		var notaAuto *float64
		if err := rows.Scan(&alunoAPI, &temTentativa, &fimReal, &nT,
			&entregueEm, &atrasada, &corrStatus, &notaAuto); err != nil {
			return nil, fmt.Errorf("estado: scan: %w", err)
		}
		e := contract.EstadoAluno{
			AlunoAPIID: alunoAPI, CorrecaoStatus: "pendente", Estado: "pendente",
		}
		switch {
		case nT != nil:
			e.Estado = "entregue"
			e.NT = *nT
			e.UltimaSubmissao = entregueEm
		case temTentativa:
			e.Estado = "aberta"
		}
		switch {
		case temTentativa && fimReal != nil:
			// prazo efetivo = fim_real (recalculado no abrir)
			e.PrazoEfetivo = fimReal
		case !temTentativa:
			if prazo, ok := prazoSemAtividade(carga); ok {
				e.PrazoEfetivo = &prazo
			}
		}
		if corrStatus != nil {
			e.CorrecaoStatus = *corrStatus
		}
		e.NotaAuto = notaAuto

		out.Alunos = append(out.Alunos, e)
		switch {
		case nT != nil && atrasada:
			out.Agregados.Atrasos++
			out.Agregados.Entregues++
		case nT != nil:
			out.Agregados.Entregues++
		case temTentativa:
			out.Agregados.Abertas++
		}
		out.Agregados.Total++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("estado: alunos: %w", err)
	}
	return out, rows.Err()
}

// prazoSemAtividade extrai o prazo efetivo (fim fixo §5.1) da carga para
// alunos que ainda não abriram (sem tentativa para ler fim_real).
func prazoSemAtividade(carga []byte) (time.Time, bool) {
	var t struct {
		Janela contract.Janela `json:"janela"`
	}
	if err := json.Unmarshal(carga, &t); err != nil {
		return time.Time{}, false
	}
	j := t.Janela
	if j.Prazo.IsZero() {
		return time.Time{}, false
	}
	if j.DuracaoSeg != nil {
		if d := j.Inicio.Add(time.Duration(*j.DuracaoSeg) * time.Second); d.Before(j.Prazo) {
			return d, true
		}
	}
	return j.Prazo, true
}
