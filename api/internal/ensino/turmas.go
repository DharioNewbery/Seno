// Package ensino — turmas e matrículas. Regras de escopo (ARQUITETURA
// §5.4): professor só vê/opera as próprias turmas (404 fora do escopo);
// admin/super operam sobre qualquer turma. Criação é do professor (dono,
// PROJETO §Fluxo Completo). Matrícula segue §Matrícula: aluno inexistente
// nasce pendente com convite; "remover aluno" encerra (saída em).
package ensino

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
)

// CadastroTurma é o corpo da criação de turma (professor dono).
// O período letivo é escolhido entre os cadastrados pelo admin.
type CadastroTurma struct {
	MateriaID int64  `json:"materia_id"`
	PeriodoID int64  `json:"periodo_id"`
	Titulo    string `json:"titulo"`
}

// EditarTurma é o corpo da edição de turma.
type EditarTurma struct {
	Titulo *string `json:"titulo"`
}

// FiltroTurmas parametriza a listagem (professor é forçado ao próprio id).
type FiltroTurmas struct {
	ProfessorID *int64
	MateriaID   *int64
	PeriodoID   *int64
	Encerradas  string // "" (só ativas), "only" ou "all"
	Pagina      *int
	Por         *int
}

func tituloPtr(t string) *string {
	if t == "" {
		return nil
	}
	return &t
}

// encerradasOpcional: "" vira NULL ( фильт de só ativas no SQL).
func encerradasOpcional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// int8OrNull converte id opcional em pgtype.Int8 (nil → NULL).
func int8OrNull(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}

func turmaDe(row gen.ListTurmasRow) Turma {
	var encerrado *time.Time
	if row.EncerradoEm.Valid {
		t := row.EncerradoEm.Time
		encerrado = &t
	}
	titulo := ""
	if row.Titulo != nil {
		titulo = *row.Titulo
	}
	return Turma{
		ID: row.ID,
		Materia: Materia{
			ID:     row.MateriaID,
			Codigo: row.MateriaCodigo,
			Nome:   row.MateriaNome,
		},
		Trimestre: row.PeriodoTrimestre,
		Ano:       row.PeriodoAno,
		Titulo:    titulo,
		Professor: Professor{
			ID:        row.ProfessorID,
			Email:     row.ProfEmail,
			FirstName: row.ProfFirstName,
			LastName:  row.ProfLastName,
		},
		EncerradoEm: encerrado,
		CreatedAt:   row.CreatedAt.Time,
	}
}

func turmaDeGet(row gen.GetTurmaRow) Turma {
	return turmaDe(gen.ListTurmasRow{
		ID:               row.ID,
		MateriaID:        row.MateriaID,
		PeriodoID:        row.PeriodoID,
		ProfessorID:      row.ProfessorID,
		Titulo:           row.Titulo,
		EncerradoEm:      row.EncerradoEm,
		CreatedAt:        row.CreatedAt,
		MateriaCodigo:    row.MateriaCodigo,
		MateriaNome:      row.MateriaNome,
		PeriodoTrimestre: row.PeriodoTrimestre,
		PeriodoAno:       row.PeriodoAno,
		ProfFirstName:    row.ProfFirstName,
		ProfLastName:     row.ProfLastName,
		ProfEmail:        row.ProfEmail,
	})
}

// CriaTurma registra a turma do professor logado (dono = ator). Apenas
// usuários com cargo de professor criam turmas (admin puro não é dono).
func (s *Service) CriaTurma(ctx context.Context, actor domain.User, cad CadastroTurma) (Turma, error) {
	if !domain.HasAnyRole(actor.Roles, domain.RoleProfessor) {
		return Turma{}, platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Apenas professores criam turmas.")
	}
	materia, err := s.store.Q.GetMateria(ctx, cad.MateriaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Turma{}, materiaNaoEncontrada()
		}
		return Turma{}, err
	}
	// O período deve existir: escolhido entre os cadastrados pelo admin.
	if _, err := s.store.Q.GetPeriodo(ctx, cad.PeriodoID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Turma{}, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"Período letivo inválido: escolha um período cadastrado.")
		}
		return Turma{}, err
	}
	titulo := strings.TrimSpace(cad.Titulo)

	var turmaID int64
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		nova, err := q.InsertTurma(ctx, gen.InsertTurmaParams{
			MateriaID:   cad.MateriaID,
			PeriodoID:   cad.PeriodoID,
			ProfessorID: actor.ID,
			Titulo:      tituloPtr(titulo),
			CreatedBy:   int8Ptr(actor.ID),
		})
		if err != nil {
			return err
		}
		turmaID = nova.ID
		return nil
	})
	if err != nil {
		return Turma{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID,
		Actor:   actor.Email,
		Kind:    LogKindTurmaCreate,
		Detail:  fmt.Sprintf("turma=%d materia=%s", turmaID, materia.Codigo),
	})
	return s.VerTurma(ctx, actor, turmaID)
}

