// Service do fluxo de realização do aluno (spec §5.1–5.2): abrir tentativa
// (idempotente, fim_real = min(inicio+duração, prazo)) e autosave com revisão
// otimista + rascunho_ponto imutável a cada intervalo.
//
// Regras comuns (spec §5): atividade precisa estar ativa, janelas recalculadas
// no servidor a cada operação, e o aluno só age na atividade da própria
// sessão (conferido no handler).
package tentativas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
)

type Service struct {
	pool      *pgxpool.Pool
	intervalo time.Duration // SENO_SNAPSHOT_INTERVALO_S (0 = snapshota sempre)
}

func New(pool *pgxpool.Pool, intervalo time.Duration) *Service {
	return &Service{pool: pool, intervalo: intervalo}
}

// Erros de negócio que os handlers mapeiam (errors.Is).
var (
	ErrEncerrada    = errors.New("janela encerrada")
	ErrFinalizada   = errors.New("cota de submissões atingida")
	ErrSemTentativa = errors.New("sem tentativa aberta")
	ErrRevisao      = errors.New("revisão obsoleta")
)

// ErroInicio é janela futura, com contagem para o handler do 409 (errors.As).
type ErroInicio struct{ Segundos int64 }

func (e *ErroInicio) Error() string { return "janela ainda não começou" }

// EstadoJanela é o resultado do recálculo da janela no servidor.
type EstadoJanela struct {
	Iniciada          bool
	Atrasada          bool // passou o prazo, atraso ainda possível
	Terminada         bool // entrega não mais possível (nem atrasada)
	FimReal           time.Time
	SegundosRestantes int64
}

// Aberta é a resposta consolidada do abrir: carga integral (o handler
// sanitiza) + estado corrente da tentativa.
type Aberta struct {
	Carga     contract.Carga
	FimReal   time.Time
	Restantes int64
	Atrasada  bool
	Revisao   int
	Respostas map[string]any
}

// Abrir cria ou devolve a tentativa do aluno (spec §5.1, idempotente).
func (s *Service) Abrir(ctx context.Context, mid string, aluno int64, agora time.Time) (*Aberta, error) {
	c, err := s.carregar(ctx, mid)
	if err != nil {
		return nil, err
	}
	if c.status != "ativa" {
		return nil, ErrEncerrada
	}
	est := avaliarJanela(c.janela, agora)
	if !est.Iniciada {
		return nil, &ErroInicio{Segundos: int64(c.janela.Inicio.Sub(agora) / time.Second)}
	}
	if est.Terminada {
		return nil, ErrEncerrada
	}
	if finalizada(ctx, s.pool, mid, aluno, c.regras) {
		return nil, ErrFinalizada
	}

	// fim_real efetivo: dentro do prazo = min(inicio+duração, prazo);
	// em atraso = prazo + limite (limite desconhecido: submissão imediata).
	fimReal := c.janela.Prazo
	if !est.Atrasada && c.janela.DuracaoSeg != nil {
		if d := c.janela.Inicio.Add(time.Duration(*c.janela.DuracaoSeg) * time.Second); d.Before(fimReal) {
			fimReal = d
		}
	} else if est.Atrasada {
		if lim, ok := parseIsoDur(ptrStr(c.janela.LimiteAtraso)); ok {
			fimReal = c.janela.Prazo.Add(lim)
		} else {
			fimReal = agora
		}
	}

	// Idempotente: primeira chamada define aberta_em.
	if _, err := s.pool.Exec(ctx, `INSERT INTO tentativa
	    (monitor_id, aluno_api_id, aberta_em, fim_real, revisao, dados)
	    VALUES ($1, $2, $3, $4, 0, '{}'::jsonb)
	    ON CONFLICT (monitor_id, aluno_api_id) DO NOTHING`,
		mid, aluno, agora, fimReal); err != nil {
		return nil, fmt.Errorf("tentativas: abrindo: %w", err)
	}

	var fimRealRow time.Time
	var revisao int
	var dados []byte
	if err := s.pool.QueryRow(ctx, `SELECT fim_real, revisao, dados
	    FROM tentativa WHERE monitor_id = $1 AND aluno_api_id = $2`,
		mid, aluno).Scan(&fimRealRow, &revisao, &dados); err != nil {
		return nil, fmt.Errorf("tentativas: lendo tentativa: %w", err)
	}
	resp := map[string]any{}
	if len(dados) > 0 {
		_ = json.Unmarshal(dados, &resp)
	}
	restantes := fimRealRow.Sub(agora)
	if restantes < 0 {
		restantes = 0
	}
	return &Aberta{
		Carga:     c.carga,
		FimReal:   fimRealRow,
		Restantes: int64(restantes / time.Second),
		Atrasada:  est.Atrasada,
		Revisao:   revisao,
		Respostas: resp,
	}, nil
}

