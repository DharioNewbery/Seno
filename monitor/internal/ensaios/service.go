// Package ensaios — Execução de teste do aluno no Monitor (spec §5.3):
// POST /testar cria um job assíncrono (fila PostgreSQL, prioridade alta —
// hora da prova!), o worker envia os testes públicos ao Judge0 com callback,
// o callback (PUT ?key=...&run=...&t=...) registra os resultados nas linhas
// de `execucao` criadas como placeholders, e o GET do run devolve o estado
// do ensaio quando pronto. Limites: 5/min por aluno (somadas, spec §5.3);
// retenção de SENO_EXECUCAO_RETENCAO_D com limpeza a cada inserção.
// O payload do job carrega somente testes públicos (privados não saem),
// mas o código do aluno sim (é o que o Judge0 executa).
package ensaios

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/fila"
	"github.com/seno-project/seno/monitor/internal/judge0"
	"github.com/seno-project/seno/monitor/internal/tentativas"
)

const (
	// KindEnsaio é o kind do job de ensaio na fila.
	KindEnsaio    = "ensaio"
	prioridadeTop = 10 // acima da correção (ARQUITETURA §6: hora da prova)
)

// Service coordena ensaios.
type Service struct {
	pool         *pgxpool.Pool
	j0           *judge0.Client
	janela       *tentativas.Service
	callbackBase string        // o Monitor visto pelo Judge0 (ex.: http://host.docker.internal:8081)
	callbackKey  string        // segredo validado no callback (?key=)
	maxPorMin    int           // SENO_ALUNO_EXEC_MAX_PER_MIN
	retencao     time.Duration // SENO_EXECUCAO_RETENCAO_D
}

func New(pool *pgxpool.Pool, j0 *judge0.Client, janela *tentativas.Service,
	callbackBase, callbackKey string, maxPorMin int, execucaoRetencao time.Duration) *Service {
	return &Service{pool: pool, j0: j0, janela: janela,
		callbackBase: callbackBase, callbackKey: callbackKey,
		maxPorMin: maxPorMin, retencao: execucaoRetencao,
	}
}

// Erros que o handler mapeia.
var (
	ErrLimite     = errors.New("limite de execuções por minuto")
	ErrSemTarefa  = errors.New("tarefa não encontrada")
	ErrSemPublico = errors.New("tarefa sem testes públicos")
)

type payloadEnsaio struct {
	Run       string           `json:"run"`
	MonitorID string           `json:"monitor_id"`
	Aluno     int64            `json:"aluno_api_id"`
	Ordem     int              `json:"ordem"`
	Linguagem string           `json:"linguagem"`
	Codigo    string           `json:"codigo"`
	Limites   contract.Limites `json:"limites"`
	Testes    []testePayload   `json:"testes"`
}

type testePayload struct {
	Ordem          int    `json:"ordem"`
	Stdin          string `json:"stdin"`
	StdoutEsperado string `json:"stdout_esperado"`
}

