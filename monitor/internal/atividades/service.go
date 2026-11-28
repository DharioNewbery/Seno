package atividades

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
)

// Service recebe e persiste atividades transferidas da API (spec §2).
// Acesso a dados com pgx direto; sqlc entra quando a CLI existir no ambiente
// (Fase 2, dev-a-do-plano).
type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Receber grava a atividade e as matrículas do snapshot. Idempotente por
// api_id: reenvio devolve o monitor_id existente e sinaliza existente=true.
func (s *Service) Receber(ctx context.Context, t *contract.Transferencia) (monitorID string, existente bool, err error) {
	carga, err := json.Marshal(t)
	if err != nil {
		return "", false, fmt.Errorf("serializando carga: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", false, fmt.Errorf("transação: %w", err)
	}
	defer tx.Rollback(ctx)

	// Idempotência: SELECT..FOR UPDATE dentro da transação para fechar a
	// janela entre ver e inserir com cadastro concorrente (evita que o item
	// seja criado 2x por requestes simultâneos com o mesmo api_id).
	var existing string
	if err := tx.QueryRow(ctx,
		`SELECT monitor_id FROM atividade WHERE api_id = $1 FOR UPDATE`, t.APIID,
	).Scan(&existing); err == nil {
		if _, err := tx.Exec(ctx,
			`UPDATE atividade SET carga = $2::jsonb WHERE monitor_id = $1`,
			existing, string(carga)); err != nil {
			return "", false, fmt.Errorf("atualizando atividade: %w", err)
		}
		// Sincroniza matrículas: insere novos alunos ativos e remove alunos
		// desmatriculados que ainda não possuam submissão registrada.
		idsAtivos := make([]int64, 0, len(t.Alunos))
		for _, a := range t.Alunos {
			idsAtivos = append(idsAtivos, a.APIID)
			if _, err := tx.Exec(ctx, `INSERT INTO atividade_aluno
			    (monitor_id, aluno_api_id) VALUES ($1, $2)
			    ON CONFLICT (monitor_id, aluno_api_id) DO NOTHING`, existing, a.APIID); err != nil {
				return "", false, fmt.Errorf("sincronizando aluno %d: %w", a.APIID, err)
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM atividade_aluno
		    WHERE monitor_id = $1
		      AND NOT (aluno_api_id = ANY($2::bigint[]))
		      AND NOT EXISTS (
		          SELECT 1 FROM submissao s
		          WHERE s.monitor_id = $1 AND s.aluno_api_id = atividade_aluno.aluno_api_id
		      )`, existing, idsAtivos); err != nil {
			return "", false, fmt.Errorf("removendo alunos inativos: %w", err)
		}
		fimReal := fimRealJanela(t.Janela)
		if _, err := tx.Exec(ctx,
			`UPDATE tentativa SET fim_real = $2 WHERE monitor_id = $1`,
			existing, fimReal); err != nil {
			return "", false, fmt.Errorf("atualizando fim_real de tentativas: %w", err)
		}
		return existing, true, tx.Commit(ctx)
	}

	id, err := gerarMonitorID()
	if err != nil {
		return "", false, fmt.Errorf("gerando monitor_id: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO atividade
	    (monitor_id, api_id, carga, recebida_em, status)
	    VALUES ($1, $2, $3::jsonb, now(), 'ativa')`, id, t.APIID, string(carga)); err != nil {
		return "", false, fmt.Errorf("inserindo atividade: %w", err)
	}

	for _, a := range t.Alunos {
		if _, err := tx.Exec(ctx, `INSERT INTO atividade_aluno
		    (monitor_id, aluno_api_id) VALUES ($1, $2)`, id, a.APIID); err != nil {
			return "", false, fmt.Errorf("inserindo aluno %d: %w", a.APIID, err)
		}
	}

	return id, false, tx.Commit(ctx)
}

// fimRealJanela calcula fim_real = min(inicio + duração, prazo).
func fimRealJanela(j contract.Janela) time.Time {
	fim := j.Prazo
	if j.DuracaoSeg != nil {
		if d := j.Inicio.Add(time.Duration(*j.DuracaoSeg) * time.Second); d.Before(fim) {
			fim = d
		}
	}
	return fim
}

// gerarMonitorID devolve "atv_" + 12 hex aleatórios (CSPRNG).
func gerarMonitorID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return "atv_" + hex.EncodeToString(b), nil
}
