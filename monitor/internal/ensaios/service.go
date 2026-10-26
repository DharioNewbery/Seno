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

	"github.com/jackc/pgx/v5"
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
	// (callback só faz UPDATE: sem INSERT arbitrário via fio). codigo vai
	// em toda linha do run: é o que o histórico (GET /testar) mostra depois.
	run, err = runID()
	if err != nil {
		return "", fmt.Errorf("ensaios: run: %w", err)
	}
	for _, te := range testes {
		if _, err := s.pool.Exec(ctx, `INSERT INTO execucao
		    (run, monitor_id, aluno_api_id, tipo, tarefa_ordem, teste_ordem, publico, codigo, criada_em)
		    VALUES ($1, $2, $3, 'ensaio', $4, $5, true, $6, $7)`,
			run, mid, aluno, ordem, te.Ordem, codigo, agora); err != nil {
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
	    COALESCE(tempo_seg, 0), COALESCE(memoria_kb, 0), stderr, compilacao
	    FROM execucao WHERE run = $1 AND monitor_id = $2 AND aluno_api_id = $3
	    ORDER BY teste_ordem`, run, mid, aluno)
	if err != nil {
		return nil, fmt.Errorf("ensaios: consultar: %w", err)
	}
	defer rows.Close()
	var e EnsaioEstado
	for rows.Next() {
		r, err := scanResultado(rows)
		if err != nil {
			return nil, fmt.Errorf("ensaios: consultar: %w", err)
		}
		if r.Veredito == "" {
			e.Pendentes++
			continue
		}
		e.Resultados = append(e.Resultados, r)
	}
	e.Pronto = e.Pendentes == 0
	return &e, rows.Err()
}

// scanResultado varre uma linha de execucao em ResultadoEnsaio (veredito
// "" significa "ainda pendente": urbanização comum às duas consultas).
func scanResultado(row pgx.Row) (ResultadoEnsaio, error) {
	var r ResultadoEnsaio
	var veredito, stdout, stderr, compilacao *string
	if err := row.Scan(&r.TesteOrdem, &veredito, &stdout, &r.TempoSeg,
		&r.MemoriaKB, &stderr, &compilacao); err != nil {
		return r, err
	}
	if veredito != nil {
		r.Veredito = *veredito
	}
	r.StdoutObtido = stdout
	r.Stderr = stderr
	r.Compilacao = compilacao
	return r, nil
}

// registra aplica o veredito na linha do teste (callback OU polling).
func (s *Service) registra(ctx context.Context, run string, testeOrdem int, r judge0.Resposta) error {
	// Estados de fila (1 = In Queue, 2 = Processing) não são veredito: deixa
	// a linha pendente para o próximo callback/polling.
	if r.StatusID() == 1 || r.StatusID() == 2 {
		return nil
	}
	stdout := decodificaB64(r.Stdout)
	// stderr e saída do compilador alimentam as mensagens de erro legíveis
	// da tela (o aluno vê por que o teste falhou, não só o código do judge).
	stderr := decodificaB64(r.Stderr)
	compilacao := decodificaB64(r.CompileOutput)
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
	    stdout_obtido = $4, tempo_seg = $5, memoria_kb = $6,
	    stderr = $7, compilacao = $8
	    WHERE run = $1 AND teste_ordem = $2`,
		run, testeOrdem, veredito, stdout, t, mem, stderr, compilacao)
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
	// Detalhes para a mensagem de erro (vazios em accepted).
	Stderr     *string `json:"stderr,omitempty"`
	Compilacao *string `json:"compilacao,omitempty"`
	TempoSeg   float64 `json:"tempo"`
	MemoriaKB  int     `json:"memoria"`
}

// RunHistorico é um ensaio anterior: código rodado + vereditos dos testes.
type RunHistorico struct {
	RunID      string            `json:"run_id"`
	Ordem      int               `json:"ordem"`
	CriadaEm   time.Time         `json:"criada_em"`
	Codigo     string            `json:"codigo"`
	Resultados []ResultadoEnsaio `json:"resultados"`
}

// Consultar devolve o estado do run (ensaios até §5.3; rota GET).
// Para testes ainda pendentes com token conhecido, consulta o Judge0
// diretamente (fallback quando o callback não chegou).
func (s *Service) Consultar(ctx context.Context, mid string, aluno int64, run string) (*EnsaioEstado, error) {
	rows, err := s.pool.Query(ctx, `SELECT teste_ordem, veredito, stdout_obtido,
	    COALESCE(tempo_seg, 0), COALESCE(memoria_kb, 0), stderr, compilacao, token
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
		var token *string
		if err := rows.Scan(&r.TesteOrdem, &veredito, &r.StdoutObtido,
			&r.TempoSeg, &r.MemoriaKB, &r.Stderr, &r.Compilacao, &token); err != nil {
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

// Historico devolve os últimos runs de ensaio do aluno nesta atividade
// (teto de 20, já limitados pela retenção de execucao): vereditos e o
// código antigo que rodou — alimenta o botão "Ver ensaios anteriores".
func (s *Service) Historico(ctx context.Context, mid string, aluno int64) ([]RunHistorico, error) {
	rows, err := s.pool.Query(ctx, `SELECT run, tarefa_ordem, criada_em,
	    COALESCE(codigo, ''), COALESCE(teste_ordem, 0), veredito,
	    COALESCE(tempo_seg, 0), COALESCE(memoria_kb, 0), stdout_obtido,
	    stderr, compilacao
	    FROM execucao
	    WHERE monitor_id = $1 AND aluno_api_id = $2 AND tipo = 'ensaio'
	      AND run IN (SELECT run FROM execucao
	                  WHERE monitor_id = $1 AND aluno_api_id = $2 AND tipo = 'ensaio'
	                  GROUP BY run ORDER BY max(criada_em) DESC LIMIT 20)
	    ORDER BY criada_em DESC, teste_ordem`, mid, aluno)
	if err != nil {
		return nil, fmt.Errorf("ensaios: histórico: %w", err)
	}
	defer rows.Close()
	var out []RunHistorico
	indice := map[string]int{}
	for rows.Next() {
		var run string
		var ordem int
		var criada time.Time
		var codigo string
		var testeOrdem int
		var veredito, stdout, stderr, compilacao *string
		var tempo float64
		var mem int
		if err := rows.Scan(&run, &ordem, &criada, &codigo, &testeOrdem,
			&veredito, &tempo, &mem, &stdout, &stderr, &compilacao); err != nil {
			return nil, fmt.Errorf("ensaios: histórico: %w", err)
		}
		i, ok := indice[run]
		if !ok {
			i = len(out)
			indice[run] = i
			out = append(out, RunHistorico{RunID: run, Ordem: ordem,
				CriadaEm: criada, Codigo: codigo})
		}
		if veredito == nil {
			continue // teste pendente: fora da lista de resultados
		}
		out[i].Resultados = append(out[i].Resultados, ResultadoEnsaio{
			TesteOrdem: testeOrdem, Veredito: *veredito, StdoutObtido: stdout,
			Stderr: stderr, Compilacao: compilacao, TempoSeg: tempo, MemoriaKB: mem,
		})
	}
	return out, rows.Err()
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
