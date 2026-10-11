// Package preavaliacao — pré-avaliação (spec §5.4): roda TODOS os testes
// públicos de TODAS as tarefas sobre as respostas correntes, comparação
// exata via expected_output do Judge0 (status 3 = accepted), grava
// pre_avaliacao com revisao_ref e expira em SENO_PRE_AVA_TTL_S.
//
// Nota de extração à instância real (PROBES.md): o batch com wait=true do
// Judge0 1.13 NÃO devolve resultados inline — a pré-avaliação roda
// sequencialmente por POST /submissions?wait=true (cada item <5 s com os
// limites default; teto do relatório é SENO_PRE_AVA_TTL_S = 15 min).
// Não entra no histórico de ensaios (nenhuma linha em execucao).
package preavaliacao

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/judge0"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

type Service struct {
	pool   *pgxpool.Pool
	j0     *judge0.Client
	janela *tentativas.Service
	ttl    time.Duration // SENO_PRE_AVA_TTL_S
}

func New(pool *pgxpool.Pool, j0 *judge0.Client, janela *tentativas.Service, ttl time.Duration) *Service {
	return &Service{pool: pool, j0: j0, janela: janela, ttl: ttl}
}

// Erros mapeados (403 要求 no spec: sem tentativa/janela expirada).
var (
	ErrSemTentativa = errors.New("sem tentativa aberta")
)

type TestePre struct {
	Ordem        int     `json:"ordem"`
	Status       string  `json:"status"`
	Tempo        float64 `json:"tempo"`
	Memoria      int     `json:"memoria"`
	StdoutObtido *string `json:"stdout_obtido,omitempty"`
}

type TarefaPre struct {
	Ordem    int        `json:"ordem"`
	Veredito string     `json:"veredito"`
	Testes   []TestePre `json:"testes"`
}

type Relatorio struct {
	PreID  string `json:"pre_id"`
	Resumo struct {
		Acertos int `json:"acertos"`
		Total   int `json:"total"`
	} `json:"resumo"`
	Tarefas []TarefaPre `json:"tarefas"`
}

