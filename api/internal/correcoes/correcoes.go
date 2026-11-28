// Package correcoes — correção da submissão (PROJETO §Correção): gerada
// na entrega; execução Judge0 assíncrona por lote com callback (apenas
// a API fala com o Judge0). Nota automática por tarefa: valor_pts quando
// TODOS os testes passam, 0 com reprovação; vazia em tarefas abertas.
// Sem Judge0 configurado, a correção fica pendente (grade manual).
// Professor ajusta nota/feedback, confirma e publica (ordem garantida).
// §Visibilidade: detalhes só alcançam o aluno após publicar.
package correcoes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/fila"
	"github.com/seno-project/seno/api/internal/judge0"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/contract"
)

// KindCorrecaoExecutar é o kind do job na fila PostgreSQL quando o Judge0
// estava indisponível no momento da submissão/reexecução.
const KindCorrecaoExecutar = "correcao_executar"

// Service coordena as correções.
type Service struct {
	store  *store.Store
	audit  *platform.Audit
	judge0 *judge0.Client
	cfg    judge0.Config
}

func New(st *store.Store, audit *platform.Audit, cli *judge0.Client, cfg judge0.Config) *Service {
	return &Service{store: st, audit: audit, judge0: cli, cfg: cfg}
}

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindCorrecaoCreate     = "correcao.create"
	LogKindCorrecaoUpdate     = "correcao.update"
	LogKindCorrecaoConfirm    = "correcao.confirmar"
	LogKindCorrecaoPublish    = "correcao.publicar"
	LogKindCorrecaoReexecutar = "correcao.reexecutar"
	LogKindCallback           = "correcao.callback"
)

// Status da correção (PROJETO §Correção).
const (
	StatusPendente   = "pendente"
	StatusExecutando = "executando"
	StatusConcluida  = "concluida"
	StatusFalhou     = "falhou"
)

