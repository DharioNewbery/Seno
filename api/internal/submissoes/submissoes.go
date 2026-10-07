// Package submissoes — entrega final da atividade pelo aluno (PROJETO
// §Submissão). Regras do §Atribuição aplicadas no servidor: prazo
// efetivo = min(comecou_em + duração, prazo); entrega fora dele só é
// aceita quando "pode ser entregue atrasado" (marcada como atrasada).
// Requere Tentativa aberta (rascunho). A Tentativa é apagada entre as
// gravações; a Submissão é única por (aluno, atribuição) — submissão
// dupla → 409. Visão do professor: entregas da atribuição (dono|staff).
package submissoes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// Service coordena as submissões.
type Service struct {
	store *store.Store
	audit *platform.Audit
}

func New(st *store.Store, audit *platform.Audit) *Service {
	return &Service{store: st, audit: audit}
}

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindSubmissao = "submissao.create"
)

// Submissao é a visão completa de uma entrega.
type Submissao struct {
	ID           int64           `json:"id"`
	AtribuicaoID int64           `json:"atribuicao_id"`
	AlunoID      int64           `json:"aluno_id"`
	ComecouEm    time.Time       `json:"comecou_em"`
	EntregueEm   time.Time       `json:"entregue_em"`
	Atrasada     bool            `json:"atrasada"`
	Snapshot     json.RawMessage `json:"snapshot"`
}

// Entrega é a visão do professor (aluno + tempo + estado).
type Entrega struct {
	ID         int64     `json:"id"`
	Aluno      string    `json:"aluno"`
	AlunoEmail string    `json:"aluno_email"`
	ComecouEm  time.Time `json:"comecou_em"`
	EntregueEm time.Time `json:"entregue_em"`
	Atrasada   bool      `json:"atrasada"`
}

func indisponivel(msg string) error {
	return platform.NewAPIError(
		http.StatusUnprocessableEntity, platform.CodeUnprocessable, msg)
}

func eDuplicado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// prazoEfetivo = min(comecou_em + duração, prazo), §Atribuição.
func prazoEfetivo(comecouEm time.Time, duracaoSeg pgtype.Int4, prazo pgtype.Timestamptz) pgtype.Timestamptz {
	if duracaoSeg.Valid {
		fimDuracao := comecouEm.Add(time.Duration(duracaoSeg.Int32) * time.Second)
		if !prazo.Valid || fimDuracao.Before(prazo.Time) {
			return pgtype.Timestamptz{Time: fimDuracao, Valid: true}
		}
	}
	return prazo
}

