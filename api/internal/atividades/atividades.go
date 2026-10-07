// Package atividades — banco de atividades do professor (PROJETO
// §Atividades): conjunto de tarefas com ordem, pontuação e linguagem
// por tarefa, e conteúdo JSON pré-hidratação (schema_version 1). Escopo
// §5.4: professor só as próprias (404 fora do escopo); staff tudo.
package atividades

import (
	"context"
	"encoding/json"
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
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/tarefas"
)

// Service coordena o banco de atividades.
type Service struct {
	store   *store.Store
	audit   *platform.Audit
	tarefaS *tarefas.Service
}

func New(st *store.Store, audit *platform.Audit, tarefaS *tarefas.Service) *Service {
	return &Service{store: st, audit: audit, tarefaS: tarefaS}
}

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindAtividadeCreate   = "atividade.create"
	LogKindAtividadeUpdate   = "atividade.update"
	LogKindAtividadeDelete   = "atividade.delete"
	LogKindAtividadeDuplicar = "atividade.duplicar"
)

// Linguagens suportadas na v1 (PROJETO §Atividades; o identificador é
// próprio do Seno e mapeia para o language_id do Judge0).
var linguagens = map[string]bool{"python": true, "c": true, "cpp": true}

// TarefaAtributo é o vínculo por tarefa na atividade: ordem, pontuação
// e linguagem (definidas pelo professor na Atividade, PROJETO §Tarefas).
type TarefaAtributo struct {
	TarefaID  int64  `json:"tarefa_id"`
	Ordem     int    `json:"ordem"`
	ValorPts  int    `json:"valor_pts"`
	Linguagem string `json:"linguagem"`
}

// DetalheTarefa é a visão da tarefa dentro de uma atividade (dados da
// tarefa + os atributos da atividade).
type DetalheTarefa struct {
	TarefaID        int64      `json:"tarefa_id"`
	Ordem           int        `json:"ordem"`
	ValorPts        int        `json:"valor_pts"`
	Linguagem       string     `json:"linguagem"`
	TarefaNome      string     `json:"tarefa_nome"`
	TarefaEnunciado string     `json:"tarefa_enunciado"`
	TarefaExcluida  *time.Time `json:"tarefa_excluida,omitempty"`
}

