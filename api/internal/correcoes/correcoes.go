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
	"github.com/seno-project/seno/api/internal/judge0"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

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
	LogKindCorrecaoCreate  = "correcao.create"
	LogKindCorrecaoUpdate  = "correcao.update"
	LogKindCorrecaoConfirm = "correcao.confirmar"
	LogKindCorrecaoPublish = "correcao.publicar"
	LogKindCallback        = "correcao.callback"
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

// Tarefa é a correção de tarefa (nota automática/final + resultados).
type Tarefa struct {
	ID         int64       `json:"id"`
	TarefaID   int64       `json:"tarefa_id"`
	TarefaNome string      `json:"tarefa_nome"`
	ValorPts   int32       `json:"valor_pts"`
	NotaAuto   *int32      `json:"nota_auto,omitempty"`
	NotaFinal  *int32      `json:"nota_final,omitempty"`
	Feedback   *string     `json:"feedback,omitempty"`
	Resultados []Resultado `json:"resultados"`
}

// Resultado é a execução de um teste (veredito Judge0).
type Resultado struct {
	TesteID       int64   `json:"teste_id"`
	Status        *string `json:"status,omitempty"`
	Stdout        *string `json:"stdout,omitempty"`
	Stderr        *string `json:"stderr,omitempty"`
	CompileOutput *string `json:"compile_output,omitempty"`
	Tempo         *string `json:"tempo,omitempty"`
	Memoria       *string `json:"memoria,omitempty"`
	CPU           *string `json:"cpu,omitempty"`
	Publico       bool    `json:"publico"`
}

// SubmissaoResumo é o contexto/professor da submissão corrigida.
type SubmissaoResumo struct {
	ID            int64     `json:"id"`
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
	Publicada   bool       `json:"publicada"`
	PublicadaEm *time.Time `json:"publicada_em,omitempty"`
	Correcao    *Correcao  `json:"correcao,omitempty"`
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
			CPU: textoPtr(r.Cpu), Publico: r.Publico,
		})
	}
	tarefas := make([]Tarefa, 0, len(cts))
	for _, ct := range cts {
		tarefas = append(tarefas, Tarefa{
			ID: ct.ID, TarefaID: ct.TarefaID, TarefaNome: ct.TarefaNome,
			ValorPts: ct.ValorPts,
			NotaAuto: notaPtr(ct.NotaAuto), NotaFinal: notaPtr(ct.NotaFinal),
			Feedback:   textoPtr(ct.Feedback),
			Resultados: porCT[ct.ID],
		})
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

	// Correção de tarefa: por tarefa da atividade (ordem/valor).
	tarefasDaAtividade, err := s.store.Q.TarefasDaAtribuicao(ctx, sub.AtribuicaoID)
	if err != nil {
		return nil, err
	}
	var snap snapshotTasks
	if len(sub.Snapshot) > 0 {
		if err := json.Unmarshal(sub.Snapshot, &snap); err != nil {
			return nil, fmt.Errorf("snapshot: %w", err)
		}
	}
	fontes := map[int64]string{}
	for _, t := range snap.Tasks {
		fontes[t.ID] = t.Text
	}

	statusEnvio := StatusPendente
	var itens []judge0.SubmissaoJudge0
	for _, at := range tarefasDaAtividade {
		// Correção de tarefa nasce sempre (grade manual funciona mesmo
		// sem Judge0; com Judge0, recebe os resultados por callback).
		ctID, err := s.criarCT(ctx, row.ID, at.TarefaID, at.ValorPts)
		if err != nil {
			return nil, err
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
			return nil, err
		}
		for _, te := range testes {
			itens = append(itens, judge0.SubmissaoJudge0{
				LanguageID:     lingID,
				SourceCode:     fontes[at.TarefaID],
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
				Detail: fmt.Sprintf("correcao=%d err=%v", row.ID, err),
			})
			statusEnvio = StatusFalhou
		}
	}
	if statusEnvio != StatusPendente {
		if err := s.store.Q.UpdateStatusCorrecao(ctx, gen.UpdateStatusCorrecaoParams{
			ID: row.ID, Status: statusEnvio,
		}); err != nil {
			return nil, err
		}
	}
	return s.montar(ctx, submissaoID)
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
	ct, err := s.store.Q.GetCorrecaoTarefa(ctx, correcaoTarefaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return jaNaoEncontrada()
		}
		return err
	}
	status := judge0.StatusVeredito(r.StatusID())
	if err := s.store.Q.AplicarResultado(ctx, gen.AplicarResultadoParams{
		CorrecaoTarefaID: correcaoTarefaID,
		TesteID:          testeID,
		Status:           &status,
		Stdout:           textoL(&r.Stdout), Stderr: textoL(&r.Stderr),
		CompileOutput: textoL(&r.CompileOutput),
		Tempo:         textoL(&r.Time), Memoria: textoL(&r.Memory), Cpu: textoL(&r.CPUTime),
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
	return VisaoProfessor{
		Submissao: SubmissaoResumo{
			ID:         row.ID,
			Aluno:      row.AlunoFirstName + " " + row.AlunoLastName,
			AlunoEmail: row.AlunoEmail, AtividadeNome: row.AtividadeNome,
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
	for _, t := range cad.Tarefas {
		if err := s.store.Q.AjustarCorrecaoTarefa(ctx, gen.AjustarCorrecaoTarefaParams{
			ID: t.ID, NotaFinal: notaArgumento(t.NotaFinal), Feedback: textoL(t.Feedback),
		}); err != nil {
			return VisaoProfessor{}, err
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
	visao := VisaoAluno{}
	if cor != nil && cor.PublicadaEm != nil {
		visao.Publicada = true
		visao.PublicadaEm = cor.PublicadaEm
		visao.Correcao = cor
	}
	return visao, nil
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

func notaArgumento(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}
