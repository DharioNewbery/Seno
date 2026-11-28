// Package preavaliacao — pré-avaliação (spec §5.4): roda TODOS os testes
// públicos de TODAS as tarefas sobre as respostas correntes, comparação
// exata via expected_output do Judge0 (status 3 = accepted), grava
// pre_avaliacao com revisao_ref e expira em SENO_PRE_AVA_TTL_S.
//
// Resiliência: caso o Judge0 esteja desligado ou temporariamente fora do ar,
// a pré-avaliação NÃO bloqueia a entrega da prova — grava o relatório com
// veredito "avaliador_indisponivel" e aviso explícito ao aluno, permitindo
// confirmar e salvar a submissão com segurança para correção posterior na fila.
package preavaliacao

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// Erros mapeados (403 no spec: sem tentativa/janela expirada).
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
	Ordem       int        `json:"ordem"`
	Tipo        string     `json:"tipo,omitempty"`
	Veredito    string     `json:"veredito"`
	NomeArquivo string     `json:"nome_arquivo,omitempty"`
	Testes      []TestePre `json:"testes"`
}

type Relatorio struct {
	PreID  string `json:"pre_id"`
	Resumo struct {
		Acertos int `json:"acertos"`
		Total   int `json:"total"`
	} `json:"resumo"`
	Tarefas        []TarefaPre `json:"tarefas"`
	AvisoAvaliador string      `json:"aviso_avaliador,omitempty"`
}

const avisoJudge0Offline = "O avaliador automático está temporariamente indisponível. Você pode confirmar sua entrega normalmente — suas respostas serão salvas com segurança e corrigidas automaticamente assim que o avaliador voltar."

// Avaliar roda a pré-avaliação sincronizada com orçamento de tempo limitado e devolve o relatório.
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

	// Orçamento máximo total para chamadas ao Judge0 na pré-avaliação (evita travar a rota).
	j0BudgetCtx, cancelBudget := context.WithTimeout(ctx, 25*time.Second)
	defer cancelBudget()
	judge0Indisponivel := s.j0 == nil || !s.j0.Ligada()

	rel := &Relatorio{PreID: preID}
	for _, tarefa := range t.Carga.Tarefas {
		tipo := contract.NormalizarTipoTarefa(tarefa.Tipo)
		if tipo == contract.TipoTarefaArquivo {
			var nomeArq string
			errArq := s.pool.QueryRow(ctx, `SELECT nome_arquivo FROM tentativa_arquivo
			    WHERE monitor_id = $1 AND aluno_api_id = $2 AND tarefa_ordem = $3`,
				mid, aluno, tarefa.Ordem).Scan(&nomeArq)
			if errArq == nil && nomeArq != "" {
				rel.Tarefas = append(rel.Tarefas, TarefaPre{
					Ordem:       tarefa.Ordem,
					Tipo:        tipo,
					Veredito:    "arquivo_anexado",
					NomeArquivo: nomeArq,
				})
			} else {
				rel.Tarefas = append(rel.Tarefas, TarefaPre{
					Ordem:    tarefa.Ordem,
					Tipo:     tipo,
					Veredito: "sem_arquivo",
				})
			}
			continue
		}

		// Conta testes públicos da tarefa.
		temPublico := false
		for _, te := range tarefa.Testes {
			if te.Publico {
				temPublico = true
				break
			}
		}
		codigo := codigoPara(respostas, tarefa.Ordem)
		if !temPublico {
			veredito := "sem_testes_publicos"
			if strings.TrimSpace(codigo) == "" {
				veredito = "wrong_answer"
			}
			rel.Tarefas = append(rel.Tarefas, TarefaPre{
				Ordem: tarefa.Ordem, Veredito: veredito,
			})
			continue
		}
		if strings.TrimSpace(codigo) == "" {
			rel.Tarefas = append(rel.Tarefas, TarefaPre{
				Ordem: tarefa.Ordem, Veredito: "wrong_answer",
			})
			continue
		}
		if judge0Indisponivel {
			rel.AvisoAvaliador = avisoJudge0Offline
			rel.Tarefas = append(rel.Tarefas, TarefaPre{
				Ordem: tarefa.Ordem, Veredito: "avaliador_indisponivel",
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
		falhouJudge0 := false
		for i, te := range tarefa.Testes {
			if !te.Publico {
				continue
			}
			callCtx, cancelCall := context.WithTimeout(j0BudgetCtx, 10*time.Second)
			r, err := s.j0.EnviarSync(callCtx, judge0.Submissao{
				LanguageID:     idLinguagem,
				SourceCode:     codigo,
				Stdin:          te.Stdin,
				ExpectedOutput: te.StdoutEsperado,
				CPUTimeSec:     float64(lim.TempoCPUMs) / 1000,
				WallTimeSec:    float64(lim.TempoTotalMs) / 1000,
				MemoriaKB:      lim.MemoriaMB * 1024,
			})
			cancelCall()
			if err != nil {
				slog.Warn("preavaliacao: Judge0 indisponível; liberando entrega com aviso",
					"monitor_id", mid, "aluno", aluno, "tarefa", tarefa.Ordem, "err", err)
				judge0Indisponivel = true
				falhouJudge0 = true
				rel.AvisoAvaliador = avisoJudge0Offline
				break
			}
			totalTarefa++
			stdout := r.Stdout // base64_encoded=false no EnviarSync já devolve texto cru
			item := TestePre{Ordem: i + 1, Status: "judge0:" + fmt.Sprint(r.StatusID()), Tempo: tempo(r.Time)}
			if r.StatusID() == judge0.StatusAcceptedID {
				item.Status = "accepted"
				acertosTarefa++
			} else {
				tp.Veredito = item.Status
				item.StdoutObtido = &stdout
			}
			item.Memoria = memoria(r.Memory)
			tp.Testes = append(tp.Testes, item)
			rel.Resumo.Total++
		}
		if falhouJudge0 {
			tp.Veredito = "avaliador_indisponivel"
			rel.Tarefas = append(rel.Tarefas, tp)
			continue
		}
		if totalTarefa > 0 && acertosTarefa == totalTarefa {
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

func novoPreID() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pv_" + hex.EncodeToString(b), nil
}
