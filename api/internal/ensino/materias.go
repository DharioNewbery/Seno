// Package ensino — CRUD de matérias (admin/super via rota; leitura válida
// também para professor) e períodos letivos (idempotentes).
package ensino

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
)

// CadastroMateria é o corpo da criação/edição de matéria.
type CadastroMateria struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
}

func materiaDe(rowInsert gen.InsertMateriaRow) Materia {
	return Materia{
		ID:        rowInsert.ID,
		Codigo:    rowInsert.Codigo,
		Nome:      rowInsert.Nome,
		CreatedAt: rowInsert.CreatedAt.Time,
	}
}

func materiaDeUpdate(rowUpdate gen.UpdateMateriaRow) Materia {
	return materiaDe(gen.InsertMateriaRow{
		ID:        rowUpdate.ID,
		Codigo:    rowUpdate.Codigo,
		Nome:      rowUpdate.Nome,
		CreatedAt: rowUpdate.CreatedAt,
	})
}

func validaMateria(cad CadastroMateria) (string, string, error) {
	codigo := strings.ToUpper(strings.TrimSpace(cad.Codigo))
	nome := strings.TrimSpace(cad.Nome)
	if codigo == "" || nome == "" {
		return "", "", platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe código e nome da matéria.")
	}
	if len(codigo) > 32 {
		return "", "", platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Código muito longo (máx. 32 caracteres).")
	}
	return codigo, nome, nil
}

// CriarMateria cadastra uma matéria (código único, §Organização de Matérias).
func (s *Service) CriarMateria(ctx context.Context, actor domain.User, cad CadastroMateria) (Materia, error) {
	codigo, nome, err := validaMateria(cad)
	if err != nil {
		return Materia{}, err
	}
	row, err := s.store.Q.InsertMateria(ctx, gen.InsertMateriaParams{
		Codigo: codigo, Nome: nome, CreatedBy: int8Ptr(actor.ID),
	})
	if err != nil {
		if eDuplicado(err) {
			return Materia{}, conflito("Código de matéria em uso.")
		}
		return Materia{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindMateriaCreate,
		Detail: "matéria=" + codigo,
	})
	return materiaDe(row), nil
}

// EditarMateria renomeia/recodifica; inexistente → 404, duplicado → 409.
func (s *Service) EditarMateria(ctx context.Context, actor domain.User, id int64, cad CadastroMateria) (Materia, error) {
	codigo, nome, err := validaMateria(cad)
	if err != nil {
		return Materia{}, err
	}
	row, err := s.store.Q.UpdateMateria(ctx, gen.UpdateMateriaParams{
		ID: id, Codigo: codigo, Nome: nome,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Materia{}, materiaNaoEncontrada()
	}
	if err != nil {
		if eDuplicado(err) {
			return Materia{}, conflito("Código de matéria em uso.")
		}
		return Materia{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindMateriaUpdate,
		Detail: fmt.Sprintf("materia=%d código=%s", id, codigo),
	})
	return materiaDeUpdate(row), nil
}

// ExcluirMateria faz soft delete (turmas antigas mantêm a referência).
func (s *Service) ExcluirMateria(ctx context.Context, actor domain.User, id int64) error {
	rows, err := s.store.Q.SoftDeleteMateria(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return materiaNaoEncontrada()
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindMateriaDelete,
		Detail: fmt.Sprintf("materia=%d", id),
	})
	return nil
}

// ListarMaterias devolve as matérias ativas (professor escolhe ao criar turma).
func (s *Service) ListarMaterias(ctx context.Context) ([]Materia, error) {
	rows, err := s.store.Q.ListMaterias(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Materia, 0, len(rows))
	for _, row := range rows {
		out = append(out, materiaDe(gen.InsertMateriaRow{
			ID:        row.ID,
			Codigo:    row.Codigo,
			Nome:      row.Nome,
			CreatedAt: row.CreatedAt,
		}))
	}
	return out, nil
}

// CadastroPeriodo é o corpo (explícito mesmo) do período letivo.
type CadastroPeriodo struct {
	Trimestre int16 `json:"trimestre"`
	Ano       int16 `json:"ano"`
}

// CriarPeriodo registra (ou devolve o já existente) período letivo.
func (s *Service) CriarPeriodo(ctx context.Context, actor domain.User, cad CadastroPeriodo) (Periodo, error) {
	if cad.Trimestre < 1 || cad.Trimestre > 3 || cad.Ano < 2000 || cad.Ano > 2100 {
		return Periodo{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Período inválido: trimestre 1–3; ano plausível.")
	}
	row, err := s.store.Q.FindOrCreatePeriodo(ctx, gen.FindOrCreatePeriodoParams{
		Trimestre: cad.Trimestre, Ano: cad.Ano,
	})
	if err != nil {
		return Periodo{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind: LogKindPeriodoCreate,
		Detail: "trimestre=" + strconv.Itoa(int(cad.Trimestre)) +
			" ano=" + strconv.Itoa(int(cad.Ano)),
	})
	return Periodo{ID: row.ID, Trimestre: row.Trimestre, Ano: row.Ano}, nil
}

// ListarPeriodos devolve os períodos (professor os escolhe ao criar turma).
func (s *Service) ListarPeriodos(ctx context.Context) ([]Periodo, error) {
	rows, err := s.store.Q.ListPeriodos(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Periodo, 0, len(rows))
	for _, row := range rows {
		out = append(out, Periodo{ID: row.ID, Trimestre: row.Trimestre, Ano: row.Ano})
	}
	return out, nil
}