// ListarTurmas: professor recebe APENAS as próprias (filtro forçado);
// admin/super recebem tudo, com filtros opcionais.
func (s *Service) ListarTurmas(ctx context.Context, actor domain.User, f FiltroTurmas) (ListaTurmas, error) {
	prof := f.ProfessorID
	if !actor.IsStaff() {
		pro := actor.ID
		prof = &pro
	}
	pagina, por := paginaPadrao(f.Pagina, f.Por)
	if f.Encerradas != "" && f.Encerradas != "only" && f.Encerradas != "all" {
		return ListaTurmas{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Filtro encerradas inválido (use only ou all).")
	}

	total, err := s.store.Q.CountTurmas(ctx, gen.CountTurmasParams{
		Professor:  int8OrNull(prof),
		Materia:    int8OrNull(f.MateriaID),
		Periodo:    int8OrNull(f.PeriodoID),
		Encerradas: encerradasOpcional(f.Encerradas),
	})
	if err != nil {
		return ListaTurmas{}, err
	}
	rows, err := s.store.Q.ListTurmas(ctx, gen.ListTurmasParams{
		Professor:  int8OrNull(prof),
		Materia:    int8OrNull(f.MateriaID),
		Periodo:    int8OrNull(f.PeriodoID),
		Encerradas: encerradasOpcional(f.Encerradas),
		Limit:      int32(por),
		Offset:     int32((pagina - 1) * por),
	})
	if err != nil {
		return ListaTurmas{}, err
	}
	out := make([]Turma, 0, len(rows))
	for _, row := range rows {
		out = append(out, turmaDe(row))
	}
	return ListaTurmas{
		Total: total, Pagina: pagina, Por: por, Turmas: out,
	}, nil
}

// VerTurma devolve uma turma (404 para inexistente ou de outro professor).
func (s *Service) VerTurma(ctx context.Context, actor domain.User, id int64) (Turma, error) {
	row, err := s.store.Q.GetTurma(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Turma{}, turmaNaoEncontrada()
		}
		return Turma{}, err
	}
	if err := escopoTurma(actor, row.ProfessorID); err != nil {
		return Turma{}, err
	}
	return turmaDeGet(row), nil
}

// EditarTurma muda o título; dono ou staff; turma encerrada → 409.
func (s *Service) EditarTurma(ctx context.Context, actor domain.User, id int64, cad EditarTurma) (Turma, error) {
	roda, err := s.VerTurma(ctx, actor, id)
	if err != nil {
		return Turma{}, err
	}
	if roda.EncerradoEm != nil {
		return Turma{}, platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict, "Turma encerrada.")
	}
	var titulo *string
	if cad.Titulo != nil {
		t := strings.TrimSpace(*cad.Titulo)
		titulo = tituloPtr(t)
	}
	if err := s.store.Q.UpdateTurma(ctx, gen.UpdateTurmaParams{
		ID: id, Titulo: tituloPtr(valorOuVazio(titulo)),
	}); err != nil {
		return Turma{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTurmaUpdate,
		Detail: fmt.Sprintf("turma=%d", id),
	})
	return s.VerTurma(ctx, actor, id)
}

func valorOuVazio(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// EncerrarTurma marca o fim da turma (encerrado_em), idempotente pela
// query; já encerrada → 409.
func (s *Service) EncerrarTurma(ctx context.Context, actor domain.User, id int64) error {
	roda, err := s.VerTurma(ctx, actor, id)
	if err != nil {
		return err
	}
	if roda.EncerradoEm != nil {
		return platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict, "Turma já encerrada.")
	}
	if _, err := s.store.Q.EncerrarTurma(ctx, id); err != nil {
		return err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTurmaUpdate,
		Detail: fmt.Sprintf("turma=%d encerrada", id),
	})
	return nil
}

// ExcluirTurma apaga a turma; com matrículas ativas → 409 (histórico).
func (s *Service) ExcluirTurma(ctx context.Context, actor domain.User, id int64) error {
	if _, err := s.VerTurma(ctx, actor, id); err != nil {
		return err
	}
	ativas, err := s.store.Q.CountMatriculasAtivasTurma(ctx, id)
	if err != nil {
		return err
	}
	if ativas > 0 {
		return platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict,
			"Turma com matrículas ativas — encerre os alunos antes.")
	}
	rows, err := s.store.Q.DeleteTurma(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return turmaNaoEncontrada()
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTurmaDelete,
		Detail: fmt.Sprintf("turma=%d", id),
	})
	return nil
}

