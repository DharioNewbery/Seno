// Package atribuicoes — conecta uma Turma a uma Atividade (PROJETO
// §Atribuição). Padrões quando ausentes: sem início = disponível
// imediatamente; sem prazo = aberta até encerrar a turma; sem duração =
// sem limite além do prazo. O prazo efetivo (min(início+duração, prazo))
// é sempre calculado no servidor, no fluxo do aluno. Edição fica
// bloqueada a partir da primeira Submissão (futuro, quando submissões
// existirem). Escopo: professor atua nas atribuições das PRÓPRIAS
// turmas (404 fora do escopo); admin/super tudo.
package atribuicoes

import (
	"context"
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

// Service coordena as atribuições.
type Service struct {
	store *store.Store
	audit *platform.Audit
}

func New(st *store.Store, audit *platform.Audit) *Service {
	return &Service{store: st, audit: audit}
}

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindAtribuicaoCreate = "atribuicao.create"
	LogKindAtribuicaoUpdate = "atribuicao.update"
	LogKindAtribuicaoDelete = "atribuicao.delete"
)

// tetoDuracao: 7 dias (segundos).
const tetoDuracao = 7 * 24 * 60 * 60

// Atribuicao é a visão completa de uma atribuição.
type Atribuicao struct {
	ID           int64      `json:"id"`
	TurmaID      int64      `json:"turma_id"`
	AtividadeID  int64      `json:"atividade_id"`
	Autocomplete bool       `json:"autocomplete"`
	Inicio       *time.Time `json:"inicio,omitempty"`
	Prazo        *time.Time `json:"prazo,omitempty"`
	DuracaoSeg   *int32     `json:"duracao_seg,omitempty"`
	PodeAtrasado bool       `json:"pode_atrasado"`
	CreatedAt    time.Time  `json:"created_at"`
	// Contexto (escopo e rótulos para o front).
	TurmaProfessorID int64  `json:"-"`
	TurmaTitulo      string `json:"turma_titulo,omitempty"`
	AtividadeNome    string `json:"atividade_nome,omitempty"`
}

// CadastroAtribuicao é o corpo. Campos de tempo: nil = ausente (aplica o
// padrão §Atribuição). Autocomplete em criação: nil usa o padrão (off
// com prazo/duração — prova; on nas demais); em PATCH, nil preserva o
// valor atual. Turma e atividade não são re-ligados em PATCH (é excluir
// e criar de novo).
type CadastroAtribuicao struct {
	TurmaID      int64      `json:"turma_id"`
	AtividadeID  int64      `json:"atividade_id"`
	Autocomplete *bool      `json:"autocomplete,omitempty"`
	Inicio       *time.Time `json:"inicio,omitempty"`
	Prazo        *time.Time `json:"prazo,omitempty"`
	DuracaoSeg   *int32     `json:"duracao_seg,omitempty"`
	PodeAtrasado *bool      `json:"pode_atrasado,omitempty"`
}

func horaPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func atribuicaoDe(row gen.InsertAtribuicaoRow) Atribuicao {
	return Atribuicao{
		ID: row.ID, TurmaID: row.TurmaID, AtividadeID: row.AtividadeID,
		Autocomplete: row.Autocomplete,
		Inicio: horaPtr(row.Inicio), Prazo: horaPtr(row.Prazo),
		DuracaoSeg: intPtr(row.DuracaoSeg), PodeAtrasado: row.PodeAtrasado,
		CreatedAt: horaDe(row.CreatedAt),
	}
}

func intPtr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	out := v.Int32
	return &out
}

func horaDe(ts pgtype.Timestamptz) time.Time { return ts.Time }

func atribuicaoNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Atribuição não encontrada.")
}

// escopoAtribuicao: staff passa; professor fora do dono da turma → 404.
func escopoAtribuicao(actor domain.User, professorID int64) error {
	if actor.IsStaff() {
		return nil
	}
	if actor.ID != professorID {
		return atribuicaoNaoEncontrada()
	}
	return nil
}

// validaConf valida as marcações: duração 1..7d; prazo não anterior ao
// início; pode_atrasado no § (marcado) só interage com o prazo efetivo.
func validaConf(cad CadastroAtribuicao) error {
	if cad.DuracaoSeg != nil &&
		(*cad.DuracaoSeg < 1 || int(*cad.DuracaoSeg) > tetoDuracao) {
		return platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Duração inválida (1 seg a 7 dias).")
	}
	if cad.Inicio != nil && cad.Prazo != nil && cad.Prazo.Before(*cad.Inicio) {
		return platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Prazo não pode ser anterior ao início.")
	}
	return nil
}

// autocompletePadrao do §Atribuição: desligado quando há prazo ou
// duração (prova); ligado nas demais.
func autocompletePadrao(cad CadastroAtribuicao) bool {
	if cad.Autocomplete != nil {
		return *cad.Autocomplete
	}
	return cad.Prazo == nil && cad.DuracaoSeg == nil
}