// Atividade é a visão completa da atividade.
type Atividade struct {
	ID          int64           `json:"id"`
	ProfessorID int64           `json:"professor_id"`
	Nome        string          `json:"nome"`
	Conteudo    json.RawMessage `json:"conteudo"`
	Tarefas     []DetalheTarefa `json:"tarefas,omitempty"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ListaAtividades é o resultado paginado (atividades são listadas sem
// tarefas; Ver traz o detalhe).
type ListaAtividades struct {
	Total      int64        `json:"total"`
	Pagina     int          `json:"pagina"`
	Por        int          `json:"por"`
	Atividades []Atividade  `json:"atividades"`
}

// CadastroAtividade é o corpo da criação/edição. Em PATCH, tarefas
// substituem a lista completa.
type CadastroAtividade struct {
	Nome     string           `json:"nome"`
	Conteudo json.RawMessage  `json:"conteudo"`
	Tarefas  []TarefaAtributo `json:"tarefas"`
}

// FiltroAtividades parametriza a listagem (professor forçado ao próprio id).
type FiltroAtividades struct {
	ProfessorID *int64
	Pagina      *int
	Por         *int
}

// validoConteudo checa o JSON do conteúdo: objeto com schema_version=1.
// O hidratar/render é responsabilidade do front; aqui apenas a sanidade.
func validaConteudo(bruto json.RawMessage) (json.RawMessage, error) {
	if len(bruto) == 0 {
		return nil, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Conteúdo da atividade ausente.")
	}
	var est struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(bruto, &est); err != nil || est.SchemaVersion != 1 {
		return nil, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Conteúdo inválido: objeto JSON com schema_version=1.")
	}
	return json.RawMessage(bruto), nil
}

func validaTarefas(atribuicoes []TarefaAtributo) ([]TarefaAtributo, error) {
	if len(atribuicoes) == 0 {
		return nil, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Atividade sem tarefas.")
	}
	for i := range atribuicoes {
		t := &atribuicoes[i]
		if t.TarefaID < 1 {
			return nil, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"tarefa_id inválido.")
		}
		if t.Ordem < 1 || t.Ordem > len(atribuicoes) {
			return nil, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"Ordem das tarefas deve ser 1..N.")
		}
		if t.ValorPts < 0 {
			return nil, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"Pontuação não pode ser negativa.")
		}
		if !linguagens[t.Linguagem] {
			return nil, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"Linguagem inválida (python, c ou cpp).")
		}
	}
	return atribuicoes, nil
}

func namae(nome string) (string, error) {
	nome = strings.TrimSpace(nome)
	if len(nome) == 0 {
		return "", platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe o nome da atividade.")
	}
	if len(nome) > 128 {
		return "", platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Nome muito longo (máx. 128 caracteres).")
	}
	return nome, nil
}

func int8Ptr(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

func horaDe(ts pgtype.Timestamptz) time.Time { return ts.Time }

func horaPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func atividadesDeListas(row gen.ListAtividadesRow) Atividade {
	return Atividade{
		ID: row.ID, ProfessorID: row.ProfessorID, Nome: row.Nome,
		DeletedAt: horaPtr(row.DeletedAt), CreatedAt: horaDe(row.CreatedAt),
	}
}

// gravarTarefas (em tx): valida e grava a lista de tarefas da atividade.
func (s *Service) gravarTarefas(
	ctx context.Context, q *gen.Queries, id int64, lista []TarefaAtributo,
) error {
	for _, t := range lista {
		if err := q.InsertAtividadeTarefa(ctx, gen.InsertAtividadeTarefaParams{
			AtividadeID: id,
			TarefaID:    t.TarefaID,
			Ordem:       int32(t.Ordem),
			ValorPts:    int32(t.ValorPts),
			Linguagem:   t.Linguagem,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Criar registra a atividade (dono = ator; cargo professor).
func (s *Service) Criar(ctx context.Context, actor domain.User, cad CadastroAtividade) (Atividade, error) {
	if !domain.HasAnyRole(actor.Roles, domain.RoleProfessor) {
		return Atividade{}, platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Apenas professores criam atividades.")
	}
	nome, err := namae(cad.Nome)
	if err != nil {
		return Atividade{}, err
	}
	conteudo, err := validaConteudo(cad.Conteudo)
	if err != nil {
		return Atividade{}, err
	}
	lista, err := validaTarefas(cad.Tarefas)
	if err != nil {
		return Atividade{}, err
	}
	if err := s.donoDasTarefas(ctx, actor, lista); err != nil {
		return Atividade{}, err
	}

	var nova Atividade
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		row, err := q.InsertAtividade(ctx, gen.InsertAtividadeParams{
			ProfessorID: actor.ID, Nome: nome,
			Conteudo: conteudo, CreatedBy: int8Ptr(actor.ID),
		})
		if err != nil {
			return err
		}
		if err := s.gravarTarefas(ctx, q, row.ID, lista); err != nil {
			return err
		}
		nova = Atividade{
			ID: row.ID, ProfessorID: row.ProfessorID, Nome: row.Nome,
			Conteudo: row.Conteudo,
			DeletedAt: horaPtr(row.DeletedAt), CreatedAt: horaDe(row.CreatedAt),
		}
		return nil
	})
	if err != nil {
		return Atividade{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindAtividadeCreate,
		Detail: fmt.Sprintf("atividade=%d tarefas=%d", nova.ID, len(lista)),
	})
	return nova, nil
}

// donoDasTarefas: todas as tarefas referenciadas devem existir e ser do
// ator (professor) — atividade é envelope do banco do próprio professor.
func (s *Service) donoDasTarefas(
	ctx context.Context, actor domain.User, lista []TarefaAtributo,
) error {
	for _, t := range lista {
		row, err := s.store.Q.GetTarefa(ctx, t.TarefaID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return tarefaInexistente(t.TarefaID)
			}
			return err
		}
		if row.DeletedAt.Valid {
			return tarefaInexistente(t.TarefaID)
		}
		if !actor.IsStaff() && row.ProfessorID != actor.ID {
			// 404 (não revelar bancos de tarefas alheios).
			return platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Tarefa "+fmt.Sprintf("%d", t.TarefaID)+" não encontrada.")
		}
	}
	return nil
}

func tarefaInexistente(id int64) error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound,
		"Tarefa "+fmt.Sprintf("%d", id)+" não encontrada.")
}

func atividadesNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Atividade não encontrada.")
}

// escopoAtividade: staff passa; professor não-dono → 404.
func escopoAtividade(actor domain.User, professorID int64) error {
	if actor.IsStaff() {
		return nil
	}
	if actor.ID != professorID {
		return atividadesNaoEncontrada()
	}
	return nil
}

// Ver devolve a atividade com as tarefas detalhadas.
func (s *Service) Ver(ctx context.Context, actor domain.User, id int64) (Atividade, error) {
	row, err := s.store.Q.GetAtividade(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Atividade{}, atividadesNaoEncontrada()
		}
		return Atividade{}, err
	}
	profID := row.ProfessorID
	if err := escopoAtividade(actor, profID); err != nil {
		return Atividade{}, err
	}
	vinculos, err := s.store.Q.ListAtividadeTarefas(ctx, id)
	if err != nil {
		return Atividade{}, err
	}
	detalhes := make([]DetalheTarefa, 0, len(vinculos))
	for _, vt := range vinculos {
		detalhes = append(detalhes, DetalheTarefa{
			TarefaID:        vt.TarefaID,
			Ordem:           int(vt.Ordem),
			ValorPts:        int(vt.ValorPts),
			Linguagem:       vt.Linguagem,
			TarefaNome:      vt.TarefaNome,
			TarefaEnunciado: vt.TarefaEnunciado,
			TarefaExcluida:  horaPtr(vt.TarefaDeletedAt),
		})
	}
	return Atividade{
		ID: row.ID, ProfessorID: row.ProfessorID, Nome: row.Nome,
		Conteudo: row.Conteudo, Tarefas: detalhes,
		DeletedAt: horaPtr(row.DeletedAt), CreatedAt: horaDe(row.CreatedAt),
	}, nil
}

// Listar: professor recebe apenas as próprias (filtro forçado).
func (s *Service) Listar(ctx context.Context, actor domain.User, f FiltroAtividades) (ListaAtividades, error) {
	prof := f.ProfessorID
	if !actor.IsStaff() {
		pro := actor.ID
		prof = &pro
	}
	pagina, por := paginaPadrao(f.Pagina, f.Por)
	total, err := s.store.Q.CountAtividades(ctx, profInt(prof))
	if err != nil {
		return ListaAtividades{}, err
	}
	rows, err := s.store.Q.ListAtividades(ctx, gen.ListAtividadesParams{
		Professor: profInt(prof),
		Limit:     int32(por),
		Offset:    int32((pagina - 1) * por),
	})
	if err != nil {
		return ListaAtividades{}, err
	}
	out := make([]Atividade, 0, len(rows))
	for _, row := range rows {
		out = append(out, atividadesDeListas(row))
	}
	return ListaAtividades{
		Total: total, Pagina: pagina, Por: por, Atividades: out,
	}, nil
}

// Editar muda nome/conteúdo/tarefas; atividade com submissão → 409
// (futuro: quando Submissões existirem; agora sempre editável se alcançada).
func (s *Service) Editar(ctx context.Context, actor domain.User, id int64, cad CadastroAtividade) (Atividade, error) {
	atual, err := s.Ver(ctx, actor, id)
	if err != nil {
		return Atividade{}, err
	}
	_ = atual
	nome, err := namae(cad.Nome)
	if err != nil {
		return Atividade{}, err
	}
	conteudo, err := validaConteudo(cad.Conteudo)
	if err != nil {
		return Atividade{}, err
	}
	lista, err := validaTarefas(cad.Tarefas)
	if err != nil {
		return Atividade{}, err
	}
	if err := s.donoDasTarefas(ctx, actor, lista); err != nil {
		return Atividade{}, err
	}
	if _, err := s.store.Q.UpdateAtividade(ctx, gen.UpdateAtividadeParams{
		ID: id, Nome: nome, Conteudo: conteudo,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Atividade{}, atividadesNaoEncontrada()
		}
		return Atividade{}, err
	}
	if err := s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		if _, err := q.DeleteAtividadeTarefas(ctx, id); err != nil {
			return err
		}
		return s.gravarTarefas(ctx, q, id, lista)
	}); err != nil {
		return Atividade{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindAtividadeUpdate,
		Detail: fmt.Sprintf("atividade=%d tarefas=%d", id, len(lista)),
	})
	return s.Ver(ctx, actor, id)
}

// Duplicar copia atividade + vínculos de tarefas (nova editável).
func (s *Service) Duplicar(ctx context.Context, actor domain.User, id int64) (Atividade, error) {
	origem, err := s.Ver(ctx, actor, id)
	if err != nil {
		return Atividade{}, err
	}
	novoNome := origem.Nome + " (cópia)"
	if len(novoNome) > 128 {
		novoNome = origem.Nome[:128-len(" (cópia)")] + " (cópia)"
	}
	var copia Atividade
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		row, err := q.CopyAtividade(ctx, gen.CopyAtividadeParams{
			ID: id, Nome: novoNome, CreatedBy: int8Ptr(actor.ID),
		})
		if err != nil {
			return err
		}
		if _, err := q.CopyAtividadeTarefas(ctx, gen.CopyAtividadeTarefasParams{
			AtividadeID:   id,
			AtividadeID_2: row.ID,
		}); err != nil {
			return err
		}
		copia = Atividade{
			ID: row.ID, ProfessorID: row.ProfessorID, Nome: row.Nome,
			Conteudo: row.Conteudo,
			DeletedAt: horaPtr(row.DeletedAt), CreatedAt: horaDe(row.CreatedAt),
		}
		return nil
	})
	if err != nil {
		return Atividade{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindAtividadeDuplicar,
		Detail: fmt.Sprintf("atividade=%d → %d", id, copia.ID),
	})
	return copia, nil
}

// Excluir: apaga definitivamente (sem submissões hoje — hard delete;
// com submissões, soft delete e visível somente leitura no futuro).
func (s *Service) Excluir(ctx context.Context, actor domain.User, id int64) error {
	if _, err := s.Ver(ctx, actor, id); err != nil {
		return err
	}
	rows, err := s.store.Q.DeleteAtividade(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return atividadesNaoEncontrada()
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindAtividadeDelete,
		Detail: fmt.Sprintf("atividade=%d", id),
	})
	return nil
}

// helpers -----------------------------------------------------------

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

func profInt(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}
