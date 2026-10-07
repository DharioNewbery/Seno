// Package tentativas — rascunho da atividade para o aluno (PROJETO
// §Tentativa): única por (aluno, atribuição), `comecou_em` do primeiro
// acesso, revisão incrementada a cada gravação de autosave — gravação
// com revisão antiga é rejeitada (duas abas). Excluída quando a
// Submissão é feita (regra do §, no futuro). Disponibilidade segue
// §Atribuição: início futuro e turma encerrada bloqueiam; prazo passado
// bloqueia quando a entrega atrasada não é permitida.
package tentativas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// Service coordena as tentativas.
type Service struct {
	store *store.Store
	audit *platform.Audit
}

func New(st *store.Store, audit *platform.Audit) *Service {
	return &Service{store: st, audit: audit}
}

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindTentativaAbrir  = "tentativa.abrir"
	LogKindTentativaGravar = "tentativa.gravar"
)

// Tentativa é a visão do rascunho do aluno.
type Tentativa struct {
	ID           int64           `json:"id"`
	AtribuicaoID int64           `json:"atribuicao_id"`
	AlunoID      int64           `json:"aluno_id"`
	ComecouEm    time.Time       `json:"comecou_em"`
	Revisao      int32           `json:"revisao"`
	GravadoEm    time.Time       `json:"gravado_em"`
	Snapshot     json.RawMessage `json:"snapshot"`
}

// CadastroGravacao é o corpo do PUT (autosave). `revisao` deve ser a
// corrente do rascunho; o servidor incrementa. A 1ª gravação usa 1.
type CadastroGravacao struct {
	Revisao  int32           `json:"revisao"`
	Snapshot json.RawMessage `json:"snapshot"`
}

func tentaDeGet(row gen.GetTentativaRow) Tentativa {
	return Tentativa{
		ID: row.ID, AtribuicaoID: row.AtribuicaoID, AlunoID: row.AlunoID,
		ComecouEm: horaDe(row.ComecouEm), Revisao: row.Revisao,
		GravadoEm: horaDe(row.GravadoEm), Snapshot: row.Snapshot,
	}
}

func tentaDeInsert(row gen.InsertTentativaRow) Tentativa {
	return Tentativa{
		ID: row.ID, AtribuicaoID: row.AtribuicaoID, AlunoID: row.AlunoID,
		ComecouEm: horaDe(row.ComecouEm), Revisao: row.Revisao,
		GravadoEm: horaDe(row.GravadoEm), Snapshot: row.Snapshot,
	}
}

func tentaDeGravar(row gen.GravarTentativaRow) Tentativa {
	return Tentativa{
		ID: row.ID, AtribuicaoID: row.AtribuicaoID, AlunoID: row.AlunoID,
		ComecouEm: horaDe(row.ComecouEm), Revisao: row.Revisao,
		GravadoEm: horaDe(row.GravadoEm), Snapshot: row.Snapshot,
	}
}

func tentativaNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Tentativa não encontrada.")
}

func horaDe(ts pgtype.Timestamptz) time.Time { return ts.Time }

func int8Ptr(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

func conflitoRevisao() error {
	return platform.NewAPIError(
		http.StatusConflict, platform.CodeConflict,
		"Outra aba gravou neste meio-tempo — recarregue o rascunho.")
}

func indisponivel(msg string) error {
	return platform.NewAPIError(
		http.StatusUnprocessableEntity, platform.CodeUnprocessable, msg)
}

// prazoEfetivo = min(comecou_em + duração, prazo), §Atribuição (com
// duração ausente, devolve o próprio prazo).
func prazoEfetivo(comecouEm time.Time, duracaoSeg pgtype.Int4, prazo pgtype.Timestamptz) pgtype.Timestamptz {
	if duracaoSeg.Valid {
		fimDuracao := comecouEm.Add(time.Duration(duracaoSeg.Int32) * time.Second)
		if !prazo.Valid || fimDuracao.Before(prazo.Time) {
			return pgtype.Timestamptz{Time: fimDuracao, Valid: true}
		}
	}
	return prazo
}

// contexto resolve a atribuição, a turma e a matrícula ativa do aluno.
type contexto struct {
	atribuicao gen.GetAtribuicaoRow
	turma      gen.GetTurmaRow
}

func (s *Service) resolver(ctx context.Context, atribuicaoID int64) (contexto, error) {
	atr, err := s.store.Q.GetAtribuicao(ctx, atribuicaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contexto{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Atribuição não encontrada.")
		}
		return contexto{}, err
	}
	turma, err := s.store.Q.GetTurma(ctx, atr.TurmaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contexto{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Atribuição não encontrada.")
		}
		return contexto{}, err
	}
	return contexto{atribuicao: atr, turma: turma}, nil
}

// escopoAluno: precisa de cargo student e matrícula ativa na turma;
// staff não opera rascunho alheio (coleta entra com as submissões).
func (s *Service) escopoAluno(ctx context.Context, actor domain.User, c contexto) error {
	if !domain.HasAnyRole(actor.Roles, domain.RoleStudent) {
		return platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Apenas alunos abrem tentativa.")
	}
	mat, err := s.store.Q.MatriculaAtiva(
		ctx, gen.MatriculaAtivaParams{TurmaID: c.turma.ID, AlunoID: actor.ID})
	if err != nil {
		return err
	}
	if !mat {
		return platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound,
			"Atribuição não encontrada.")
	}
	return nil
}