func podeAtrasadoPadrao(cad CadastroAtribuicao) bool {
	return cad.PodeAtrasado != nil && *cad.PodeAtrasado
}

// Criar liga a atividade à turma (dono da turma cria; atividade deve
// ser do MESMO professor que o dono da turma — envelopes por professor).
func (s *Service) Criar(ctx context.Context, actor domain.User, cad CadastroAtribuicao) (Atribuicao, error) {
	// Turma e escopo.
	turma, err := s.store.Q.GetTurma(ctx, cad.TurmaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Atribuicao{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound, "Turma não encontrada.")
		}
		return Atribuicao{}, err
	}
	if err := escopoAtribuicao(actor, turma.ProfessorID); err != nil {
		return Atribuicao{}, err
	}
	if turma.EncerradoEm.Valid {
		return Atribuicao{}, platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict, "Turma encerrada.")
	}
	if err := validaConf(cad); err != nil {
		return Atribuicao{}, err
	}

	// Atividade: ativa, do escopo (mesmo professor da turma).
	atv, err := s.store.Q.GetAtividade(ctx, cad.AtividadeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Atribuicao{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound, "Atividade não encontrada.")
		}
		return Atribuicao{}, err
	}
	if atv.DeletedAt.Valid {
		return Atribuicao{}, platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound, "Atividade não encontrada.")
	}
	if atv.ProfessorID != turma.ProfessorID {
		return Atribuicao{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Atividade deve ser do mesmo professor da turma.")
	}

	row, err := s.store.Q.InsertAtribuicao(ctx, gen.InsertAtribuicaoParams{
		TurmaID:     cad.TurmaID,
		AtividadeID: cad.AtividadeID,
		Autocomplete: autocompletePadrao(cad),
		Inicio:      tsOrNull(cad.Inicio),
		Prazo:       tsOrNull(cad.Prazo),
		DuracaoSeg:  intOrNull(cad.DuracaoSeg),
		PodeAtrasado: podeAtrasadoPadrao(cad),
		CreatedBy:   int8Ptr(actor.ID),
	})
	if err != nil {
		if eDuplicado(err) {
			return Atribuicao{}, platform.NewAPIError(
				http.StatusConflict, platform.CodeConflict,
				"Esta atividade já está atribuída à turma.")
		}
		return Atribuicao{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindAtribuicaoCreate,
		Detail: fmt.Sprintf("atividade=%d turma=%d", cad.AtividadeID, cad.TurmaID),
	})
	out := atribuicaoDe(row)
	out.TurmaProfessorID = turma.ProfessorID
	out.TurmaTitulo = tituloDe(turma.Titulo)
	out.AtividadeNome = atv.Nome
	return out, nil
}

