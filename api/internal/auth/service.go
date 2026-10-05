package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// Códigos de erro do domínio de autenticação.
const (
	ErrCodeInvalidCredentials = platform.CodeUnauthorized
	ErrCodeRateLimited        = platform.CodeRateLimited
	ErrCodeAccountUnavailable = platform.CodeForbidden
)

// Eventos do Log de autenticação (PROJETO §Log).
const (
	LogKindLogin       = "auth.login"
	LogKindLoginFailed = "auth.login_failed"
	LogKindLogout      = "auth.logout"
)

// Service coordena a autenticação de usuários.
type Service struct {
	store     *store.Store
	audit     *platform.Audit
	ttl       time.Duration
	maxTTL    time.Duration
	limiter   *LoginLimiter
	webOrigin string
}

// New cria o serviço de autenticação.
func New(s *store.Store, audit *platform.Audit, ttl, maxTTL time.Duration, webOrigin string) *Service {
	return &Service{
		store: s, audit: audit,
		ttl: ttl, maxTTL: maxTTL,
		limiter:   NewLoginLimiter(10, 15*time.Minute),
		webOrigin: webOrigin,
	}
}

// Login valida credenciais e abre uma sessão opaca.
func (s *Service) Login(ctx context.Context, email, password, ip, userAgent string) (domain.User, string, time.Time, error) {
	email = normalizeEmail(email)

	key := ip + "|" + email
	if blocked, wait := s.limiter.Blocked(key); blocked {
		return domain.User{}, "", time.Time{}, platform.NewAPIError(
			429, ErrCodeRateLimited,
			fmt.Sprintf("Muitas tentativas. Tente novamente em %s.", wait.Round(time.Second)))
	}

	row, err := s.store.Q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Enumeração evitada: mensagem genérica.
			s.recordFailedLogin(ctx, email, ip, userAgent)
			return domain.User{}, "", time.Time{}, invalidCredentials()
		}
		return domain.User{}, "", time.Time{}, err
	}

	if row.PasswordHash == nil {
		// Usuário pendente (convite não aceito) age como credencial inválida.
		s.recordFailedLogin(ctx, email, ip, userAgent)
		return domain.User{}, "", time.Time{}, invalidCredentials()
	}
	ok, err := VerifyPassword(*row.PasswordHash, password)
	if err != nil || !ok {
		s.recordFailedLogin(ctx, email, ip, userAgent)
		return domain.User{}, "", time.Time{}, invalidCredentials()
	}
	if row.Status == domain.StatusPending {
		s.recordFailedLogin(ctx, email, ip, userAgent)
		return domain.User{}, "", time.Time{}, platform.NewAPIError(
			403, ErrCodeAccountUnavailable, "Conta pendente de convite (defina a senha pelo e-mail recebido).")
	}
	if row.Status == domain.StatusDisabled || row.DisabledAt.Valid {
		s.recordFailedLogin(ctx, email, ip, userAgent)
		return domain.User{}, "", time.Time{}, platform.NewAPIError(
			403, ErrCodeAccountUnavailable, "Conta bloqueada.")
	}

	// Sessão: token opaco de 32 B; banco guarda apenas o SHA-256.
	token, tokenHash, err := NewToken()
	if err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	now := time.Now()
	expires := now.Add(s.ttl)
	if max := now.Add(s.maxTTL); expires.After(max) {
		expires = max
	}
	if _, err := s.store.Q.InsertSession(ctx, gen.InsertSessionParams{
		UserID:       row.ID,
		TokenHash:    tokenHash,
		ExpiresAt:    pgtype.Timestamptz{Time: expires, Valid: true},
		MaxExpiresAt: pgtype.Timestamptz{Time: now.Add(s.maxTTL), Valid: true},
		CreatedIp:    strPtr(ip),
		UserAgent:    strPtr(userAgent),
	}); err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	s.limiter.Reset(key)
	user, err := userFromRow(ctx, s.store, coreFromUsernameRow(row))
	if err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &row.ID,
		Actor:   email,
		Kind:    LogKindLogin,
		Detail:  fmt.Sprintf("ip=%s ua=%q", ip, truncate(userAgent, 200)),
	})
	return user, token, expires, nil
}

