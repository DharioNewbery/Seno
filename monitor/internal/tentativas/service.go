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
	ErrEncerrada          = errors.New("janela encerrada")
	ErrFinalizada         = errors.New("cota de submissões atingida")
	ErrSemTentativa       = errors.New("sem tentativa aberta")
	ErrRevisao            = errors.New("revisão obsoleta")
	ErrSemTarefa          = errors.New("tarefa não encontrada")
	ErrTipoNaoArquivo     = errors.New("tarefa não aceita submissão de arquivo")
	ErrArquivoInexistente = errors.New("arquivo não encontrado")
)

// ArquivoTentativaMeta descreve os metadados de um arquivo anexado na tentativa.
type ArquivoTentativaMeta struct {
	TarefaOrdem  int       `json:"tarefa_ordem"`
	TarefaAPIID  int64     `json:"tarefa_api_id"`
	NomeArquivo  string    `json:"nome_arquivo"`
	MimeType     string    `json:"mime_type"`
	TamanhoBytes int64     `json:"tamanho_bytes"`
	Sha256       string    `json:"sha256"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

// ErroInicio é janela futura, com contagem para o handler do 409 (errors.As).
type ErroInicio struct{ Segundos int64 }

func (e *ErroInicio) Error() string { return "janela ainda não começou" }

// EstadoJanela é o resultado do recálculo da janela no servidor.
type EstadoJanela struct {
	Iniciada          bool
	Atrasada          bool // passou o prazo, atraso ainda possível
	Terminada         bool // entrega não mais possível (nem atrasada)
	FimReal           time.Time
	Prazo             time.Time // prazo bruto da janela (para segundos_atraso)
	SegundosRestantes int64
}

// Aberta é a resposta consolidada do abrir: carga integral (o handler
// sanitiza) + estado corrente da tentativa.
type Aberta struct {
	Carga     contract.Carga
	Regras    contract.Regras
	FimReal   time.Time
	Restantes int64
	Atrasada  bool
	Revisao   int
	Respostas map[string]any
	Arquivos  map[string]ArquivoTentativaMeta
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
	// §5.6: com submissões múltiplas, Nova tentativa transporta o conteúdo
	// da última submissão (seq_submissão segue o contador global).
	var seqSub int
	var dadosIniciais string
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM submissao
	    WHERE monitor_id = $1 AND aluno_api_id = $2`, mid, aluno).Scan(&seqSub); err != nil {
		return nil, fmt.Errorf("tentativas: contador de submissões: %w", err)
	}
	dadosIniciais = "{}"
	if seqSub > 0 && c.regras.SubmissoesMultiplas {
		var ultimo []byte
		if err := s.pool.QueryRow(ctx, `SELECT dados FROM submissao
		    WHERE monitor_id = $1 AND aluno_api_id = $2 ORDER BY n_t DESC LIMIT 1`,
			mid, aluno).Scan(&ultimo); err == nil {
			dadosIniciais = string(ultimo)
		}
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO tentativa
	    (monitor_id, aluno_api_id, aberta_em, fim_real, revisao, seq_submissao, dados)
	    VALUES ($1, $2, $3, $4, 0, $5, $6::jsonb)
	    ON CONFLICT (monitor_id, aluno_api_id) DO UPDATE SET fim_real = EXCLUDED.fim_real`,
		mid, aluno, agora, fimReal, seqSub, dadosIniciais); err != nil {
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
	arquivos := s.listarArquivosMeta(ctx, mid, aluno)
	return &Aberta{
		Carga:     c.carga,
		Regras:    c.regras,
		FimReal:   fimRealRow,
		Restantes: int64(restantes / time.Second),
		Atrasada:  est.Atrasada,
		Revisao:   revisao,
		Respostas: resp,
		Arquivos:  arquivos,
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

// VerificarJanela revalida a janela da atividade para operações fora do
// abrir/salvar (ensaios, pré-avaliação): devolve erro de negócio conforme
// EstadoJanela (ErroInicio/ErrEncerrada) ou nil.
func (s *Service) VerificarJanela(ctx context.Context, mid string, aluno int64, agora time.Time) (EstadoJanela, error) {
	c, err := s.carregar(ctx, mid)
	if err != nil {
		return EstadoJanela{}, ErrEncerrada // atividade ausente: bloqueia
	}
	est := avaliarJanela(c.janela, agora)
	if !est.Iniciada {
		return est, &ErroInicio{Segundos: int64(c.janela.Inicio.Sub(agora) / time.Second)}
	}
	if est.Terminada {
		return est, ErrEncerrada
	}
	return est, nil
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

func (s *Service) listarArquivosMeta(ctx context.Context, mid string, aluno int64) map[string]ArquivoTentativaMeta {
	out := map[string]ArquivoTentativaMeta{}
	rows, err := s.pool.Query(ctx, `SELECT tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, atualizado_em
	    FROM tentativa_arquivo WHERE monitor_id = $1 AND aluno_api_id = $2 ORDER BY tarefa_ordem`, mid, aluno)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var m ArquivoTentativaMeta
		if err := rows.Scan(&m.TarefaOrdem, &m.TarefaAPIID, &m.NomeArquivo, &m.MimeType, &m.TamanhoBytes, &m.Sha256, &m.AtualizadoEm); err == nil {
			out[strconv.Itoa(m.TarefaOrdem)] = m
		}
	}
	return out
}

// SalvarArquivo valida e grava o arquivo submetido pelo aluno para uma tarefa do tipo arquivo_submissao.
func (s *Service) SalvarArquivo(ctx context.Context, mid string, aluno int64, ordem int,
	nome string, dados []byte, maxPadrao int64, agora time.Time) (ArquivoTentativaMeta, int, error) {
	if _, err := s.VerificarJanela(ctx, mid, aluno, agora); err != nil {
		return ArquivoTentativaMeta{}, 0, err
	}
	c, err := s.carregar(ctx, mid)
	if err != nil {
		return ArquivoTentativaMeta{}, 0, ErrSemTentativa
	}
	var tarefa *contract.TarefaCarga
	for i := range c.carga.Tarefas {
		if c.carga.Tarefas[i].Ordem == ordem {
			tarefa = &c.carga.Tarefas[i]
			break
		}
	}
	if tarefa == nil {
		return ArquivoTentativaMeta{}, 0, ErrSemTarefa
	}
	if contract.NormalizarTipoTarefa(tarefa.Tipo) != contract.TipoTarefaArquivo {
		return ArquivoTentativaMeta{}, 0, ErrTipoNaoArquivo
	}
	var cfg contract.ConfigTarefaArquivo
	if tarefa.ConfigArquivo != nil {
		cfg = *tarefa.ConfigArquivo
	}
	val, err := contract.ValidarArquivo(nome, dados, cfg, maxPadrao)
	if err != nil {
		return ArquivoTentativaMeta{}, 0, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ArquivoTentativaMeta{}, 0, fmt.Errorf("tentativas: tx arquivo: %w", err)
	}
	defer tx.Rollback(ctx)

	var novaRevisao int
	err = tx.QueryRow(ctx, `UPDATE tentativa SET revisao = revisao + 1
	    WHERE monitor_id = $1 AND aluno_api_id = $2
	    RETURNING revisao`, mid, aluno).Scan(&novaRevisao)
	if errors.Is(err, pgx.ErrNoRows) {
		return ArquivoTentativaMeta{}, 0, ErrSemTentativa
	}
	if err != nil {
		return ArquivoTentativaMeta{}, 0, fmt.Errorf("tentativas: revisao arquivo: %w", err)
	}

	var atualizadoEm time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO tentativa_arquivo
	    (monitor_id, aluno_api_id, tarefa_ordem, tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, atualizado_em)
	    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	    ON CONFLICT (monitor_id, aluno_api_id, tarefa_ordem) DO UPDATE SET
	      tarefa_api_id = EXCLUDED.tarefa_api_id,
	      nome_arquivo = EXCLUDED.nome_arquivo,
	      mime_type = EXCLUDED.mime_type,
	      tamanho_bytes = EXCLUDED.tamanho_bytes,
	      sha256 = EXCLUDED.sha256,
	      conteudo = EXCLUDED.conteudo,
	      atualizado_em = EXCLUDED.atualizado_em
	    RETURNING atualizado_em`,
		mid, aluno, ordem, tarefa.TarefaAPIID, val.NomeSanitizado, val.MimeType, val.TamanhoBytes, val.Sha256, dados, agora,
	).Scan(&atualizadoEm); err != nil {
		return ArquivoTentativaMeta{}, 0, fmt.Errorf("tentativas: upsert arquivo: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ArquivoTentativaMeta{}, 0, err
	}
	return ArquivoTentativaMeta{
		TarefaOrdem:  ordem,
		TarefaAPIID:  tarefa.TarefaAPIID,
		NomeArquivo:  val.NomeSanitizado,
		MimeType:     val.MimeType,
		TamanhoBytes: val.TamanhoBytes,
		Sha256:       val.Sha256,
		AtualizadoEm: atualizadoEm,
	}, novaRevisao, nil
}