// Entregar executa o envio final do aluno: valida rascunho/janela,
// grava a submissão e apaga a Tentativa em transação.
func (s *Service) Entregar(
	ctx context.Context, actor domain.User, atribuicaoID int64, observacao string,
) (Submissao, error) {
	if !domain.HasAnyRole(actor.Roles, domain.RoleStudent) {
		return Submissao{}, platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Operação exclusiva de alunos.")
	}
	atr, err := s.store.Q.GetAtribuicao(ctx, atribuicaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submissao{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Atribuição não encontrada.")
		}
		return Submissao{}, err
	}
	mat, err := s.store.Q.MatriculaAtiva(ctx, gen.MatriculaAtivaParams{
		TurmaID: atr.TurmaID, AlunoID: actor.ID,
	})
	if err != nil {
		return Submissao{}, err
	}
	if !mat {
		return Submissao{}, platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound,
			"Atribuição não encontrada.")
	}

	// Já entregue → 409 (a Tentativa já foi apagada na primeira entrega).
	_, err = s.store.Q.GetSubmissaoAluno(ctx, gen.GetSubmissaoAlunoParams{
		AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
	})
	if err == nil {
		return Submissao{}, platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict,
			"Esta atividade já foi entregue.")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Submissao{}, err
	}
	turma, err := s.store.Q.GetTurma(ctx, atr.TurmaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submissao{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Atribuição não encontrada.")
		}
		return Submissao{}, err
	}

	// Tentativa: exige rascunho aberto (o começo da prova).
	atual, err := s.store.Q.GetTentativa(ctx, gen.GetTentativaParams{
		AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Submissao{}, indisponivel(
			"Abra a atividade antes de entregar.")
	case err != nil:
		return Submissao{}, err
	}
	comecouEm := atual.ComecouEm.Time

	// Janela (§Atribuição): início futuro; turma encerrada; prazo
	// efetivo passado exige atraso permitido (submissão marcada).
	agora := time.Now()
	if atr.Inicio.Valid && agora.Before(atr.Inicio.Time) {
		return Submissao{}, indisponivel("A atividade ainda não começou.")
	}
	if turma.EncerradoEm.Valid {
		return Submissao{}, indisponivel("A turma está encerrada.")
	}
	prazo := prazoEfetivo(comecouEm, atr.DuracaoSeg, atr.Prazo)
	atrasada := false
	if prazo.Valid && agora.After(prazo.Time) {
		if !atr.PodeAtrasado {
			return Submissao{}, indisponivel(
				"Prazo encerrado (entrega atrasada não permitida).")
		}
		atrasada = true
	}

	observacao = truncObservacao(observacao)
	var nova Submissao
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		row, err := q.InsertSubmissao(ctx, gen.InsertSubmissaoParams{
			AtribuicaoID: atribuicaoID,
			AlunoID:      actor.ID,
			ComecouEm:    pgtype.Timestamptz{Time: comecouEm, Valid: true},
			Atrasada:     atrasada,
			Observacao:   textoPtr(observacao),
			Snapshot:     atual.Snapshot,
			CreatedBy:    int8Ptr(actor.ID),
		})
		if err != nil {
			return err
		}
		if _, err := q.ApagarTentativa(ctx, gen.ApagarTentativaParams{
			AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
		}); err != nil {
			return err
		}
		nova = Submissao{
			ID: row.ID, AtribuicaoID: row.AtribuicaoID, AlunoID: row.AlunoID,
			ComecouEm: row.ComecouEm.Time, EntregueEm: row.EntregueEm.Time,
			Atrasada: row.Atrasada, Snapshot: row.Snapshot,
		}
		return nil
	})
	if err != nil {
		if eDuplicado(err) {
			return Submissao{}, platform.NewAPIError(
				http.StatusConflict, platform.CodeConflict,
				"Esta atividade já foi entregue.")
		}
		return Submissao{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindSubmissao,
		Detail: fmt.Sprintf("atribuicao=%d atrasada=%v", atribuicaoID, atrasada),
	})
	return nova, nil
}

// Listar: entregas de uma atribuição (dono da turma|staff).
func (s *Service) Listar(
	ctx context.Context, actor domain.User, atribuicaoID int64,
) ([]Entrega, error) {
	atr, err := s.store.Q.GetAtribuicao(ctx, atribuicaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Atribuição não encontrada.")
		}
		return nil, err
	}
	if !actor.IsStaff() && actor.ID != atr.TurmaProfessorID {
		return nil, platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound,
			"Atribuição não encontrada.")
	}
	rows, err := s.store.Q.ListSubmissoesAtribuicao(ctx, atribuicaoID)
	if err != nil {
		return nil, err
	}
	out := make([]Entrega, 0, len(rows))
	for _, row := range rows {
		out = append(out, Entrega{
			ID:         row.ID,
			Aluno:      row.AlunoFirstName + " " + row.AlunoLastName,
			AlunoEmail: row.AlunoEmail,
			ComecouEm:  horaDe(row.ComecouEm),
			EntregueEm: horaDe(row.EntregueEm),
			Atrasada:   row.Atrasada,
		})
	}
	return out, nil
}

// helpers -----------------------------------------------------------

func int8Ptr(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

func horaDe(ts pgtype.Timestamptz) time.Time { return ts.Time }

func textoPtr(t string) *string {
	if t == "" {
		return nil
	}
	return &t
}

// truncObservacao: limite plausível para a observação livre do aluno.
func truncObservacao(t string) string {
	r := []rune(t)
	if len(r) > 1024 {
		return string(r[:1024])
	}
	return t
}