// Agendar valida janela/tarefa/rate-limit e agenda o job. run_id devolvido
// alimenta o polling GET /testar/{run}.
func (s *Service) Agendar(ctx context.Context, mid string, aluno int64,
	ordem int, linguagem, codigo string, agora time.Time) (run string, err error) {
	if _, err := s.janela.VerificarJanela(ctx, mid, aluno, agora); err != nil {
		return "", err
	}

	// spec §5.3: ensaio exige tentativa aberta (a matrícula/janela seguem).
	var existe bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2)`, mid, aluno).Scan(&existe); err != nil {
		return "", fmt.Errorf("ensaios: tentativa: %w", err)
	}
	if !existe {
		return "", tentativas.ErrSemTentativa
	}

	// Limpeza da retenção em toda inserção (spec §5.3).
	_, _ = s.pool.Exec(ctx, `DELETE FROM execucao
	    WHERE criada_em < now() - $1::interval`, retInterval(s.retencao))

	// Rate limit: ensaios do aluno (todas as tarefas somadas) na última min.
	var ultimas int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM execucao
	    WHERE tipo = 'ensaio' AND aluno_api_id = $1 AND monitor_id = $2
	      AND criada_em > now() - interval '1 minute'`,
		aluno, mid).Scan(&ultimas); err != nil {
		return "", fmt.Errorf("ensaios: rate limit: %w", err)
	}
	if ultimas >= s.maxPorMin {
		return "", ErrLimite
	}

	carga, err := cargaDaAtividade(ctx, s.pool, mid)
	if err != nil {
		return "", err
	}
	var tarefa *contract.TarefaCarga
	for i := range carga.Tarefas {
		if carga.Tarefas[i].Ordem == ordem {
			tarefa = &carga.Tarefas[i]
			break
		}
	}
	if tarefa == nil {
		return "", ErrSemTarefa
	}
	if _, ok := judge0.LinguagemDo(linguagem); !ok || linguagem != tarefa.Linguagem {
		return "", fmt.Errorf("ensaios: linguagem %q differs da tarefa (%q)", linguagem, tarefa.Linguagem)
	}
	var testes []testePayload
	for i, te := range tarefa.Testes {
		if te.Publico {
			testes = append(testes, testePayload{Ordem: i + 1, Stdin: te.Stdin, StdoutEsperado: te.StdoutEsperado})
		}
	}
	if len(testes) == 0 {
		return "", ErrSemPublico
	}
	limits := contract.Limites{TempoCPUMs: 5000, TempoTotalMs: 10000, MemoriaMB: 128}
	if tarefa.Limites != nil {
		limits = *tarefa.Limites
	}

	// run = random hex 16; execucao placeholders criadas ANTES
	// (callback só faz UPDATE: sem INSERT arbitrário via fio).
	run, err = runID()
	if err != nil {
		return "", fmt.Errorf("ensaios: run: %w", err)
	}
	for _, te := range testes {
		if _, err := s.pool.Exec(ctx, `INSERT INTO execucao
		    (run, monitor_id, aluno_api_id, tipo, tarefa_ordem, teste_ordem, publico, criada_em)
		    VALUES ($1, $2, $3, 'ensaio', $4, $5, true, $6)`,
			run, mid, aluno, ordem, te.Ordem, agora); err != nil {
			return "", fmt.Errorf("ensaios: placeholder: %w", err)
		}
	}

	if _, err := fila.Agenda(ctx, s.pool, KindEnsaio, payloadEnsaio{
		Run: run, MonitorID: mid, Aluno: aluno, Ordem: ordem,
		Linguagem: linguagem, Codigo: codigo,
		Limites: limits, Testes: testes,
	}, prioridadeTop); err != nil {
		return "", fmt.Errorf("ensaios: agendando: %w", err)
	}
	return run, nil
}

// Handler é o handler da fila para o kind "ensaio": envia os testes
// públicos ao Judge0 (async + callback por item). Erro → retentativa da fila.
func (s *Service) Handler() fila.Handler {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p payloadEnsaio
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("ensaio: payload: %w", err)
		}
		idLinguagem, ok := judge0.LinguagemDo(p.Linguagem)
		if !ok {
			return fmt.Errorf("ensaio: linguagem %s", p.Linguagem)
		}
		lim := p.Limites
		if lim.TempoCPUMs == 0 {
			lim.TempoCPUMs = 5000 // padrão global vindo da carga (§2)
		}
		if lim.TempoTotalMs == 0 {
			lim.TempoTotalMs = 10000
		}
		if lim.MemoriaMB == 0 {
			lim.MemoriaMB = 128
		}

		for _, te := range p.Testes {
			cb := fmt.Sprintf("%s/interna/callbacks/judge0?key=%s&run=%s&t=%d",
				s.callbackBase, s.callbackKey, p.Run, te.Ordem)
			token, err := s.j0.EnviarAsync(ctx, judge0.Submissao{
				LanguageID:     idLinguagem,
				SourceCode:     p.Codigo,
				Stdin:          te.Stdin,
				ExpectedOutput: te.StdoutEsperado,
				CallbackURL:    cb,
				CPUTimeSec:     float64(lim.TempoCPUMs) / 1000,
				WallTimeSec:    float64(lim.TempoTotalMs) / 1000,
				MemoriaKB:      lim.MemoriaMB * 1024,
			})
			if err != nil {
				return fmt.Errorf("ensaio: judge0 (teste %d): %w", te.Ordem, err)
			}
			// Fallback: o Judge0 entrega callback silenciosamente descartável
			// (Windows internet firewall em dev; rede em prod). Com o token, o
			// GET do run pode buscar o resultado direto do Judge0 (Consultar).
			_, _ = s.pool.Exec(ctx, `UPDATE execucao SET token = $3
			    WHERE run = $1 AND teste_ordem = $2`, p.Run, te.Ordem, token)
		}
		return nil
	}
}