// ObterArquivo lê o arquivo anexado na tentativa (ou na última submissão se a tentativa já tiver sido encerrada).
func (s *Service) ObterArquivo(ctx context.Context, mid string, aluno int64, ordem int) (ArquivoTentativaMeta, []byte, error) {
	var m ArquivoTentativaMeta
	var conteudo []byte
	m.TarefaOrdem = ordem
	err := s.pool.QueryRow(ctx, `SELECT tarefa_api_id, nome_arquivo, mime_type, tamanho_bytes, sha256, conteudo, atualizado_em
	    FROM tentativa_arquivo
	    WHERE monitor_id = $1 AND aluno_api_id = $2 AND tarefa_ordem = $3`,
		mid, aluno, ordem).Scan(&m.TarefaAPIID, &m.NomeArquivo, &m.MimeType, &m.TamanhoBytes, &m.Sha256, &conteudo, &m.AtualizadoEm)
	if err == nil {
		return m, conteudo, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ArquivoTentativaMeta{}, nil, err
	}
	// Fallback: se o aluno já submeteu e quer baixar seu próprio arquivo enviado
	err = s.pool.QueryRow(ctx, `SELECT sa.tarefa_api_id, sa.nome_arquivo, sa.mime_type, sa.tamanho_bytes, sa.sha256, sa.conteudo, sa.criado_em
	    FROM submissao_arquivo sa
	    JOIN submissao s ON s.id = sa.submissao_id
	    WHERE s.monitor_id = $1 AND s.aluno_api_id = $2 AND sa.tarefa_ordem = $3
	    ORDER BY s.n_t DESC LIMIT 1`,
		mid, aluno, ordem).Scan(&m.TarefaAPIID, &m.NomeArquivo, &m.MimeType, &m.TamanhoBytes, &m.Sha256, &conteudo, &m.AtualizadoEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return ArquivoTentativaMeta{}, nil, ErrArquivoInexistente
	}
	if err != nil {
		return ArquivoTentativaMeta{}, nil, err
	}
	return m, conteudo, nil
}

// RemoverArquivo remove o arquivo anexado na tentativa aberta.
func (s *Service) RemoverArquivo(ctx context.Context, mid string, aluno int64, ordem int, agora time.Time) (int, error) {
	if _, err := s.VerificarJanela(ctx, mid, aluno, agora); err != nil {
		return 0, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var novaRevisao int
	err = tx.QueryRow(ctx, `UPDATE tentativa SET revisao = revisao + 1
	    WHERE monitor_id = $1 AND aluno_api_id = $2
	    RETURNING revisao`, mid, aluno).Scan(&novaRevisao)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrSemTentativa
	}
	if err != nil {
		return 0, err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM tentativa_arquivo
	    WHERE monitor_id = $1 AND aluno_api_id = $2 AND tarefa_ordem = $3`, mid, aluno, ordem)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() == 0 {
		return 0, ErrArquivoInexistente
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return novaRevisao, nil
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
	est := EstadoJanela{Iniciada: true, FimReal: j.Prazo, Prazo: j.Prazo}
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