func tsOrNull(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func intOrNull(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func int8Ptr(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

func eDuplicado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func tituloDe(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Atribuicao com contexto completo via GetAtribuicaoRow.
func atribuicaoDeGet(row gen.GetAtribuicaoRow) Atribuicao {
	return Atribuicao{
		ID: row.ID, TurmaID: row.TurmaID, AtividadeID: row.AtividadeID,
		Autocomplete: row.Autocomplete,
		Inicio:       horaPtr(row.Inicio), Prazo: horaPtr(row.Prazo),
		DuracaoSeg: intPtr(row.DuracaoSeg), PodeAtrasado: row.PodeAtrasado,
		CreatedAt: horaDe(row.CreatedAt),
		TurmaProfessorID: row.TurmaProfessorID,
		TurmaTitulo:      tituloDe(row.TurmaTitulo),
		AtividadeNome:    row.AtividadeNome,
	}
}

// Ver devolve a atribuição (404 fora do escopo do dono da turma).
func (s *Service) Ver(ctx context.Context, actor domain.User, id int64) (Atribuicao, error) {
	row, err := s.store.Q.GetAtribuicao(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Atribuicao{}, atribuicaoNaoEncontrada()
		}
		return Atribuicao{}, err
	}
	if err := escopoAtribuicao(actor, row.TurmaProfessorID); err != nil {
		return Atribuicao{}, err
	}
	return atribuicaoDeGet(row), nil
}

// FiltroAtribuicoes parametriza a listagem (professor é forçado às
// próprias turmas).
type FiltroAtribuicoes struct {
	TurmaID     *int64
	AtividadeID *int64
	Pagina      *int
	Por         *int
}

// ListaAtribuicoes é o resultado paginado de atribuições.
type ListaAtribuicoes struct {
	Total       int64        `json:"total"`
	Pagina      int          `json:"pagina"`
	Por         int          `json:"por"`
	Atribuicoes []Atribuicao `json:"atribuicoes"`
}

// Listar: professor vê apenas atribuições de turmas próprias (filtro
// via escopo linha a linha: turma alheia = 404 em Ver; nesta lista é
// escondida pela entrega das exclusivas do ator, §5.4).
func (s *Service) Listar(ctx context.Context, actor domain.User, f FiltroAtribuicoes) (ListaAtribuicoes, error) {
	pagina, por := paginaPadrao(f.Pagina, f.Por)
	rows, err := s.store.Q.ListAtribuicoes(ctx, gen.ListAtribuicoesParams{
		Turma:     intOrNull64(f.TurmaID),
		Atividade: intOrNull64(f.AtividadeID),
		Limit:     int32(por),
		Offset:    int32((pagina - 1) * por),
	})
	if err != nil {
		return ListaAtribuicoes{}, err
	}
	out := make([]Atribuicao, 0, len(rows))
	for _, row := range rows {
		item := Atribuicao{
			ID: row.ID, TurmaID: row.TurmaID, AtividadeID: row.AtividadeID,
			Autocomplete: row.Autocomplete,
			Inicio:       horaPtr(row.Inicio), Prazo: horaPtr(row.Prazo),
			DuracaoSeg: intPtr(row.DuracaoSeg), PodeAtrasado: row.PodeAtrasado,
			CreatedAt: horaDe(row.CreatedAt),
			TurmaTitulo:   tituloDe(row.TurmaTitulo),
			AtividadeNome: row.AtividadeNome,
			TurmaProfessorID: row.TurmaProfessorID,
		}
		// Escopo §5.4: professor não vê atribuição de turma alheia.
		if err := escopoAtribuicao(actor, row.TurmaProfessorID); err != nil {
			continue
		}
		out = append(out, item)
	}
	return ListaAtribuicoes{
		Total: int64(len(out)), Pagina: pagina, Por: por, Atribuicoes: out,
	}, nil
}

// Editar muda as marcações (autocomplete/início/prazo/duração/atraso).
// Bloqueio por submissão entra no futuro (quando submissões existirem).
func (s *Service) Editar(ctx context.Context, actor domain.User, id int64, cad CadastroAtribuicao) (Atribuicao, error) {
	atual, err := s.Ver(ctx, actor, id)
	if err != nil {
		return Atribuicao{}, err
	}
	// Em PATCH, nil preserva o valor atual.
	if cad.Inicio == nil {
		cad.Inicio = atual.Inicio
	}
	if cad.Prazo == nil {
		cad.Prazo = atual.Prazo
	}
	if cad.DuracaoSeg == nil {
		cad.DuracaoSeg = atual.DuracaoSeg
	}
	if cad.Autocomplete == nil {
		aut := atual.Autocomplete
		cad.Autocomplete = &aut
	}
	if cad.PodeAtrasado == nil {
		atr := atual.PodeAtrasado
		cad.PodeAtrasado = &atr
	}
	if err := validaConf(cad); err != nil {
		return Atribuicao{}, err
	}
	row, err := s.store.Q.UpdateAtribuicao(ctx, gen.UpdateAtribuicaoParams{
		ID: id,
		Autocomplete: *cad.Autocomplete,
		Inicio:       tsOrNull(cad.Inicio),
		Prazo:        tsOrNull(cad.Prazo),
		DuracaoSeg:   intOrNull(cad.DuracaoSeg),
		PodeAtrasado: *cad.PodeAtrasado,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Atribuicao{}, atribuicaoNaoEncontrada()
	}
	if err != nil {
		return Atribuicao{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindAtribuicaoUpdate,
		Detail: fmt.Sprintf("atribuicao=%d", id),
	})
	out := Atribuicao{
		ID: row.ID, TurmaID: row.TurmaID, AtividadeID: row.AtividadeID,
		Autocomplete: row.Autocomplete,
		Inicio:       horaPtr(row.Inicio), Prazo: horaPtr(row.Prazo),
		DuracaoSeg: intPtr(row.DuracaoSeg), PodeAtrasado: row.PodeAtrasado,
		CreatedAt: horaDe(row.CreatedAt),
		TurmaTitulo:      atual.TurmaTitulo,
		AtividadeNome:    atual.AtividadeNome,
		TurmaProfessorID: atual.TurmaProfessorID,
	}
	return out, nil
}

// Excluir desliga a atividade da turma (dono|staff).
func (s *Service) Excluir(ctx context.Context, actor domain.User, id int64) error {
	if _, err := s.Ver(ctx, actor, id); err != nil {
		return err
	}
	rows, err := s.store.Q.DeleteAtribuicao(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return atribuicaoNaoEncontrada()
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindAtribuicaoDelete,
		Detail: fmt.Sprintf("atribuicao=%d", id),
	})
	return nil
}

// helpers -----------------------------------------------------------

func intOrNull64(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
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