// Correcao é a visão completa da correção.
type Correcao struct {
	ID           int64      `json:"id"`
	SubmissaoID  int64      `json:"submissao_id"`
	Status       string     `json:"status"`
	Feedback     string     `json:"feedback,omitempty"`
	ConfirmadaEm *time.Time `json:"confirmada_em,omitempty"`
	PublicadaEm  *time.Time `json:"publicada_em,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	Tarefas      []Tarefa   `json:"tarefas"`
}

// ArquivoMeta descreve o arquivo anexado numa tarefa de submissão de arquivo.
type ArquivoMeta struct {
	NomeArquivo  string    `json:"nome_arquivo"`
	MimeType     string    `json:"mime_type"`
	TamanhoBytes int64     `json:"tamanho_bytes"`
	Sha256       string    `json:"sha256"`
	CriadoEm     time.Time `json:"criado_em"`
}

// Tarefa é a correção de tarefa (nota automática/final + resultados).
type Tarefa struct {
	ID            int64                         `json:"id"`
	TarefaID      int64                         `json:"tarefa_id"`
	Ordem         int32                         `json:"ordem,omitempty"`
	Linguagem     string                        `json:"linguagem,omitempty"`
	TarefaNome    string                        `json:"tarefa_nome"`
	TarefaTipo    string                        `json:"tarefa_tipo"`
	ConfigArquivo *contract.ConfigTarefaArquivo `json:"config_arquivo,omitempty"`
	Enunciado     string                        `json:"enunciado,omitempty"`
	Codigo        string                        `json:"codigo,omitempty"`
	Arquivo       *ArquivoMeta                  `json:"arquivo,omitempty"`
	ValorPts      int32                         `json:"valor_pts"`
	NotaAuto      *int32                        `json:"nota_auto,omitempty"`
	NotaFinal     *int32                        `json:"nota_final,omitempty"`
	Feedback      *string                       `json:"feedback,omitempty"`
	Resultados    []Resultado                   `json:"resultados"`
}

// Resultado é a execução de um teste (veredito Judge0).
type Resultado struct {
	TesteID        int64   `json:"teste_id"`
	Status         *string `json:"status,omitempty"`
	Stdout         *string `json:"stdout,omitempty"`
	Stderr         *string `json:"stderr,omitempty"`
	CompileOutput  *string `json:"compile_output,omitempty"`
	Tempo          *string `json:"tempo,omitempty"`
	Memoria        *string `json:"memoria,omitempty"`
	CPU            *string `json:"cpu,omitempty"`
	Stdin          string  `json:"stdin,omitempty"`
	StdoutEsperado string  `json:"stdout_esperado,omitempty"`
	Publico        bool    `json:"publico"`
}

// SubmissaoResumo é o contexto/professor da submissão corrigida.
type SubmissaoResumo struct {
	ID            int64     `json:"id"`
	AtribuicaoID  int64     `json:"atribuicao_id"`
	Aluno         string    `json:"aluno"`
	AlunoEmail    string    `json:"aluno_email"`
	AtividadeNome string    `json:"atividade_nome"`
	EntregueEm    time.Time `json:"entregue_em"`
	Atrasada      bool      `json:"atrasada"`
}

// VisaoProfessor é a submissão + correção completa (professor|staff).
type VisaoProfessor struct {
	Submissao SubmissaoResumo `json:"submissao"`
	Correcao  *Correcao       `json:"correcao"`
}

// VisaoAluno: pré-publicação traz apenas o estado (sem detalhes);
// pós-publicação traz tudo, inclusive testes privados (§Visibilidade).
type VisaoAluno struct {
	Publicada     bool       `json:"publicada"`
	PublicadaEm   *time.Time `json:"publicada_em,omitempty"`
	Confirmada    bool       `json:"confirmada,omitempty"`
	AtribuicaoID  int64      `json:"atribuicao_id,omitempty"`
	AtividadeNome string     `json:"atividade_nome,omitempty"`
	Correcao      *Correcao  `json:"correcao,omitempty"`
}

// Edição do professor (PATCH): feedback geral + notas por tarefa.
type CadastroEdicao struct {
	Feedback *string        `json:"feedback,omitempty"`
	Tarefas  []EdicaoTarefa `json:"tarefas,omitempty"`
}

// EdicaoTarefa: nota final livre (pode ser parcial) + feedback.
type EdicaoTarefa struct {
	ID        int64   `json:"id"`
	NotaFinal *int32  `json:"nota_final,omitempty"`
	Feedback  *string `json:"feedback,omitempty"`
}

// ------------------------------------------------------------ helpers

func horaPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func int8Ptr(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

func textoPtr(s *string) *string { return s }

func textoDe(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func eDuplicado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func jaNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Submissão não encontrada.")
}

func conflito(msg string) error {
	return platform.NewAPIError(http.StatusConflict, platform.CodeConflict, msg)
}

// snapshotTasks decodifica {"tasks":[{"id":X,"text":"y"}]}.
type snapshotTasks struct {
	Tasks []struct {
		ID   int64  `json:"id"`
		Text string `json:"text"`
	} `json:"tasks"`
}

// ------------------------------------------------------------ montagem

// montar devolve a correção completa (tarefas + resultados) para a
// submissão informada (usada pelo professor e pelo aluno publicado).
func (s *Service) montar(ctx context.Context, submissaoID int64) (*Correcao, error) {
	row, err := s.store.Q.GetCorrecaoPorSubmissao(ctx, submissaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s.montarDeRow(ctx, row)
}

func (s *Service) montarDeRow(
	ctx context.Context, row gen.GetCorrecaoPorSubmissaoRow,
) (*Correcao, error) {
	cts, err := s.store.Q.ListCorrecaoTarefas(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(cts))
	for _, ct := range cts {
		ids = append(ids, ct.ID)
	}
	var resRows []gen.ListResultadosRow
	if len(ids) > 0 {
		resRows, err = s.store.Q.ListResultados(ctx, ids)
		if err != nil {
			return nil, err
		}
	}
	porCT := map[int64][]Resultado{}
	for _, r := range resRows {
		porCT[r.CorrecaoTarefaID] = append(porCT[r.CorrecaoTarefaID], Resultado{
			TesteID: r.TesteID, Status: textoPtr(r.Status),
			Stdout: textoPtr(r.Stdout), Stderr: textoPtr(r.Stderr),
			CompileOutput: textoPtr(r.CompileOutput),
			Tempo:         textoPtr(r.Tempo), Memoria: textoPtr(r.Memoria),
			CPU:            textoPtr(r.Cpu),
			Stdin:          r.Stdin,
			StdoutEsperado: r.StdoutEsperado,
			Publico:        r.Publico,
		})
	}
	codigosPorTarefa := map[int64]string{}
	ordemPorTarefa := map[int64]int32{}
	lingPorTarefa := map[int64]string{}
	if sub, err := s.store.Q.GetSubmissaoCorrecao(ctx, row.SubmissaoID); err == nil {
		if len(sub.Snapshot) > 0 {
			var st snapshotTasks
			if err := json.Unmarshal(sub.Snapshot, &st); err == nil {
				for _, t := range st.Tasks {
					codigosPorTarefa[t.ID] = t.Text
				}
			}
		}
		if tas, errT := s.store.Q.TarefasDaAtribuicao(ctx, sub.AtribuicaoID); errT == nil {
			for _, ta := range tas {
				ordemPorTarefa[ta.TarefaID] = ta.Ordem
				lingPorTarefa[ta.TarefaID] = ta.Linguagem
			}
		}
	}
	arquivosPorTarefa := map[int64]*ArquivoMeta{}
	if arqs, err := s.store.Q.ListSubmissaoArquivosMeta(ctx, row.SubmissaoID); err == nil {
		for _, a := range arqs {
			arquivosPorTarefa[a.TarefaID] = &ArquivoMeta{
				NomeArquivo:  a.NomeArquivo,
				MimeType:     a.MimeType,
				TamanhoBytes: a.TamanhoBytes,
				Sha256:       a.Sha256,
				CriadoEm:     a.CriadoEm.Time,
			}
		}
	}
	tarefas := make([]Tarefa, 0, len(cts))
	for idx, ct := range cts {
		tipo := contract.NormalizarTipoTarefa(ct.TarefaTipo)
		ordem := ordemPorTarefa[ct.TarefaID]
		if ordem <= 0 {
			ordem = int32(idx + 1)
		}
		resultados := porCT[ct.ID]
		if resultados == nil {
			resultados = []Resultado{}
		}
		item := Tarefa{
			ID: ct.ID, TarefaID: ct.TarefaID,
			Ordem:      ordem,
			Linguagem:  lingPorTarefa[ct.TarefaID],
			TarefaNome: ct.TarefaNome,
			TarefaTipo: tipo,
			Enunciado:  ct.TarefaEnunciado,
			Codigo:     codigosPorTarefa[ct.TarefaID],
			Arquivo:    arquivosPorTarefa[ct.TarefaID],
			ValorPts:   ct.ValorPts,
			NotaAuto:   notaPtr(ct.NotaAuto), NotaFinal: notaPtr(ct.NotaFinal),
			Feedback:   textoPtr(ct.Feedback),
			Resultados: resultados,
		}
		if tipo == contract.TipoTarefaArquivo {
			var cfg contract.ConfigTarefaArquivo
			if len(ct.TarefaConfig) > 0 {
				_ = json.Unmarshal(ct.TarefaConfig, &cfg)
			}
			if norm, err := contract.NormalizarConfigArquivo(cfg, contract.DefaultArquivoMaxBytes); err == nil {
				item.ConfigArquivo = &norm
			}
		}
		tarefas = append(tarefas, item)
	}
	return &Correcao{
		ID: row.ID, SubmissaoID: row.SubmissaoID, Status: row.Status,
		Feedback:     textoDe(row.Feedback),
		ConfirmadaEm: horaPtr(row.ConfirmadaEm),
		PublicadaEm:  horaPtr(row.PublicadaEm),
		CreatedAt:    row.CreatedAt.Time,
		Tarefas:      tarefas,
	}, nil
}

// ------------------------------------------------------------ iniciar

// Iniciar cria a correção (idempotente) e, com Judge0 ligado, despacha
// o lote (callback por item); sem Judge0 fica pendente p/ grade manual.
func (s *Service) Iniciar(
	ctx context.Context, submissaoID int64, createdBy int64, actorEmail string,
) (*Correcao, error) {
	sub, err := s.store.Q.GetSubmissaoCorrecao(ctx, submissaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, jaNaoEncontrada()
		}
		return nil, err
	}
	row, err := s.store.Q.InsertCorrecao(ctx, gen.InsertCorrecaoParams{
		SubmissaoID: submissaoID, Status: StatusPendente,
		CreatedBy: int8Ptr(createdBy),
	})
	if eDuplicado(err) {
		return s.montar(ctx, submissaoID)
	}
	if err != nil {
		return nil, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &createdBy, Actor: actorEmail,
		Kind:   LogKindCorrecaoCreate,
		Detail: fmt.Sprintf("submissao=%d correcao=%d", submissaoID, row.ID),
	})

	if err := s.despachar(ctx, row.ID, sub, createdBy); err != nil {
		return nil, err
	}
	return s.montar(ctx, submissaoID)
}

func (s *Service) despachar(
	ctx context.Context, correcaoID int64, sub gen.GetSubmissaoCorrecaoRow, createdBy int64,
) error {
	return s.despacharInterno(ctx, correcaoID, sub, createdBy, true)
}

type payloadCorrecaoJob struct {
	CorrecaoID  int64 `json:"correcao_id"`
	SubmissaoID int64 `json:"submissao_id"`
	CreatedBy   int64 `json:"created_by"`
}

func (s *Service) despacharInterno(
	ctx context.Context, correcaoID int64, sub gen.GetSubmissaoCorrecaoRow, createdBy int64, enfileirarEmFalha bool,
) error {
	// Correção de tarefa: por tarefa da atividade (ordem/valor).
	tarefasDaAtividade, err := s.store.Q.TarefasDaAtribuicao(ctx, sub.AtribuicaoID)
	if err != nil {
		return err
	}
	var snap snapshotTasks
	if len(sub.Snapshot) > 0 {
		if err := json.Unmarshal(sub.Snapshot, &snap); err != nil {
			return fmt.Errorf("snapshot: %w", err)
		}
	}
	fontes := map[int64]string{}
	for _, t := range snap.Tasks {
		fontes[t.ID] = t.Text
	}

	statusEnvio := StatusPendente
	var itens []judge0.SubmissaoJudge0
	teveTesteResolvidoLocal := false
	for _, at := range tarefasDaAtividade {
		// Correção de tarefa nasce sempre (grade manual funciona mesmo
		// sem Judge0; com Judge0, recebe os resultados por callback).
		ctID, err := s.criarCT(ctx, correcaoID, at.TarefaID, at.ValorPts)
		if err != nil {
			return err
		}
		if contract.NormalizarTipoTarefa(at.TarefaTipo) != contract.TipoTarefaProgramacao {
			continue
		}
		if !s.cfg.Ligada() {
			continue
		}
		lingID, ok := s.cfg.LinguagemDo(at.Linguagem)
		if !ok {
			continue
		}
		testes, err := s.store.Q.ListTestesTarefa(ctx, at.TarefaID)
		if err != nil {
			return err
		}
		codigo := fontes[at.TarefaID]
		if strings.TrimSpace(codigo) == "" {
			// Judge0 rejeita source_code em branco com 422; grava reprovação imediata.
			for _, te := range testes {
				if err := s.AplicarResultado(ctx, ctID, te.ID, judge0.RespostaJudge0{
					Status: judge0.RawStatus(judge0.StatusWrongAnswerID),
				}); err != nil {
					return err
				}
				teveTesteResolvidoLocal = true
			}
			continue
		}
		for _, te := range testes {
			itens = append(itens, judge0.SubmissaoJudge0{
				LanguageID:     lingID,
				SourceCode:     codigo,
				Stdin:          te.Stdin,
				ExpectedOutput: te.StdoutEsperado,
				CallbackURL:    s.callbackURL(ctID, te.ID),
			})
		}
	}
	if s.cfg.Ligada() && len(itens) > 0 {
		statusEnvio = StatusExecutando
		if err := s.judge0.EnviarBatch(ctx, itens); err != nil {
			s.audit.Record(ctx, platform.LogEntry{
				ActorID: &createdBy, Actor: "sistema",
				Kind:   "correcao.judge0_falhou",
				Detail: fmt.Sprintf("correcao=%d err=%v", correcaoID, err),
			})
			if !enfileirarEmFalha {
				return fmt.Errorf("judge0 indisponível na correção %d: %w", correcaoID, err)
			}
			// Judge0 indisponível: armazena na fila persistente para execução posterior.
			if _, qErr := fila.Agenda(ctx, s.store, KindCorrecaoExecutar, payloadCorrecaoJob{
				CorrecaoID:  correcaoID,
				SubmissaoID: sub.ID,
				CreatedBy:   createdBy,
			}, 3); qErr == nil {
				statusEnvio = StatusPendente
			} else {
				statusEnvio = StatusFalhou
			}
		}
	} else if s.cfg.Ligada() && teveTesteResolvidoLocal {
		faltando, errF := s.store.Q.ContarResultadosFaltando(ctx, correcaoID)
		if errF == nil && faltando == 0 {
			statusEnvio = StatusConcluida
		}
	}
	if statusEnvio != StatusPendente {
		if err := s.store.Q.UpdateStatusCorrecao(ctx, gen.UpdateStatusCorrecaoParams{
			ID: correcaoID, Status: statusEnvio,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Handler devolve o worker da fila para reenviar correções pendentes ao Judge0.
func (s *Service) Handler() fila.Handler {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p payloadCorrecaoJob
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("correcao_executar: payload: %w", err)
		}
		corRow, err := s.store.Q.GetCorrecaoPorSubmissao(ctx, p.SubmissaoID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if corRow.ConfirmadaEm.Valid || corRow.PublicadaEm.Valid || corRow.Status == StatusConcluida {
			return nil
		}
		sub, err := s.store.Q.GetSubmissaoCorrecao(ctx, p.SubmissaoID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		return s.despacharInterno(ctx, corRow.ID, sub, p.CreatedBy, false)
	}
}

// MarcarFalhaJob marca a correção como 'falhou' quando todas as tentativas da fila se esgotam.
func (s *Service) MarcarFalhaJob(ctx context.Context, payload json.RawMessage) {
	var p payloadCorrecaoJob
	if err := json.Unmarshal(payload, &p); err != nil || p.CorrecaoID == 0 {
		return
	}
	_, _ = s.store.Pool.Exec(ctx, `
		UPDATE correcoes SET status = 'falhou'
		WHERE id = $1 AND status IN ('pendente', 'executando') AND confirmada_em IS NULL`,
		p.CorrecaoID)
}

// Reexecutar reinicia a avaliação automática da correção (dono|staff).
// Permitido para status 'falhou' ou 'concluida' ainda NÃO confirmada.
func (s *Service) Reexecutar(
	ctx context.Context, actor domain.User, submissaoID int64,
) (VisaoProfessor, error) {
	esc, err := s.escopoProfessor(ctx, actor, submissaoID)
	if err != nil {
		return VisaoProfessor{}, err
	}
	sub, err := s.store.Q.GetSubmissaoCorrecao(ctx, submissaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VisaoProfessor{}, jaNaoEncontrada()
		}
		return VisaoProfessor{}, err
	}
	corRow, err := s.store.Q.GetCorrecaoPorSubmissao(ctx, submissaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VisaoProfessor{}, jaNaoEncontrada()
		}
		return VisaoProfessor{}, err
	}
	if corRow.Status == StatusExecutando {
		return VisaoProfessor{}, conflito("Correção já está em execução.")
	}
	if corRow.ConfirmadaEm.Valid {
		return VisaoProfessor{}, conflito("Correção já confirmada não pode ser reexecutada.")
	}

	if err := s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		if err := q.ResetResultadosCorrecao(ctx, corRow.ID); err != nil {
			return err
		}
		if err := q.ResetNotasCorrecaoTarefas(ctx, corRow.ID); err != nil {
			return err
		}
		return q.ResetStatusCorrecao(ctx, gen.ResetStatusCorrecaoParams{
			ID: corRow.ID, Status: StatusPendente,
		})
	}); err != nil {
		return VisaoProfessor{}, err
	}

	if err := s.despachar(ctx, corRow.ID, sub, actor.ID); err != nil {
		return VisaoProfessor{}, err
	}

	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindCorrecaoReexecutar,
		Detail: fmt.Sprintf("submissao=%d correcao=%d aluno=%d", submissaoID, corRow.ID, esc.AlunoID),
	})

	return s.VerProfessor(ctx, actor, submissaoID)
}

// criarCT numa transaçãozinha própria (idempotente).
func (s *Service) criarCT(
	ctx context.Context, correcaoID, tarefaID int64, valorPts int32,
) (int64, error) {
	if err := s.store.Q.InsertCorrecaoTarefa(
		ctx, gen.InsertCorrecaoTarefaParams{
			CorrecaoID: correcaoID, TarefaID: tarefaID, ValorPts: valorPts,
		},
	); err != nil && !eDuplicado(err) {
		return 0, err
	}
	lista, err := s.store.Q.ListCorrecaoTarefas(ctx, correcaoID)
	if err != nil {
		return 0, err
	}
	for _, ct := range lista {
		if ct.TarefaID == tarefaID {
			return ct.ID, nil
		}
	}
	return 0, errors.New("correcao_tarefa não encontrada após insert")
}

func (s *Service) callbackURL(ctID, testeID int64) string {
	return fmt.Sprintf("%s/v1/judge0/callback?par=%d:%d&key=%s",
		strings.TrimSuffix(s.cfg.Origem, "/"), ctID, testeID, s.cfg.Secret)
}

// ------------------------------------------------------- callbacks

// AplicarResultado grava o veredito de um teste e recalcula a nota
// automática; ao fechar todos os testes, marca a correção concluída.
func (s *Service) AplicarResultado(
	ctx context.Context, correcaoTarefaID, testeID int64, r judge0.RespostaJudge0,
) error {
	if r.EmFila() {
		// Estados transitórios do Judge0 (1=In Queue, 2=Processing) não fecham o teste.
		return nil
	}
	ct, err := s.store.Q.GetCorrecaoTarefa(ctx, correcaoTarefaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return jaNaoEncontrada()
		}
		return err
	}
	status := judge0.StatusVeredito(r.StatusID())
	tempoS := r.Time.String()
	memS := r.Memory.String()
	cpuS := r.CPUTime.String()
	if err := s.store.Q.AplicarResultado(ctx, gen.AplicarResultadoParams{
		CorrecaoTarefaID: correcaoTarefaID,
		TesteID:          testeID,
		Status:           &status,
		Stdout:           textoL(&r.Stdout), Stderr: textoL(&r.Stderr),
		CompileOutput: textoL(&r.CompileOutput),
		Tempo:         textoL(&tempoS), Memoria: textoL(&memS), Cpu: textoL(&cpuS),
	}); err != nil {
		return err
	}
	if err := s.recalcularNotaAuto(ctx, correcaoTarefaID); err != nil {
		return err
	}
	faltando, err := s.store.Q.ContarResultadosFaltando(ctx, ct.CorrecaoID)
	if err != nil {
		return err
	}
	if faltando == 0 {
		if err := s.store.Q.UpdateStatusCorrecao(ctx, gen.UpdateStatusCorrecaoParams{
			ID: ct.CorrecaoID, Status: StatusConcluida,
		}); err != nil {
			return err
		}
	}
	s.audit.Record(ctx, platform.LogEntry{
		Actor: "judge0",
		Kind:  LogKindCallback,
		Detail: fmt.Sprintf("correcao=%d tarefa=%d teste=%d status=%s",
			ct.CorrecaoID, ct.TarefaID, testeID, status),
	})
	return nil
}

// recalcularNotaAuto: completa (resultados == testes) → valor se zero
// reprovações; 0 caso haja reprovação; tarefas abertas seguem NULL.
func (s *Service) recalcularNotaAuto(ctx context.Context, ctID int64) error {
	na, err := s.store.Q.NotaAutoCorrecaoTarefa(ctx, ctID)
	if err != nil {
		return err
	}
	if na.NumTestes == 0 || na.Aplicados < na.NumTestes {
		return nil
	}
	var nota int32
	if na.Reprovados == 0 {
		nota = na.ValorPts
	}
	return s.store.Q.SetNotaAuto(ctx, gen.SetNotaAutoParams{
		ID: ctID, NotaAuto: pgtype.Int4{Int32: nota, Valid: true},
	})
}

// -------------------------------------------------------- professor

// escopoProfessor: staff passa; professor fora do dono → 404.
func (s *Service) escopoProfessor(
	ctx context.Context, actor domain.User, submissaoID int64,
) (gen.GetSubmissaoEscopoRow, error) {
	row, err := s.store.Q.GetSubmissaoEscopo(ctx, submissaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return row, jaNaoEncontrada()
		}
		return row, err
	}
	if !actor.IsStaff() && actor.ID != row.TurmaProfessorID {
		return row, jaNaoEncontrada()
	}
	return row, nil
}

// VerProfessor devolve submissão + correção completa.
func (s *Service) VerProfessor(
	ctx context.Context, actor domain.User, submissaoID int64,
) (VisaoProfessor, error) {
	row, err := s.escopoProfessor(ctx, actor, submissaoID)
	if err != nil {
		return VisaoProfessor{}, err
	}
	cor, err := s.montar(ctx, submissaoID)
	if err != nil {
		return VisaoProfessor{}, err
	}
	subRow, _ := s.store.Q.GetSubmissaoPorID(ctx, submissaoID)
	return VisaoProfessor{
		Submissao: SubmissaoResumo{
			ID:            row.ID,
			AtribuicaoID:  row.AtribuicaoID,
			Aluno:         row.AlunoFirstName + " " + row.AlunoLastName,
			AlunoEmail:    row.AlunoEmail,
			AtividadeNome: row.AtividadeNome,
			EntregueEm:    subRow.EntregueEm.Time,
			Atrasada:      subRow.Atrasada,
		},
		Correcao: cor,
	}, nil
}

// Editar ajusta feedback geral e notas/feedback por tarefa (dono|staff).
// Publicada → 409 (§Visibilidade; página fica congelada).
func (s *Service) Editar(
	ctx context.Context, actor domain.User, submissaoID int64, cad CadastroEdicao,
) (VisaoProfessor, error) {
	_, err := s.escopoProfessor(ctx, actor, submissaoID)
	if err != nil {
		return VisaoProfessor{}, err
	}
	cor, err := s.montar(ctx, submissaoID)
	if err != nil {
		return VisaoProfessor{}, err
	}
	if cor == nil {
		return VisaoProfessor{}, jaNaoEncontrada()
	}
	if cor.PublicadaEm != nil {
		return VisaoProfessor{}, conflito("Correção já publicada.")
	}
	if cad.Feedback != nil {
		if err := s.store.Q.UpdateFeedbackCorrecao(ctx, gen.UpdateFeedbackCorrecaoParams{
			ID: cor.ID, Feedback: cad.Feedback,
		}); err != nil {
			return VisaoProfessor{}, err
		}
	}
	valor := map[int64]int32{}
	for _, t := range cor.Tarefas {
		valor[t.ID] = t.ValorPts
	}
	for _, t := range cad.Tarefas {
		linhas, err := s.store.Q.AjustarCorrecaoTarefa(ctx, gen.AjustarCorrecaoTarefaParams{
			CorrecaoID: cor.ID, // guarda de escopo: o id do corpo só v vale DENTRO desta correção
			ID:         t.ID,
			NotaFinal:  notaTravada(t.NotaFinal, valor[t.ID]),
			Feedback:   textoL(t.Feedback),
		})
		if err != nil {
			return VisaoProfessor{}, err
		}
		if linhas == 0 {
			return VisaoProfessor{}, jaNaoEncontrada()
		}
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindCorrecaoUpdate,
		Detail: fmt.Sprintf("submissao=%d", submissaoID),
	})
	return s.VerProfessor(ctx, actor, submissaoID)
}

// Confirmar registra quem confirmou (dono|staff); idempotente.
func (s *Service) Confirmar(
	ctx context.Context, actor domain.User, submissaoID int64,
) (VisaoProfessor, error) {
	if _, err := s.escopoProfessor(ctx, actor, submissaoID); err != nil {
		return VisaoProfessor{}, err
	}
	cor, err := s.montar(ctx, submissaoID)
	if err != nil || cor == nil {
		return VisaoProfessor{}, err
	}
	// Ao confirmar, assume nota_auto como nota_final para tarefas sem nota_final definida
	_, _ = s.store.Pool.Exec(ctx, `
		UPDATE correcao_tarefas
		SET nota_final = nota_auto
		WHERE correcao_id = $1 AND nota_final IS NULL AND nota_auto IS NOT NULL`, cor.ID)

	if cor.Status == StatusPendente {
		_ = s.store.Q.UpdateStatusCorrecao(ctx, gen.UpdateStatusCorrecaoParams{
			ID: cor.ID, Status: StatusConcluida,
		})
	}

	if _, err := s.store.Q.ConfirmarCorrecao(ctx, gen.ConfirmarCorrecaoParams{
		ID: cor.ID, ConfirmadaPor: int8Ptr(actor.ID),
	}); err != nil {
		return VisaoProfessor{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindCorrecaoConfirm,
		Detail: fmt.Sprintf("submissao=%d", submissaoID),
	})
	return s.VerProfessor(ctx, actor, submissaoID)
}

// Publicar torna visível ao aluno (exige confirmada; idempotente).
func (s *Service) Publicar(
	ctx context.Context, actor domain.User, submissaoID int64,
) (VisaoProfessor, error) {
	if _, err := s.escopoProfessor(ctx, actor, submissaoID); err != nil {
		return VisaoProfessor{}, err
	}
	cor, err := s.montar(ctx, submissaoID)
	if err != nil || cor == nil {
		return VisaoProfessor{}, err
	}
	if _, err := s.store.Q.PublicarCorrecao(ctx, cor.ID); err != nil {
		return VisaoProfessor{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindCorrecaoPublish,
		Detail: fmt.Sprintf("submissao=%d", submissaoID),
	})
	return s.VerProfessor(ctx, actor, submissaoID)
}

// PublicarLote publica de uma vez as correções confirmadas e não
// publicadas da atribuição (dono|staff); devolve quantas publicou.
// Idempotente: já publicadas ficam de fora — reexecução devolve 0.
func (s *Service) PublicarLote(
	ctx context.Context, actor domain.User, atribuicaoID int64,
) (int64, error) {
	atr, err := s.store.Q.GetAtribuicao(ctx, atribuicaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, jaNaoEncontrada()
		}
		return 0, err
	}
	if !actor.IsStaff() && actor.ID != atr.TurmaProfessorID {
		return 0, jaNaoEncontrada()
	}
	n, err := s.store.Q.PublicarCorrecoesDaAtribuicao(ctx, atribuicaoID)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		s.audit.Record(ctx, platform.LogEntry{
			ActorID: &actor.ID, Actor: actor.Email,
			Kind:   LogKindCorrecaoPublish,
			Detail: fmt.Sprintf("atribuicao=%d publicadas=%d", atribuicaoID, n),
		})
	}
	return n, nil
}

// ------------------------------------------------------------ aluno

// VerAluno: correção da própria submissão — detalhes só se publicada.
func (s *Service) VerAluno(
	ctx context.Context, actor domain.User, submissaoID int64,
) (VisaoAluno, error) {
	row, err := s.store.Q.GetSubmissaoEscopo(ctx, submissaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VisaoAluno{}, jaNaoEncontrada()
		}
		return VisaoAluno{}, err
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleStudent) || row.AlunoID != actor.ID {
		return VisaoAluno{}, jaNaoEncontrada()
	}
	cor, err := s.montar(ctx, submissaoID)
	if err != nil {
		return VisaoAluno{}, err
	}
	visao := VisaoAluno{
		AtribuicaoID:  row.AtribuicaoID,
		AtividadeNome: row.AtividadeNome,
	}
	if cor != nil {
		visao.Confirmada = cor.ConfirmadaEm != nil
		if cor.PublicadaEm != nil {
			visao.Publicada = true
			visao.PublicadaEm = cor.PublicadaEm
			visao.Correcao = cor
		}
	}
	return visao, nil
}

// ObterArquivoSubmissao devolve o arquivo anexado a uma tarefa da submissão.
// Autorizado para: dono da turma (professor), staff (admin/super) ou o próprio aluno dono da submissão.
func (s *Service) ObterArquivoSubmissao(
	ctx context.Context, actor domain.User, submissaoID, tarefaID int64,
) (gen.SubmissaoArquivo, error) {
	esc, err := s.store.Q.GetSubmissaoEscopo(ctx, submissaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.SubmissaoArquivo{}, jaNaoEncontrada()
		}
		return gen.SubmissaoArquivo{}, err
	}
	autorizado := actor.IsStaff() ||
		actor.ID == esc.TurmaProfessorID ||
		(domain.HasAnyRole(actor.Roles, domain.RoleStudent) && actor.ID == esc.AlunoID)
	if !autorizado {
		return gen.SubmissaoArquivo{}, jaNaoEncontrada()
	}
	arq, err := s.store.Q.GetSubmissaoArquivo(ctx, gen.GetSubmissaoArquivoParams{
		SubmissaoID: submissaoID,
		TarefaID:    tarefaID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.SubmissaoArquivo{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound, "Arquivo não encontrado para esta tarefa.")
		}
		return gen.SubmissaoArquivo{}, err
	}
	return arq, nil
}

// ------------------------------------------------------------ helpers

func textoL(s *string) *string {
	if s == nil {
		return nil
	}
	return s
	// nota: valor é valor; apontador aqui só preserva a ausência.
}

func notaPtr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	out := v.Int32
	return &out
}

// notaTravada trava a nota final em [0, valor_pts] (id 1–1 com a regra da
// nota automática); campo ausente segue NOT NULL — não zera a nota.
func notaTravada(v *int32, valorPts int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	n := *v
	if n < 0 {
		n = 0
	}
	if n > valorPts {
		n = valorPts
	}
	return pgtype.Int4{Int32: n, Valid: true}
}

// ------------------------------------------------------------ watchdog

// FecharPerdidas é o watchdog da correção (§7.2): callback do Judge0
// perdido deixa a correção eternamente 'executando' (ContarResultados-
// Faltando nunca zera), sem sinal ao professor. Marca 'falhou' a correção
// 'executando' criada há mais de 10 min — todo item do Judge0 termina em
// segundos (limite por execução), 10 min cobre filas. Callback que chega
// tarde ainda conclui: AplicarResultado marca 'concluida' quando os
// resultados fecham — o estado se cura sozinho.
const janelaSemCallback = 10 * time.Minute

const fecharPerdidas = `UPDATE correcoes
SET status = 'falhou'
WHERE status = 'executando'
  AND created_at < now() - $1::interval
RETURNING id`

// FecharPerdidas executa o watchdog e devolve os ids afetados (0 na
// maioria das voltas); o chamador agenda o ciclo.
func (s *Service) FecharPerdidas(ctx context.Context) ([]int64, error) {
	rows, err := s.store.Pool.Query(ctx, fecharPerdidas, janelaSemCallback)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		s.audit.Record(ctx, platform.LogEntry{
			Actor:  "sistema",
			Kind:   "correcao.perdida",
			Detail: fmt.Sprintf("correcao=%d sem callback do Judge0", id),
		})
	}
	return ids, nil
}