// Avaliar roda a pré-avaliação sincronizada e devolve o relatório.
func (s *Service) Avaliar(ctx context.Context, mid string, aluno int64, agora time.Time) (*Relatorio, error) {
	if _, err := s.janela.VerificarJanela(ctx, mid, aluno, agora); err != nil {
		return nil, err
	}

	// Respostas correntes + revisão (para revisao_ref do 409 do confirmar).
	var revisao int
	var dados []byte
	if err := s.pool.QueryRow(ctx, `SELECT revisao, dados FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno,
	).Scan(&revisao, &dados); err != nil {
		return nil, ErrSemTentativa
	}
	respostas := map[string]any{}
	_ = json.Unmarshal(dados, &respostas)

	var cargaJSON []byte
	if err := s.pool.QueryRow(ctx, `SELECT carga FROM atividade WHERE monitor_id = $1`, mid).Scan(&cargaJSON); err != nil {
		return nil, fmt.Errorf("preavaliacao: carga: %w", err)
	}
	var t struct {
		Carga contract.Carga `json:"carga"`
	}
	if err := json.Unmarshal(cargaJSON, &t); err != nil {
		return nil, fmt.Errorf("preavaliacao: carga ilegível: %w", err)
	}

	preID, err := novoPreID()
	if err != nil {
		return nil, fmt.Errorf("preavaliacao: id: %w", err)
	}

	rel := &Relatorio{PreID: preID}
	for _, tarefa := range t.Carga.Tarefas {
		codigo := codigoPara(respostas, tarefa.Ordem)
		if strings.TrimSpace(codigo) == "" {
			// resposta vazia: Judge0 rejeita source_code blank (422); relata
			// "wrong_answer" sem submissão (caminho real do aluno: não avaliar
			// código que não existe).
			rel.Tarefas = append(rel.Tarefas, TarefaPre{
				Ordem: tarefa.Ordem, Veredito: "wrong_answer",
			})
			continue
		}
		if strings.TrimSpace(codigo) == "" {
			// resposta vazia: teste roda sinalizando "sem resposta" (status
			// "wrong_answer" com stdout null) — o Judge0 rejeita source blank
			// (422), então não há submissão real a fazer.
			rel.Tarefas = append(rel.Tarefas, TarefaPre{
				Ordem: tarefa.Ordem, Veredito: "wrong_answer",
				Testes: nil, // nenhum teste público entregue
			})
			continue
		}
		lim := tarefa.Limites
		if lim == nil {
			lim = &contract.Limites{TempoCPUMs: 5000, TempoTotalMs: 10000, MemoriaMB: 128}
		}
		idLinguagem, ok := judge0.LinguagemDo(tarefa.Linguagem)
		if !ok {
			return nil, fmt.Errorf("preavaliacao: linguagem %s", tarefa.Linguagem)
		}

		tp := TarefaPre{Ordem: tarefa.Ordem, Veredito: "accepted"}
		acertosTarefa := 0
		totalTarefa := 0
		for i, te := range tarefa.Testes {
			if !te.Publico {
				continue
			}
			totalTarefa++
			r, err := s.j0.EnviarSync(ctx, judge0.Submissao{
				LanguageID:     idLinguagem,
				SourceCode:     codigo,
				Stdin:          te.Stdin,
				ExpectedOutput: te.StdoutEsperado,
				CPUTimeSec:     float64(lim.TempoCPUMs) / 1000,
				WallTimeSec:    float64(lim.TempoTotalMs) / 1000,
				MemoriaKB:      lim.MemoriaMB * 1024,
			})
			if err != nil {
				return nil, fmt.Errorf("preavaliacao: judge0 (tarefa %d, teste %d): %w",
					tarefa.Ordem, i+1, err)
			}
			stdout := deB64(r.Stdout)
			item := TestePre{Ordem: i + 1, Status: "judge0:" + fmt.Sprint(r.StatusID()), Tempo: tempo(r.Time)}
			if r.StatusID() == judge0.StatusAcceptedID {
				item.Status = "accepted"
				acertosTarefa++
			} else {
				tp.Veredito = item.Status
				if item.Status != "accepted" {
					item.StdoutObtido = &stdout
				}
			}
			item.Memoria = memoria(r.Memory)
			tp.Testes = append(tp.Testes, item)
			rel.Resumo.Total++
		}
		if acertosTarefa == totalTarefa {
			rel.Resumo.Acertos += acertosTarefa
		}
		rel.Tarefas = append(rel.Tarefas, tp)
	}

	relatorio, err := json.Marshal(rel)
	if err != nil {
		return nil, fmt.Errorf("preavaliacao: relatório: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO pre_avaliacao
	    (id, monitor_id, aluno_api_id, revisao_ref, relatorio, criada_em, expira_em)
	    VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`,
		preID, mid, aluno, revisao, string(relatorio), agora, agora.Add(s.ttl)); err != nil {
		return nil, fmt.Errorf("preavaliacao: gravando: %w", err)
	}
	return rel, nil
}

// Dias usados no entregar? a confirmar na Fase 6: o confirmar valida pre_id não expirado.

func codigoPara(respostas map[string]any, ordem int) string {
	if v, ok := respostas[fmt.Sprint(ordem)]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func tempo(t judge0.NumStr) float64 {
	var f float64
	if _, err := fmt.Sscan(string(t), &f); err == nil {
		return f
	}
	return 0
}

func memoria(m judge0.NumStr) int {
	var n int
	if _, err := fmt.Sscan(string(m), &n); err == nil {
		return n
	}
	return 0
}

func deB64(s string) string {
	if s == "" {
		return ""
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b)
	}
	return s
}

func novoPreID() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pv_" + hex.EncodeToString(b), nil
}
