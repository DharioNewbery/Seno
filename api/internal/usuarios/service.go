// Package usuarios — gestão de professores e admins pelo portal admin
// (PROJETO §Portal Admin, ARQUITETURA §5.4). Criação segue §5.3: o novo
// usuário nasce pendente e recebe convite por e-mail para definir a senha.
// Super admin é singleton (bootstrap): não se cria, não se bloqueia.
// Alunos entram junto com turmas/matrículas, em outra rodada.
package usuarios

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/auth"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindCreate  = "user.create"
	LogKindUpdate  = "user.update"
	LogKindDisable = "user.disable"
	LogKindEnable  = "user.enable"
)

// Service coordena o CRUD de professores e admins.
type Service struct {
	store     *store.Store
	audit     *platform.Audit
	recoverer *auth.Recoverer
}

func New(st *store.Store, audit *platform.Audit, rec *auth.Recoverer) *Service {
	return &Service{store: st, audit: audit, recoverer: rec}
}

// Filtros da listagem (rota GET /v1/users).
type Filtros struct {
	Busca  string      // e-mail e nome/sobrenome (ILIKE)
	Status string      // active|pending|disabled|"" (todos)
	Cargo  domain.Role // cargo exato (opcional)
	Pagina int         // 1-based
	Por    int         // itens por página (teto 200)
}

// Pagina é o resultado paginado da listagem.
type Pagina struct {
	Total    int64         `json:"total"`
	Pagina   int           `json:"pagina"`
	Por      int           `json:"por"`
	Usuarios []domain.User `json:"usuarios"`
}

var statusValidos = map[string]bool{
	"": true, "active": true, "pending": true, "disabled": true,
}