// Callback registra o resultado de um teste (PUT do Judge0; handler valida
// ?key= e faz UPDATE no placeholder). Late/duplicado com run inexistente é
// ignorado silenciosamente (cache do judge0 pode re-entregar).
func (s *Service) Callback(ctx context.Context, run string, testeOrdem int,
	r judge0.Resposta) error {
	return s.registra(ctx, run, testeOrdem, r)
}

// lerEstado monta o EnsaioEstado a partir das linhas atuais do run.
func (s *Service) lerEstado(ctx context.Context, mid string, aluno int64, run string) (*EnsaioEstado, error) {
	rows, err := s.pool.Query(ctx, `SELECT teste_ordem, veredito, stdout_obtido,
	    COALESCE(tempo_seg, 0), COALESCE(memoria_kb, 0)
	    FROM execucao WHERE run = $1 AND monitor_id = $2 AND aluno_api_id = $3
	    ORDER BY teste_ordem`, run, mid, aluno)
	if err != nil {
		return nil, fmt.Errorf("ensaios: consultar: %w", err)
	}
	defer rows.Close()
	var e EnsaioEstado
	for rows.Next() {
		var r ResultadoEnsaio
		var veredito, stdout *string
		if err := rows.Scan(&r.TesteOrdem, &veredito, &stdout, &r.TempoSeg, &r.MemoriaKB); err != nil {
			return nil, fmt.Errorf("ensaios: consultar: %w", err)
		}
		if veredito == nil {
			e.Pendentes++
			continue
		}
		r.Veredito = *veredito
		r.StdoutObtido = stdout
		e.Resultados = append(e.Resultados, r)
	}
	e.Pronto = e.Pendentes == 0
	return &e, rows.Err()
}

// registra aplica o veredito na linha do teste (callback OU polling).
func (s *Service) registra(ctx context.Context, run string, testeOrdem int, r judge0.Resposta) error {
	// Estados de fila (1 = In Queue, 2 = Processing) não são veredito: deixa
	// a linha pendente para o próximo callback/polling.
	if r.StatusID() == 1 || r.StatusID() == 2 {
		return nil
	}
	stdout := decodificaB64(r.Stdout)
	if r.CompileOutput != "" {
		stdout = "" // corpo de compilação não é stdout do programa
	}
	var t float64
	if tS := string(r.Time); tS != "" {
		if v, err := strconv.ParseFloat(tS, 64); err == nil {
			t = v
		}
	}
	var mem int
	if mS := string(r.Memory); mS != "" {
		if v, err := strconv.Atoi(mS); err == nil {
			mem = v
		}
	}
	veredito := "judge0:" + strconv.Itoa(r.StatusID())
	if r.StatusID() == judge0.StatusAcceptedID {
		veredito = "accepted"
	}
	ct, err := s.pool.Exec(ctx, `UPDATE execucao SET veredito = $3,
	    stdout_obtido = $4, tempo_seg = $5, memoria_kb = $6
	    WHERE run = $1 AND teste_ordem = $2`,
		run, testeOrdem, veredito, stdout, t, mem)
	if err != nil {
		return fmt.Errorf("ensaios: callback: %w", err)
	}
	// disparo perdido: run inexistente (não bloqueia, só registra)
	_ = ct
	return nil
}