// recordFailedLogin registra a falha (rate limit + Log).
func (s *Service) recordFailedLogin(ctx context.Context, email, ip, ua string) {
	s.limiter.RegisterFail(ip + "|" + email)
	s.audit.Record(ctx, platform.LogEntry{
		Actor:  email,
		Kind:   LogKindLoginFailed,
		Detail: fmt.Sprintf("ip=%s ua=%q", ip, truncate(ua, 200)),
	})
}

// Authenticate carrega a sessão pelo token Bearer e renova a janela
// sliding com teto máximo (ARQUITETURA §5.2).
func (s *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	hash := TokenHash(token)
	sess, err := s.store.Q.FindSessionByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, invalidCredentials()
		}
		return domain.User{}, err
	}

	row, err := s.store.Q.GetUserByID(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, invalidCredentials()
		}
		return domain.User{}, err
	}
	if row.Status == domain.StatusDisabled || row.DisabledAt.Valid {
		// Conta bloqueada: sessão encerrada imediatamente.
		if err := s.store.Q.DeleteSessionByTokenHash(ctx, hash); err != nil {
			return domain.User{}, err
		}
		return domain.User{}, invalidCredentials()
	}

	// Renovação sliding: expires_at = min(now + ttl, max_expires_at).
	now := time.Now()
	renewed := now.Add(s.ttl)
	if renewed.After(sess.MaxExpiresAt.Time) {
		renewed = sess.MaxExpiresAt.Time
	}
	if err := s.store.Q.UpdateSessionActivity(ctx, gen.UpdateSessionActivityParams{
		ID:        sess.ID,
		ExpiresAt: pgtype.Timestamptz{Time: renewed, Valid: true},
	}); err != nil {
		return domain.User{}, err
	}

	user, err := userFromRow(ctx, s.store, coreFromIDRow(row))
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// Logout encerra a sessão atual.
func (s *Service) Logout(ctx context.Context, token string, actor domain.User) error {
	hash := TokenHash(token)
	if err := s.store.Q.DeleteSessionByTokenHash(ctx, hash); err != nil {
		return err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID,
		Actor:   actor.Email,
		Kind:    LogKindLogout,
	})
	return nil
}

// userCore é a projeção comum entre as consultas de usuário ativo.
type userCore struct {
	id        int64
	email     string
	status    string
	firstName string
	lastName  string
	disabled  pgtype.Timestamptz
	createdAt pgtype.Timestamptz
	updatedAt pgtype.Timestamptz
}

func coreFromUsernameRow(row gen.GetUserByEmailRow) userCore {
	return userCore{
		id: row.ID, email: row.Email, status: row.Status,
		firstName: row.FirstName, lastName: row.LastName,
		disabled: row.DisabledAt, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt,
	}
}

func coreFromIDRow(row gen.GetUserByIDRow) userCore {
	return userCore{
		id: row.ID, email: row.Email, status: row.Status,
		firstName: row.FirstName, lastName: row.LastName,
		disabled: row.DisabledAt, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt,
	}
}

// userFromRow carrega os cargos e converte a projeção em domain.User.
func userFromRow(ctx context.Context, s *store.Store, c userCore) (domain.User, error) {
	roleStrings, err := s.Q.GetUserRoles(ctx, c.id)
	if err != nil {
		return domain.User{}, err
	}
	roles := make([]domain.Role, 0, len(roleStrings))
	for _, role := range roleStrings {
		roles = append(roles, domain.Role(role))
	}
	u := domain.User{
		ID:        c.id,
		Email:     c.email,
		FirstName: c.firstName,
		LastName:  c.lastName,
		Status:    c.status,
		Roles:     roles,
		CreatedAt: c.createdAt.Time,
	}
	if c.disabled.Valid {
		u.DisabledAt = &c.disabled.Time
	}
	if c.updatedAt.Valid {
		u.UpdatedAt = &c.updatedAt.Time
	}
	return u, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func invalidCredentials() error {
	return platform.NewAPIError(401, ErrCodeInvalidCredentials, "Credenciais inválidas.")
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