// Salvar grava respostas com revisão otimista (spec §5.2) e, por intervalo,
// fixa um rascunho_ponto imutável do estado ANTERIOR à gravação.
// Snapshot intervalo vindo do cfg (0 = snapshota em toda gravação).
func (s *Service) Salvar(ctx context.Context, mid string, aluno int64,
	respostas map[string]any, revisao int, agora time.Time) (int, error) {
	c, err := s.carregar(ctx, mid)
	if err != nil {
		return 0, ErrSemTentativa // join vazio: não há tentativa
	}
	est := avaliarJanela(c.janela, agora)
	if !est.Iniciada {
		return 0, &ErroInicio{Segundos: int64(c.janela.Inicio.Sub(agora) / time.Second)}
	}
	if est.Terminada {
		return 0, ErrEncerrada
	}
	body, err := json.Marshal(respostas)
	if err != nil {
		return 0, fmt.Errorf("tentativas: respostas: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("tentativas: tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if s.precisaSnapshot(ctx, tx, mid, aluno, agora) {
		var dadosAnteriores []byte
		if err := tx.QueryRow(ctx, `SELECT dados FROM tentativa
		    WHERE monitor_id = $1 AND aluno_api_id = $2 FOR UPDATE`, mid, aluno,
		).Scan(&dadosAnteriores); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, ErrSemTentativa
			}
			return 0, fmt.Errorf("tentativas: lendo p/ snapshot: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO rascunho_ponto
		    (monitor_id, aluno_api_id, fixado_em, dados) VALUES ($1, $2, $3, $4::jsonb)`,
			mid, aluno, agora, string(dadosAnteriores)); err != nil {
			return 0, fmt.Errorf("tentativas: snapshot: %w", err)
		}
	}

	// 0 linhas = outra aba gravou antes (revisão divergente → 409).
	var nova int
	err = tx.QueryRow(ctx, `UPDATE tentativa SET dados = $3::jsonb, revisao = revisao + 1
	    WHERE monitor_id = $1 AND aluno_api_id = $2 AND revisao = $4
	    RETURNING revisao`,
		mid, aluno, string(body), revisao).Scan(&nova)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrRevisao
	}
	if err != nil {
		return 0, fmt.Errorf("tentativas: gravando: %w", err)
	}
	return nova, tx.Commit(ctx)
}

// precisaSnapshot decide pela regra do spec §5.2: agora - ultimo_ponto >=
// intervalo (0 = snapshota sempre). Sem ponto anterior, a referência é aberta_em.
func (s *Service) precisaSnapshot(ctx context.Context, tx pgx.Tx, mid string, aluno int64, agora time.Time) bool {
	if s.intervalo <= 0 {
		return true
	}
	var ultimo *time.Time
	_ = tx.QueryRow(ctx, `SELECT max(fixado_em) FROM rascunho_ponto
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&ultimo)
	if ultimo == nil {
		var abertaEm time.Time
		if err := s.pool.QueryRow(ctx, `SELECT aberta_em FROM tentativa
		    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&abertaEm); err != nil {
			return false // sem tentativa: UPDATE seguinte devolve ErrSemTentativa
		}
		return agora.Sub(abertaEm) >= s.intervalo
	}
	return agora.Sub(*ultimo) >= s.intervalo
}

// RevisaoAtual devolve a revisão corrente (para o 409 com detalhe ao cliente).
func (s *Service) RevisaoAtual(ctx context.Context, mid string, aluno int64) (int, error) {
	var r int
	if err := s.pool.QueryRow(ctx, `SELECT revisao FROM tentativa
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&r); err != nil {
		return 0, ErrSemTentativa
	}
	return r, nil
}

// finalizada aplica o bloqueio §5.6: submissões únicas fecham após 1 entrega;
// multiplas respeitam max_submissoes (0 = ilimitado, não bloqueia).
func finalizada(ctx context.Context, pool *pgxpool.Pool, mid string, aluno int64, r contract.Regras) bool {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM submissao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&n); err != nil {
		return true // falha de leitura não deve abrir nova tentativa
	}
	if n > 0 {
		if !r.SubmissoesMultiplas {
			return true
		}
		if r.MaxSubmissoes > 0 && n >= r.MaxSubmissoes {
			return true
		}
	}
	return false
}

// --- carga e janela --------------------------------------------------------

type cargaEstrutura struct {
	status string
	janela contract.Janela
	regras contract.Regras
	carga  contract.Carga
}

func (s *Service) carregar(ctx context.Context, mid string) (*cargaEstrutura, error) {
	var cargaJSON []byte
	var status string
	if err := s.pool.QueryRow(ctx,
		`SELECT carga, status FROM atividade WHERE monitor_id = $1`, mid,
	).Scan(&cargaJSON, &status); err != nil {
		return nil, fmt.Errorf("tentativas: atividade %s: %w", mid, err)
	}
	var t struct {
		Janela contract.Janela `json:"janela"`
		Regras contract.Regras `json:"regras"`
		Carga  contract.Carga  `json:"carga"`
	}
	if err := json.Unmarshal(cargaJSON, &t); err != nil {
		return nil, fmt.Errorf("tentativas: carga ilegível: %w", err)
	}
	return &cargaEstrutura{status: status, janela: t.Janela, regras: t.Regras, carga: t.Carga}, nil
}

// avaliarJanela é a regra única de janela (recalculada a cada operação).
// fim fixo = min(inicio+duração, prazo); depois do prazo, atraso conforme
// pode_atrasado + limite_atraso (null = "até encerramento da turma": fora do
// alcance do Monitor, tratado como entrega ainda possível — ver plano Fase 8).
func avaliarJanela(j contract.Janela, agora time.Time) EstadoJanela {
	if agora.Before(j.Inicio) {
		return EstadoJanela{} // Iniciada=false; handler deriva ErroInicio
	}
	est := EstadoJanela{Iniciada: true, FimReal: j.Prazo}
	if j.DuracaoSeg != nil {
		if d := j.Inicio.Add(time.Duration(*j.DuracaoSeg) * time.Second); d.Before(est.FimReal) {
			est.FimReal = d
		}
	}
	if agora.After(est.FimReal) {
		if !j.PodeAtrasado {
			est.Terminada = true
			return est
		}
		est.Atrasada = true
		if lim, ok := parseIsoDur(ptrStr(j.LimiteAtraso)); ok {
			est.FimReal = j.Prazo.Add(lim)
			if agora.After(est.FimReal) {
				est.Terminada = true
			}
		} else {
			est.FimReal = agora // limite desconhecido: decide no confirmar
		}
	}
	if est.FimReal.After(agora) {
		est.SegundosRestantes = int64(est.FimReal.Sub(agora) / time.Second)
	}
	return est
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// parseIsoDur lê o suportado de ISO-8601: "P<n>D[<n>H<n>M<n>S]"
// (spec usa "P1D"; limite desconhecido/ilegível devolve ok=false).
func parseIsoDur(s string) (time.Duration, bool) {
	if s == "" || !strings.HasPrefix(s, "P") {
		return 0, false
	}
	parts := strings.SplitN(s[1:], "T", 2)
	total := time.Duration(0)

	if data := parts[0]; data != "" {
		i := strings.Index(data, "D")
		if i <= 0 {
			return 0, false
		}
		n, err := strconv.Atoi(data[:i])
		if err != nil {
			return 0, false
		}
		total += time.Duration(n) * 24 * time.Hour
	}

	if len(parts) == 2 {
		rest := parts[1]
		for _, u := range []struct {
			ch  byte
			mul time.Duration
		}{{'H', time.Hour}, {'M', time.Minute}, {'S', time.Second}} {
			i := strings.IndexByte(rest, u.ch)
			if i < 0 {
				continue
			}
			if i == 0 {
				return 0, false
			}
			n, err := strconv.Atoi(rest[:i])
			if err != nil {
				return 0, false
			}
			total += time.Duration(n) * u.mul
			rest = rest[i+1:]
		}
		if rest != "" {
			return 0, false
		}
	}
	return total, true
}