// Listar devolve os usuários com filtros e o total da consulta.
func (s *Service) Listar(ctx context.Context, f Filtros) (Pagina, error) {
	if !statusValidos[f.Status] {
		return Pagina{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Status inválido (use active, pending ou disabled).")
	}
	if f.Cargo != "" && !domain.IsValidRole(f.Cargo) {
		return Pagina{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Cargo inválido.")
	}
	f.Busca = strings.TrimSpace(f.Busca)
	if f.Pagina < 1 {
		f.Pagina = 1
	}
	if f.Por < 1 {
		f.Por = 50
	}
	if f.Por > 200 {
		f.Por = 200
	}

	total, err := s.store.Q.CountUsers(ctx, gen.CountUsersParams{
		Column1: f.Busca, Column2: f.Status, Column3: string(f.Cargo),
	})
	if err != nil {
		return Pagina{}, err
	}
	rows, err := s.store.Q.ListUsers(ctx, gen.ListUsersParams{
		Column1: f.Busca,
		Column2: f.Status,
		Column3: string(f.Cargo),
		Limit:   int32(f.Por),
		Offset:  int32((f.Pagina - 1) * f.Por),
	})
	if err != nil {
		return Pagina{}, err
	}

	// Cargos em lote para a página inteira.
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	cargos, err := s.store.Q.ListUserRolesByIDs(ctx, ids)
	if err != nil {
		return Pagina{}, err
	}
	porID := map[int64][]domain.Role{}
	for _, cr := range cargos {
		porID[cr.UserID] = append(porID[cr.UserID], domain.Role(cr.Role))
	}

	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		roles, ok := porID[row.ID]
		if !ok {
			roles = []domain.Role{}
		}
		users = append(users, domain.User{
			ID:        row.ID,
			Email:     row.Email,
			FirstName: row.FirstName,
			LastName:  row.LastName,
			Status:    row.Status,
			Roles:     roles,
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return Pagina{Total: total, Pagina: f.Pagina, Por: f.Por, Usuarios: users}, nil
}

// Cadastro é o corpo da criação de usuário (professor ou admin).
type Cadastro struct {
	Email     string      `json:"email"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Cargo     domain.Role `json:"cargo"`
}

// Criar cadastra professor/admin como pendente e envia convite por e-mail.
func (s *Service) Criar(ctx context.Context, actor domain.User, cad Cadastro) (domain.User, error) {
	switch cad.Cargo {
	case domain.RoleProfessor:
	case domain.RoleAdmin:
		if !actor.IsSuper() {
			return domain.User{}, platform.NewAPIError(
				http.StatusForbidden, platform.CodeForbidden,
				"Só o super admin cria outros admins.")
		}
	case domain.RoleSuper:
		return domain.User{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Existe apenas um super admin, criado na inicialização.")
	default:
		return domain.User{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Gestão de alunos entra junto com turmas e matérias; use professor ou admin.")
	}
	if strings.TrimSpace(cad.Email) == "" ||
		strings.TrimSpace(cad.FirstName) == "" ||
		strings.TrimSpace(cad.LastName) == "" {
		return domain.User{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe e-mail, nome e sobrenome.")
	}

	// Pendente + convite por e-mail (§5.3); pendente pré-existente reenvia;
	// ativo/bloqueado dá 409/403 do serviço de autenticação.
	if err := s.recoverer.Invite(
		ctx, cad.Email, cad.FirstName, cad.LastName, &actor.ID,
	); err != nil {
		return domain.User{}, err
	}
	email := strings.ToLower(strings.TrimSpace(cad.Email))
	row, err := s.store.Q.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}

	roles, err := s.store.Q.GetUserRoles(ctx, row.ID)
	if err != nil {
		return domain.User{}, err
	}
	if !domain.HasAnyRole(rolesToSlice(roles), cad.Cargo) {
		if err := s.store.Q.InsertUserRole(ctx, gen.InsertUserRoleParams{
			UserID: row.ID, Role: string(cad.Cargo),
		}); err != nil {
			return domain.User{}, err
		}
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID,
		Actor:   actor.Email,
		Kind:    LogKindCreate,
		Detail:  fmt.Sprintf("usuario=%d cargo=%s", row.ID, cad.Cargo),
	})
	rolesFinais := rolesToSlice(roles)
	if !domain.HasAnyRole(rolesFinais, cad.Cargo) {
		rolesFinais = append(rolesFinais, cad.Cargo)
	}
	return domain.User{
		ID:        row.ID,
		Email:     row.Email,
		FirstName: row.FirstName,
		LastName:  row.LastName,
		Status:    row.Status,
		Roles:     rolesFinais,
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

// EditarDados atualiza nome/sobrenome e registra o agente (updated_by).
// Contas de admin: só o super, exceto o próprio usuário editando a si.
func (s *Service) EditarDados(ctx context.Context, actor domain.User, id int64, firstName, lastName string) (domain.User, error) {
	if strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" {
		return domain.User{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Nome e sobrenome são obrigatórios.")
	}
	target, personID, err := s.alvo(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if precisaSuper(actor, target) && actor.ID != target.ID {
		return domain.User{}, platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Só o super admin opera sobre contas de admin.")
	}
	if err := s.store.Q.UpdatePerson(ctx, gen.UpdatePersonParams{
		FirstName: firstName, LastName: lastName, ID: personID,
	}); err != nil {
		return domain.User{}, err
	}
	if err := s.store.Q.SetUserUpdatedBy(ctx, gen.SetUserUpdatedByParams{
		ID: id, UpdatedBy: int8Ptr(actor.ID),
	}); err != nil {
		return domain.User{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID,
		Actor:   actor.Email,
		Kind:    LogKindUpdate,
		Detail:  fmt.Sprintf("usuario=%d", id),
	})
	target.FirstName = firstName
	target.LastName = lastName
	return target, nil
}

// Bloquear desativa a conta (login 403 e sessões morrem na hora, service).
// Super é singleton: ninguém bloqueia super, nem a si mesma (403).
func (s *Service) Bloquear(ctx context.Context, actor domain.User, id int64) error {
	target, _, err := s.alvo(ctx, id)
	if err != nil {
		return err
	}
	if target.IsSuper() || actor.ID == target.ID {
		return platform.NewAPIError(http.StatusForbidden, platform.CodeForbidden,
			"O super admin não pode ser bloqueado.")
	}
	if precisaSuper(actor, target) {
		return platform.NewAPIError(http.StatusForbidden, platform.CodeForbidden,
			"Só o super admin opera sobre contas de admin.")
	}
	if err := s.store.Q.DisableUser(ctx, gen.DisableUserParams{
		ID: id, UpdatedBy: int8Ptr(actor.ID),
	}); err != nil {
		return err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email, Kind: LogKindDisable,
		Detail: fmt.Sprintf("usuario=%d", id),
	})
	return nil
}

// Desbloquear reativa a conta bloqueada.
func (s *Service) Desbloquear(ctx context.Context, actor domain.User, id int64) error {
	target, _, err := s.alvo(ctx, id)
	if err != nil {
		return err
	}
	if precisaSuper(actor, target) {
		return platform.NewAPIError(http.StatusForbidden, platform.CodeForbidden,
			"Só o super admin opera sobre contas de admin.")
	}
	if err := s.store.Q.EnableUser(ctx, gen.EnableUserParams{
		ID: id, UpdatedBy: int8Ptr(actor.ID),
	}); err != nil {
		return err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email, Kind: LogKindEnable,
		Detail: fmt.Sprintf("usuario=%d", id),
	})
	return nil
}

// PedirNovaSenha envia o e-mail de redefinição para o usuário (uso único,
// 1 h), reaproveitando o fluxo público de recuperação (ARQUITETURA §5.1/§5.3).
func (s *Service) PedirNovaSenha(ctx context.Context, actor domain.User, id int64) error {
	target, _, err := s.alvo(ctx, id)
	if err != nil {
		return err
	}
	if precisaSuper(actor, target) {
		return platform.NewAPIError(http.StatusForbidden, platform.CodeForbidden,
			"Só o super admin opera sobre contas de admin.")
	}
	if err := s.recoverer.RequestPasswordReset(ctx, target.Email); err != nil {
		return err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email, Kind: LogKindUpdate,
		Detail: fmt.Sprintf("usuario=%d pedido de redefinição", id),
	})
	return nil
}

// alvo carrega o usuário com cargos; 404 quando não existe. Devolve o ID
// da pessoa (para edição de nome/sobrenome).
func (s *Service) alvo(ctx context.Context, id int64) (domain.User, int64, error) {
	row, err := s.store.Q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, 0, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound, "Usuário não encontrado.")
		}
		return domain.User{}, 0, err
	}
	rolesStr, err := s.store.Q.GetUserRoles(ctx, id)
	if err != nil {
		return domain.User{}, 0, err
	}
	u := domain.User{
		ID:        row.ID,
		Email:     row.Email,
		FirstName: row.FirstName,
		LastName:  row.LastName,
		Status:    row.Status,
		Roles:     rolesToSlice(rolesStr),
		CreatedAt: row.CreatedAt.Time,
	}
	if row.DisabledAt.Valid {
		u.DisabledAt = &row.DisabledAt.Time
	}
	if row.UpdatedAt.Valid {
		u.UpdatedAt = &row.UpdatedAt.Time
	}
	return u, row.PersonID, nil
}

// precisaSuper devolve se o alvo é admin/super (então só o super opera).
func precisaSuper(actor, target domain.User) bool {
	return domain.HasAnyRole(target.Roles, domain.RoleAdmin, domain.RoleSuper) &&
		!actor.IsSuper()
}

func int8Ptr(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }

func rolesToSlice(in []string) []domain.Role {
	out := make([]domain.Role, 0, len(in))
	for _, r := range in {
		out = append(out, domain.Role(r))
	}
	return out
}
