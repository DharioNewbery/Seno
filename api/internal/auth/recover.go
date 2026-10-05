package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/mail"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// Tipos de token de uso único (§5.3) e eventos de Log.
const (
	TokenKindPasswordReset = "password_reset"
	TokenKindInvite        = "invite"

	LogKindPasswordResetRequested = "auth.password_reset_requested"
	LogKindPasswordResetDone      = "auth.password_reset_done"
)

// Recoverer gerencia convite e recuperação de senha por e-mail.
type Recoverer struct {
	store      *store.Store
	audit      *platform.Audit
	sender     mail.Sender
	fallback   *mail.LogSender
	webOrigin  string
	tokenTTL   time.Duration
	sendTimout time.Duration
}

// NewRecoverer monta o fluxo de recuperação.
func NewRecoverer(s *store.Store, audit *platform.Audit, sender mail.Sender, webOrigin string) *Recoverer {
	return &Recoverer{
		store:      s,
		audit:      audit,
		sender:     sender,
		fallback:   &mail.LogSender{Audit: audit},
		webOrigin:  webOrigin,
		tokenTTL:   time.Hour,
		sendTimout: 10 * time.Second,
	}
}

// RequestPasswordReset gera token de 1h (uso único) e envia o link por
// e-mail. Resposta é sempre silenciosa (sem enumeração de contas).
func (r *Recoverer) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)

	row, err := r.store.Q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // silencioso
		}
		return err
	}
	if row.Status == "disabled" || row.DisabledAt.Valid {
		return nil // conta bloqueada: não envia
	}

	r.audit.Record(ctx, platform.LogEntry{Actor: email, Kind: LogKindPasswordResetRequested})

	// Invalida tokens anteriores do usuário (uso único vigente).
	if _, err := r.store.Q.InvalidateTokens(ctx, gen.InvalidateTokensParams{
		UserID: row.ID, Kind: TokenKindPasswordReset,
	}); err != nil {
		return err
	}
	token, tokenHash, err := NewToken()
	if err != nil {
		return err
	}
	exp := time.Now().Add(r.tokenTTL)
	var createdBy pgtype.Int8
	createdBy.Int64 = row.ID
	createdBy.Valid = true
	if err := r.store.Q.InsertOneTimeToken(ctx, gen.InsertOneTimeTokenParams{
		UserID: row.ID, Kind: TokenKindPasswordReset, TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: exp, Valid: true}, CreatedBy: createdBy,
	}); err != nil {
		return err
	}

	link := fmt.Sprintf("%s/reset-password?token=%s", r.webOrigin, token)
	msg := mail.Message{
		To:      email,
		Subject: "Seno — redefinição de senha",
		Text: fmt.Sprintf(
			"Você solicitou a redefinição de senha da sua conta no Seno.\n\n"+
				"Use o link abaixo (válido por 1 hora, uso único):\n%s\n\n"+
				"Se você não solicitou, ignore este e-mail.", link),
	}
	return mail.SendWithFallback(ctx, r.sender, r.fallback, msg, r.sendTimout)
}

// ResetPassword troca a senha pelo token e revoga todas as sessões.
func (r *Recoverer) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < MinPasswordLength {
		return platform.NewAPIError(422, platform.CodeUnprocessable,
			fmt.Sprintf("A senha deve ter pelo menos %d caracteres.", MinPasswordLength))
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Uso atômico do token (UPDATE com guard) já devolve o dono.
	tokenHash := TokenHash(token)
	userID, err := r.store.Q.ConsumeToken(ctx, gen.ConsumeTokenParams{
		TokenHash: tokenHash, Kind: TokenKindPasswordReset,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Token inválido ou expirado.")
		}
		return err
	}

	userRow, err := r.store.Q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if userRow.Status == "disabled" || userRow.DisabledAt.Valid {
		return platform.NewAPIError(403, platform.CodeForbidden, "Conta bloqueada.")
	}
	if err := r.store.Q.UpdateUserPassword(ctx, gen.UpdateUserPasswordParams{
		ID: userID, PasswordHash: &newHash,
	}); err != nil {
		return err
	}
	if userRow.Status == "pending" {
		if err := r.store.Q.ActivateUser(ctx, userID); err != nil {
			return err
		}
	}
	if _, err := r.store.Q.DeleteAllUserSessions(ctx, userID); err != nil {
		return err
	}
	r.audit.Record(ctx, platform.LogEntry{
		ActorID: &userID,
		Actor:   normalizeEmail(userRow.Email),
		Kind:    LogKindPasswordResetDone,
	})
	return nil
}
