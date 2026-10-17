// Visões do aluno autenticado (PROJETO §Portal Aluno): turmas com
// matrícula ativa e as atribuições destas turmas, incluindo o estado do
// rascunho (tentativa). O cargo student é exigido nas listagens.
package ensino

import (
	"context"
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
	PublicadaEm   *time.Time `json:"publicada_em,omitempty"`
	NotaFinal     *int32     `json:"nota_final,omitempty"`
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

// ColegaTurma representa outro aluno matriculado na mesma turma (apenas nome, conforme LGPD).
type ColegaTurma struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// DetalhesTurmaAluno é a visão completa de uma turma pelo aluno:
// metadados da turma, professor, colegas e suas atribuições com status.
type DetalhesTurmaAluno struct {
	Turma       TurmaDoAluno        `json:"turma"`
	Colegas     []ColegaTurma       `json:"colegas"`
	Atribuicoes []AtribuicaoDoAluno `json:"atribuicoes"`
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

// notaPtrMeu: nota só existe quando a correção foi publicada; o valor
// chega como texto pelo LATERAL (NULL → nil).
func notaPtrMeu(valor any, pub pgtype.Timestamptz) *int32 {
	if !pub.Valid {
		return nil
	}
	texto, ok := valor.(string)
	if !ok {
		return nil
	}
	var n int
	if _, err := fmt.Sscan(texto, &n); err != nil {
		return nil
	}
	out := int32(n)
	return &out
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

// ObterTurmaAluno: detalhes de uma turma para o aluno autenticado (turma, colegas, atribuições).
func (s *Service) ObterTurmaAluno(
	ctx context.Context, actor domain.User, turmaID int64,
) (*DetalhesTurmaAluno, error) {
	if err := exigirStudent(actor); err != nil {
		return nil, err
	}
	row, err := s.store.Q.GetTurmaAluno(ctx, gen.GetTurmaAlunoParams{
		AlunoID: actor.ID,
		TurmaID: turmaID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Turma não encontrada ou você não possui matrícula ativa nesta turma.")
		}
		return nil, err
	}

	turma := TurmaDoAluno{
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
	}

	colegasRows, err := s.store.Q.ListColegasTurma(ctx, turmaID)
	if err != nil {
		return nil, err
	}
	colegas := make([]ColegaTurma, 0, len(colegasRows))
	for _, c := range colegasRows {
		colegas = append(colegas, ColegaTurma{
			FirstName: c.FirstName,
			LastName:  c.LastName,
		})
	}

	atribuicoes, err := s.MinhasAtribuicoes(ctx, actor, &turmaID)
	if err != nil {
		return nil, err
	}

	return &DetalhesTurmaAluno{
		Turma:       turma,
		Colegas:     colegas,
		Atribuicoes: atribuicoes,
	}, nil
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
			PublicadaEm:   horaPtr(row.PublicadaEm),
			NotaFinal:     notaPtrMeu(row.MinhaNotaTexto, row.PublicadaEm),
		})
	}
	aplicaEstadoMonitor(ctx, s.store, actor.ID, out)
	return out, nil
}

// aplicaEstadoMonitor sobrepõe o estado do rascunho local com o estado
// do Monitor (Fase 9.3) quando o aluno foi trabalhar lá (metadados de
// exibição; a nota continua saindo do fluxo de correção publicada da API —
// a visibilidade "publicada" não é derivada do snapshot do Monitor).
func aplicaEstadoMonitor(ctx context.Context, st *store.Store, alunoID int64, itens []AtribuicaoDoAluno) {
	if len(itens) == 0 {
		return
	}
	type Estado struct {
		AtribuicaoID  int64
		Estado        string
		UltimaEntrega pgtype.Timestamptz
	}
	rows, err := st.Pool.Query(ctx, `
		SELECT atribuicao_id, estado, entregue_em
		FROM monitor_estado_aluno
		WHERE aluno_id = $1`, alunoID)
	if err != nil {
		return // sem estado do Monitor: mantém o estado local
	}
	defer rows.Close()
	pair := map[int64]Estado{}
	for rows.Next() {
		var est Estado
		if err := rows.Scan(&est.AtribuicaoID, &est.Estado, &est.UltimaEntrega); err != nil {
			return
		}
		pair[est.AtribuicaoID] = est
	}
	for i := range itens {
		est, ok := pair[itens[i].ID]
		if !ok {
			continue
		}
		if est.Estado == "aberta" {
			itens[i].EmAndamento = true
		}
		if est.Estado == "entregue" {
			itens[i].Entregue = true
			if est.UltimaEntrega.Valid {
				itens[i].EntregueEm = horaPtr(est.UltimaEntrega)
			}
		}
	}
}
