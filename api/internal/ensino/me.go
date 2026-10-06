// Visões do aluno autenticado (PROJETO §Portal Aluno): turmas com
// matrícula ativa e as atribuições destas turmas, incluindo o estado do
// rascunho (tentativa). O cargo student é exigido nas listagens.
package ensino

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
)

// AtribuicaoDoAluno é a atribuição na visão do aluno, com o estado do
// rascunho (sem submissões ainda, "em andamento" = tentativa aberta).
type AtribuicaoDoAluno struct {
	ID            int64      `json:"id"`
	TurmaID       int64      `json:"turma_id"`
	AtividadeID   int64      `json:"atividade_id"`
	Autocomplete  bool       `json:"autocomplete"`
	Inicio        *time.Time `json:"inicio,omitempty"`
	Prazo         *time.Time `json:"prazo,omitempty"`
	DuracaoSeg    *int32     `json:"duracao_seg,omitempty"`
	PodeAtrasado  bool       `json:"pode_atrasado"`
	MateriaCodigo string     `json:"materia_codigo"`
	TurmaTitulo   string     `json:"turma_titulo,omitempty"`
	TurmaAtiva    bool       `json:"turma_ativa"`
	AtividadeNome string     `json:"atividade_nome"`
	EmAndamento   bool       `json:"em_andamento"`
	MinhaRevisao  int32      `json:"minha_revisao,omitempty"`
	GravadoEm     *time.Time `json:"gravado_em,omitempty"`
	Entregue      bool       `json:"entregue"`
	EntregueEm    *time.Time `json:"entregue_em,omitempty"`
	Atrasada      bool       `json:"atrasada"`
}
// TurmaDoAluno é a turma na visão do aluno.
type TurmaDoAluno struct {
	ID            int64     `json:"id"`
	Titulo        string    `json:"titulo,omitempty"`
	MateriaCodigo string    `json:"materia_codigo"`
	MateriaNome   string    `json:"materia_nome"`
	Trimestre     int16     `json:"trimestre"`
	Ano           int16     `json:"ano"`
	Professor     string    `json:"professor"`
	ProfessorMail string    `json:"professor_email"`
	Ativa         bool      `json:"ativa"`
	CreatedAt     time.Time `json:"created_at"`
}

// exigirStudent: cargo student; professores/admins não usam o portal.
func exigirStudent(actor domain.User) error {
	if !domain.HasAnyRole(actor.Roles, domain.RoleStudent) {
		return platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Operação exclusiva de alunos.")
	}
	return nil
}

func intSegPtr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	out := v.Int32
	return &out
}

func tituloDe(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func horaPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// MinhasTurmas: turmas com matrícula ativa do aluno.
func (s *Service) MinhasTurmas(
	ctx context.Context, actor domain.User,
) ([]TurmaDoAluno, error) {
	if err := exigirStudent(actor); err != nil {
		return nil, err
	}
	rows, err := s.store.Q.ListTurmasAluno(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	out := make([]TurmaDoAluno, 0, len(rows))
	for _, row := range rows {
		out = append(out, TurmaDoAluno{
			ID:            row.ID,
			Titulo:        tituloDe(row.Titulo),
			MateriaCodigo: row.MateriaCodigo,
			MateriaNome:   row.MateriaNome,
			Trimestre:     row.Trimestre,
			Ano:           row.Ano,
			Professor:     row.ProfFirstName + " " + row.ProfLastName,
			ProfessorMail: row.ProfEmail,
			Ativa:         !row.EncerradoEm.Valid,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return out, nil
}

// MinhasAtribuicoes: atribuições das turmas com matrícula ativa
// (filtro opcional por turma), com o estado do rascunho.
func (s *Service) MinhasAtribuicoes(
	ctx context.Context, actor domain.User, turmaID *int64,
) ([]AtribuicaoDoAluno, error) {
	if err := exigirStudent(actor); err != nil {
		return nil, err
	}
	filtro := pgtype.Int8{}
	if turmaID != nil && *turmaID > 0 {
		filtro = pgtype.Int8{Int64: *turmaID, Valid: true}
	}
	rows, err := s.store.Q.ListAtribuicoesAluno(
		ctx, gen.ListAtribuicoesAlunoParams{AlunoID: actor.ID, Turma: filtro},
	)
	if err != nil {
		return nil, err
	}
	out := make([]AtribuicaoDoAluno, 0, len(rows))
	for _, row := range rows {
		out = append(out, AtribuicaoDoAluno{
			ID:            row.ID,
			TurmaID:       row.TurmaID,
			AtividadeID:   row.AtividadeID,
			Autocomplete:  row.Autocomplete,
			Inicio:        horaPtr(row.Inicio),
			Prazo:         horaPtr(row.Prazo),
			DuracaoSeg:    intSegPtr(row.DuracaoSeg),
			PodeAtrasado:  row.PodeAtrasado,
			MateriaCodigo: row.MateriaCodigo,
			TurmaTitulo:   tituloDe(row.TurmaTitulo),
			TurmaAtiva:    !row.TurmaEncerradoEm.Valid,
			AtividadeNome: row.AtividadeNome,
			EmAndamento:   row.MinhaRevisao.Valid,
			MinhaRevisao:  row.MinhaRevisao.Int32,
			GravadoEm:     horaPtr(row.MeuGravadoEm),
			Entregue:      row.MinhaEntrega.Valid,
			EntregueEm:    horaPtr(row.MinhaEntrega),
			Atrasada:      row.MinhaAtrasada.Bool,
		})
	}
	return out, nil
}