// janela valida a disponibilidade (§Atribuição): início futuro, turma
// encerrada e prazo efetivo passado sem atraso permitido bloqueiam.
func janela(c contexto, agora time.Time, comecouEm time.Time) error {
	if inicio := c.atribuicao.Inicio; inicio.Valid && agora.Before(inicio.Time) {
		return indisponivel("A atividade ainda não começou.")
	}
	if c.turma.EncerradoEm.Valid {
		return indisponivel("A turma está encerrada.")
	}
	prazo := prazoEfetivo(comecouEm, c.atribuicao.DuracaoSeg, c.atribuicao.Prazo)
	if prazo.Valid && agora.After(prazo.Time) && !c.atribuicao.PodeAtrasado {
		return indisponivel("Prazo encerrado (entrega atrasada não permitida).")
	}
	return nil
}

// validaSnapshot: snapshot é um objeto JSON (rascunho hidratável).
func validaSnapshot(bruto json.RawMessage) (json.RawMessage, error) {
	if len(bruto) == 0 {
		return nil, indisponivel("Snapshot ausente.")
	}
	var mapa map[string]any
	if err := json.Unmarshal(bruto, &mapa); err != nil || mapa == nil {
		return nil, indisponivel("Snapshot deve ser um objeto JSON.")
	}
	return json.RawMessage(bruto), nil
}