// CadastroMatricula é o corpo da matrícula (aluno por e-mail).
type CadastroMatricula struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// Matricular: aluno já existe (ativo) → matrícula direta; pendente →
// matrícula + garante cargo de aluno; inexistente → cria pendente+convite.
func (s *Service) Matricular(ctx context.Context, actor domain.User, turmaID int64, cad CadastroMatricula) (Matricula, error) {
	turma, err := s.VerTurma(ctx, actor, turmaID)
	if err != nil {
		return Matricula{}, err
	}
	if turma.EncerradoEm != nil {
		return Matricula{}, platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict, "Turma encerrada.")
	}

	email := strings.ToLower(strings.TrimSpace(cad.Email))
	if email == "" {
		return Matricula{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe o e-mail do aluno.")
	}
	row, err := s.store.Q.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		if row.Status == domain.StatusDisabled || row.DisabledAt.Valid {
			return Matricula{}, platform.NewAPIError(
				http.StatusForbidden, platform.CodeForbidden, "Conta bloqueada.")
		}
		if row.Status == domain.StatusActive {
			roles, err := s.store.Q.GetUserRoles(ctx, row.ID)
			if err != nil {
				return Matricula{}, err
			}
			if !domain.HasAnyRole(rolesToSlice(roles), domain.RoleStudent) {
				return Matricula{}, platform.NewAPIError(
					http.StatusUnprocessableEntity, platform.CodeUnprocessable,
					"Usuário sem cargo de aluno.")
			}
		}
	case errors.Is(err, pgx.ErrNoRows):
		if strings.TrimSpace(cad.FirstName) == "" || strings.TrimSpace(cad.LastName) == "" {
			return Matricula{}, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"Aluno novo: informe nome e sobrenome.")
		}
		if err := s.recoverer.Invite(
			ctx, email, cad.FirstName, cad.LastName, &actor.ID,
		); err != nil {
			return Matricula{}, err
		}
		row, err = s.store.Q.GetUserByEmail(ctx, email)
		if err != nil {
			return Matricula{}, err
		}
	default:
		return Matricula{}, err
	}

	// Cargo de student presente para pendentes novos (e re-matrícula).
	roles, err := s.store.Q.GetUserRoles(ctx, row.ID)
	if err != nil {
		return Matricula{}, err
	}
	if !domain.HasAnyRole(rolesToSlice(roles), domain.RoleStudent) {
		if err := s.store.Q.InsertUserRole(ctx, gen.InsertUserRoleParams{
			UserID: row.ID, Role: string(domain.RoleStudent),
		}); err != nil {
			return Matricula{}, err
		}
	}

	mat, err := s.store.Q.InsertMatricula(ctx, gen.InsertMatriculaParams{
		TurmaID: turmaID, AlunoID: row.ID, CreatedBy: int8Ptr(actor.ID),
	})
	if err != nil {
		if eDuplicado(err) {
			return Matricula{}, platform.NewAPIError(
				http.StatusConflict, platform.CodeConflict,
				"Aluno já matriculado nesta turma.")
		}
		return Matricula{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID,
		Actor:   actor.Email,
		Kind:    LogKindMatriculaNew,
		Detail:  fmt.Sprintf("turma=%d aluno=%d", turmaID, row.ID),
	})
	return Matricula{
		ID:        mat.ID,
		Aluno:     Aluno{ID: row.ID, Email: row.Email, FirstName: row.FirstName, LastName: row.LastName, Status: row.Status},
		EntradaEm: mat.EntradaEm.Time,
	}, nil
}

// ListarMatriculas: os alunos da turma (dono ou staff).
func (s *Service) ListarMatriculas(ctx context.Context, actor domain.User, turmaID int64, incluirEncerradas bool) (ListaMatriculas, error) {
	if _, err := s.VerTurma(ctx, actor, turmaID); err != nil {
		return ListaMatriculas{}, err
	}
	rows, err := s.store.Q.ListMatriculas(ctx, gen.ListMatriculasParams{
		TurmaID: turmaID,
		Column2: incluirEncerradas,
	})
	if err != nil {
		return ListaMatriculas{}, err
	}
	mats := make([]Matricula, 0, len(rows))
	for _, row := range rows {
		var saida *time.Time
		if row.SaidaEm.Valid {
			t := row.SaidaEm.Time
			saida = &t
		}
		mats = append(mats, Matricula{
			ID: row.ID,
			Aluno: Aluno{
				ID:        row.AlunoID,
				Email:     row.AlunoEmail,
				FirstName: row.AlunoFirstName,
				LastName:  row.AlunoLastName,
				Status:    row.AlunoStatus,
			},
			EntradaEm: row.EntradaEm.Time,
			SaidaEm:   saida,
		})
	}
	return ListaMatriculas{Total: int64(len(mats)), Matriculas: mats}, nil
}

// EncerrarMatricula encerra a matrícula ("remover aluno" — saída em).
func (s *Service) EncerrarMatricula(ctx context.Context, actor domain.User, matriculaID int64) error {
	row, err := s.store.Q.GetMatricula(ctx, matriculaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return matriculaNaoEncontrada()
		}
		return err
	}
	if err := escopoTurma(actor, row.TurmaProfessorID); err != nil {
		return err
	}
	if _, err := s.store.Q.EncerrarMatricula(ctx, matriculaID); err != nil {
		return err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindMatriculaExit,
		Detail: fmt.Sprintf("matricula=%d turma=%d", matriculaID, row.TurmaID),
	})
	return nil
}

func rolesToSlice(in []string) []domain.Role {
	out := make([]domain.Role, 0, len(in))
	for _, r := range in {
		out = append(out, domain.Role(r))
	}
	return out
}