// consultar é a resposta do GET do ensaio.
type EnsaioEstado struct {
	Pronto     bool              `json:"pronto"`
	Resultados []ResultadoEnsaio `json:"resultados"`
	Pendentes  int               `json:"pendentes"`
}

type ResultadoEnsaio struct {
	TesteOrdem   int     `json:"ordem"`
	Veredito     string  `json:"veredito"`
	StdoutObtido *string `json:"stdout_obtido,omitempty"`
	TempoSeg     float64 `json:"tempo"`
	MemoriaKB    int     `json:"memoria"`
}

// Consultar devolve o estado do run (ensaios até §5.3; rota GET).
// Para testes ainda pendentes com token conhecido, consulta o Judge0
// diretamente (fallback quando o callback não chegou).
func (s *Service) Consultar(ctx context.Context, mid string, aluno int64, run string) (*EnsaioEstado, error) {
	rows, err := s.pool.Query(ctx, `SELECT teste_ordem, veredito, stdout_obtido,
	    COALESCE(tempo_seg, 0), COALESCE(memoria_kb, 0), token
	    FROM execucao WHERE run = $1 AND monitor_id = $2 AND aluno_api_id = $3
	    ORDER BY teste_ordem`, run, mid, aluno)
	if err != nil {
		return nil, fmt.Errorf("ensaios: consultar: %w", err)
	}
	defer rows.Close()
	var e EnsaioEstado
	var pendentes []struct {
		ordem int
		token string
	}
	for rows.Next() {
		var r ResultadoEnsaio
		var veredito *string
		var stdout *string
		var token *string
		if err := rows.Scan(&r.TesteOrdem, &veredito, &stdout, &r.TempoSeg, &r.MemoriaKB, &token); err != nil {
			return nil, fmt.Errorf("ensaios: consultar: %w", err)
		}
		if veredito == nil {
			if token != nil && *token != "" && s.j0 != nil {
				pendentes = append(pendentes, struct {
					ordem int
					token string
				}{r.TesteOrdem, *token})
			}
			e.Pendentes++
			continue
		}
		r.Veredito = *veredito
		r.StdoutObtido = stdout
		e.Resultados = append(e.Resultados, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ensaios: consultar: %w", err)
	}
	if len(e.Resultados) == 0 && e.Pendentes == 0 && len(pendentes) == 0 {
		return nil, errors.New("run inexistente") // handler: 404
	}
	// Fallback ativo: pedido direto ao Judge0 pelo token (callback perdido).
	if len(pendentes) > 0 {
		for _, p := range pendentes {
			obterCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			r, err := s.j0.Obter(obterCtx, p.token)
			cancel()
			if err != nil {
				continue // ainda na fila ou indisponível: segue pendente
			}
			_ = s.registra(ctx, run, p.ordem, r)
		}
		// re-estado após o fallback (passam a pronto nesta mesma resposta).
		return s.lerEstado(ctx, mid, aluno, run)
	}
	e.Pronto = e.Pendentes == 0
	return &e, nil
}

// -- Helpers ------------------------------------------------------------------

func runID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func cargaDaAtividade(ctx context.Context, pool *pgxpool.Pool, mid string) (*contract.Carga, error) {
	var app []byte
	if err := pool.QueryRow(ctx, `SELECT carga FROM atividade WHERE monitor_id = $1`, mid).Scan(&app); err != nil {
		return nil, ErrSemTarefa
	}
	var t struct {
		Carga contract.Carga `json:"carga"`
	}
	if err := json.Unmarshal(app, &t); err != nil {
		return nil, fmt.Errorf("ensaios: carga ilegível: %w", err)
	}
	return &t.Carga, nil
}

func retInterval(d time.Duration) string { return fmt.Sprintf("%fs", d.Seconds()) }

// decodificaB64 decodifica o stdout do callback (base64 na entrega, PROBES.md).
func decodificaB64(s string) string {
	if s == "" {
		return ""
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b)
	}
	return s // já cru
}
