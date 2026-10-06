// Package ensino — matérias, períodos letivos, turmas e matrículas
// (PROJETO §Organização). Regras do §5.4 aplicadas na camada de serviço:
// professor só enxerga e opera sobre as PRÓPRIAS turmas (404 fora do
// escopo, nunca 403 que revele existência); admin/super enxergam tudo.
package ensino

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/internal/auth"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// Service coordena o domínio de ensino.
type Service struct {
	store     *store.Store
	audit     *platform.Audit
	recoverer *auth.Recoverer
}

func New(st *store.Store, audit *platform.Audit, rec *auth.Recoverer) *Service {
	return &Service{store: st, audit: audit, recoverer: rec}
}

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindMateriaCreate = "materia.create"
	LogKindMateriaUpdate = "materia.update"
	LogKindMateriaDelete = "materia.delete"
	LogKindPeriodoCreate = "periodo.create"
	LogKindTurmaCreate   = "turma.create"
	LogKindTurmaUpdate   = "turma.update"
	LogKindTurmaDelete   = "turma.delete"
	LogKindMatriculaNew  = "matricula.create"
	LogKindMatriculaExit = "matricula.encerrar"
)

// Materia é a visão completa de uma matéria (gestão) ou enxuta (turmas).
type Materia struct {
	ID        int64     `json:"id"`
	Codigo    string    `json:"codigo"`
	Nome      string    `json:"nome"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// Periodo é um período letivo (trimestre/ano).
type Periodo struct {
	ID        int64 `json:"id"`
	Trimestre int16 `json:"trimestre"`
	Ano       int16 `json:"ano"`
}

// Aluno é a visão do aluno dentro das matrículas.
type Aluno struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Status    string `json:"status"`
}

// Professor é a visão do professor dono da turma.
type Professor struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// Turma é a visão completa da turma.
type Turma struct {
	ID          int64      `json:"id"`
	Materia     Materia    `json:"materia"`
	Trimestre   int16      `json:"trimestre"`
	Ano         int16      `json:"ano"`
	Titulo      string     `json:"titulo,omitempty"`
	Professor   Professor  `json:"professor"`
	EncerradoEm *time.Time `json:"encerrado_em,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Matricula liga o aluno à turma (entrada/saída, PROJETO §Matrícula).
type Matricula struct {
	ID        int64      `json:"id"`
	Aluno     Aluno      `json:"aluno"`
	EntradaEm time.Time  `json:"entrada_em"`
	SaidaEm   *time.Time `json:"saida_em,omitempty"`
}

// ListaTurmas é o resultado paginado das turmas.
type ListaTurmas struct {
	Total  int64   `json:"total"`
	Pagina int     `json:"pagina"`
	Por    int     `json:"por"`
	Turmas []Turma `json:"turmas"`
}

// ListaMatriculas é o resultado da listagem de matrículas de uma turma.
type ListaMatriculas struct {
	Total      int64       `json:"total"`
	Matriculas []Matricula `json:"matriculas"`
}

// escopoTurma aplica o §5.4: admin/super passam; professor que não é dono
// recebe 404 (igual a "não existe") — não revelar turmas alheias.
func escopoTurma(actor domain.User, professorID int64) error {
	if actor.IsStaff() {
		return nil
	}
	if actor.ID != professorID {
		return turmaNaoEncontrada()
	}
	return nil
}

func turmaNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Turma não encontrada.")
}

func materiaNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Matéria não encontrada.")
}

func matriculaNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Matrícula não encontrada.")
}

func paginaPadrao(pagina, por *int) (int, int) {
	p, n := 1, 50
	if pagina != nil && *pagina > 1 {
		p = *pagina
	}
	if por != nil && *por > 0 {
		n = *por
	}
	if n > 200 {
		n = 200
	}
	return p, n
}

// helpers -----------------------------------------------------------

func int8Ptr(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

// eDuplicado identifica violação de constraint única (23505).
func eDuplicado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// eReferenciado identifica violação de chave estrangeira (23503).
func eReferenciado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func conflito(msg string) error {
	return platform.NewAPIError(http.StatusConflict, platform.CodeConflict, msg)
}