// jáSubmeteu: tentativa não volta depois da entrega (§Submissão).
func (s *Service) jaSubmeteu(ctx context.Context, atribuicaoID, alunoID int64) (bool, error) {
	_, err := s.store.Q.GetSubmissaoAluno(ctx, gen.GetSubmissaoAlunoParams{
		AtribuicaoID: atribuicaoID, AlunoID: alunoID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func entregueAntes() error {
	return platform.NewAPIError(
		http.StatusConflict, platform.CodeConflict,
		"Atividade já entregue — sem rascunho.")
}

// Abrir abre (idempotente) a tentativa do aluno nesta atribuição:
// primeira vez cria com comecou_em=agora e revisão 1; senão devolve o
// rascunho atual.
func (s *Service) Abrir(ctx context.Context, actor domain.User, atribuicaoID int64) (Tentativa, error) {
	c, err := s.resolver(ctx, atribuicaoID)
	if err != nil {
		return Tentativa{}, err
	}
	if err := s.escopoAluno(ctx, actor, c); err != nil {
		return Tentativa{}, err
	}
	if entregue, err := s.jaSubmeteu(ctx, atribuicaoID, actor.ID); err != nil {
		return Tentativa{}, err
	} else if entregue {
		return Tentativa{}, entregueAntes()
	}
	if err := janela(c, time.Now(), time.Now()); err != nil {
		return Tentativa{}, err
	}
	row, err := s.store.Q.InsertTentativa(ctx, gen.InsertTentativaParams{
		AtribuicaoID: atribuicaoID,
		AlunoID:      actor.ID,
		Snapshot:     json.RawMessage(`{}`),
		CreatedBy:    int8Ptr(actor.ID),
	})
	switch {
	case err == nil:
		s.audit.Record(ctx, platform.LogEntry{
			ActorID: &actor.ID, Actor: actor.Email,
			Kind:   LogKindTentativaAbrir,
			Detail: fmt.Sprintf("atribuicao=%d tentativa=%d", atribuicaoID, row.ID),
		})
		return tentaDeInsert(row), nil
	case errors.Is(err, pgx.ErrNoRows): // já existe (idempotente)
		atual, err := s.store.Q.GetTentativa(ctx, gen.GetTentativaParams{
			AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
		})
		if err != nil {
			return Tentativa{}, err
		}
		return tentaDeGet(atual), nil
	default:
		return Tentativa{}, err
	}
}

// Ler devolve o rascunho do próprio aluno (404 se ainda não abriu).
func (s *Service) Ler(ctx context.Context, actor domain.User, atribuicaoID int64) (Tentativa, error) {
	c, err := s.resolver(ctx, atribuicaoID)
	if err != nil {
		return Tentativa{}, err
	}
	if err := s.escopoAluno(ctx, actor, c); err != nil {
		return Tentativa{}, err
	}
	row, err := s.store.Q.GetTentativa(ctx, gen.GetTentativaParams{
		AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tentativa{}, tentativaNaoEncontrada()
		}
		return Tentativa{}, err
	}
	return tentaDeGet(row), nil
}

// Gravar recebe o autosave (PUT): a revisão informada deve ser a
// corrente; grava com revisão antiga → 409 (duas abas).
func (s *Service) Gravar(ctx context.Context, actor domain.User, atribuicaoID int64, cad CadastroGravacao) (Tentativa, error) {
	c, err := s.resolver(ctx, atribuicaoID)
	if err != nil {
		return Tentativa{}, err
	}
	if err := s.escopoAluno(ctx, actor, c); err != nil {
		return Tentativa{}, err
	}
	if entregue, err := s.jaSubmeteu(ctx, atribuicaoID, actor.ID); err != nil {
		return Tentativa{}, err
	} else if entregue {
		return Tentativa{}, entregueAntes()
	}
	snapshot, err := validaSnapshot(cad.Snapshot)
	if err != nil {
		return Tentativa{}, err
	}
	if cad.Revisao < 1 {
		return Tentativa{}, indisponivel("Revisão inválida.")
	}

	// Início futuro também bloqueia gravação; turma encerrada idem.
	atual, err := s.store.Q.GetTentativa(ctx, gen.GetTentativaParams{
		AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
	})
	switch {
	case err == nil:
		if err := janela(c, time.Now(), atual.ComecouEm.Time); err != nil {
			return Tentativa{}, err
		}
	case errors.Is(err, pgx.ErrNoRows):
		if err := janela(c, time.Now(), time.Now()); err != nil {
			return Tentativa{}, err
		}
	default:
		return Tentativa{}, err
	}

	if err == nil {
		if cad.Revisao != atual.Revisao {
			return Tentativa{}, conflitoRevisao()
		}
		rows, err := s.store.Q.GravarTentativa(ctx, gen.GravarTentativaParams{
			AtribuicaoID: atribuicaoID,
			AlunoID:      actor.ID,
			Snapshot:     snapshot,
			Revisao:      cad.Revisao,
		})
		if err != nil {
			return Tentativa{}, err
		}
		if len(rows) == 0 {
			return Tentativa{}, conflitoRevisao()
		}
		s.audit.Record(ctx, platform.LogEntry{
			ActorID: &actor.ID, Actor: actor.Email,
			Kind:   LogKindTentativaGravar,
			Detail: fmt.Sprintf("atribuicao=%d revisao=%d", atribuicaoID, rows[0].Revisao),
		})
		return tentaDeGravar(rows[0]), nil
	}

	// Sem tentativa ainda: exige revisão 1 (criação pelo autosave).
	if cad.Revisao != 1 {
		return Tentativa{}, conflitoRevisao()
	}
	row, err := s.store.Q.InsertTentativa(ctx, gen.InsertTentativaParams{
		AtribuicaoID: atribuicaoID,
		AlunoID:      actor.ID,
		Snapshot:     snapshot,
		CreatedBy:    int8Ptr(actor.ID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Corrida com outra aba abrindo agora: revise e grave de novo.
		return Tentativa{}, conflitoRevisao()
	}
	if err != nil {
		return Tentativa{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTentativaGravar,
		Detail: fmt.Sprintf("atribuicao=%d revisao=1", atribuicaoID),
	})
	return tentaDeInsert(row), nil
}
