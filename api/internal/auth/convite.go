// Package auth — núcleo de convite (ARQUITETURA §5.3): usuário pendente
// criado por admin/professor define a senha por token de uso único que
// expira em 7 dias. A emissão é serviço (sem rota própria): os módulos de
// gestão de usuários/turmas a chamam; o aceite é a rota pública da API.
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
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/mail"
	"github.com/seno-project/seno/api/internal/platform"
)

// Eventos do Log do núcleo de convite (PROJETO §Log).
const (
	LogKindInviteSent     = "auth.invite_sent"
	LogKindInviteAccepted = "auth.invite_accepted"
)

// Invite emite/reemite o convite para o usuário pendente definir a senha.
// E-mail inexistente: cria Pessoa + Usuário pendente em transação.
// Pendente: invalida convites anteriores e emite novo (o último é o válido).
// Ativo: 409 — a recuperação de senha é o fluxo correto. Bloqueada: 403.
func (r *Recoverer) Invite(ctx context.Context, email, firstName, lastName string, createdBy *int64) error {
	email = normalizeEmail(email)

	var userID int64
	row, err := r.store.Q.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		if row.Status == domain.StatusDisabled || row.DisabledAt.Valid {
			return platform.NewAPIError(
				http.StatusForbidden, platform.CodeForbidden, "Conta bloqueada.")
		}
		if row.Status == domain.StatusActive {
			return platform.NewAPIError(
				http.StatusConflict, platform.CodeConflict,
				"Usuário já ativo: use a recuperação de senha.")
		}
		userID = row.ID
	case errors.Is(err, pgx.ErrNoRows):
		var owner pgtype.Int8
		if createdBy != nil {
			owner.Int64 = *createdBy
			owner.Valid = true
		}
		err := r.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
			personID, err := q.InsertPerson(ctx, gen.InsertPersonParams{
				FirstName: firstName,
				LastName:  lastName,
			})
			if err != nil {
				return fmt.Errorf("pessoa do convite: %w", err)
			}
			user, err := q.InsertUser(ctx, gen.InsertUserParams{
				PersonID:  personID,
				Email:     email,
				Status:    domain.StatusPending,
				CreatedBy: owner,
			})
			if err != nil {
				return fmt.Errorf("usuário do convite: %w", err)
			}
			userID = user.ID
			return nil
		})
		if err != nil {
			return fmt.Errorf("convite: %w", err)
		}
	default:
		return err
	}

	r.audit.Record(ctx, platform.LogEntry{
		ActorID: &userID,
		Actor:   email,
		Kind:    LogKindInviteSent,
		Detail:  fmt.Sprintf("validade=%s", r.inviteTTL),
	})

	if _, err := r.store.Q.InvalidateTokens(ctx, gen.InvalidateTokensParams{
		UserID: userID,
		Kind:   TokenKindInvite,
	}); err != nil {
		return err
	}
	token, tokenHash, err := NewToken()
	if err != nil {
		return err
	}
	var owner pgtype.Int8
	if createdBy != nil {
		owner.Int64 = *createdBy
		owner.Valid = true
	}
	if err := r.store.Q.InsertOneTimeToken(ctx, gen.InsertOneTimeTokenParams{
		UserID:    userID,
		Kind:      TokenKindInvite,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(r.inviteTTL), Valid: true},
		CreatedBy: owner,
	}); err != nil {
		return err
	}

	link := fmt.Sprintf("%s/definir-senha?token=%s", r.webOrigin, token)
	msg := mail.Message{
		To:      email,
		Subject: "Seno — defina sua senha",
		Text: fmt.Sprintf(
			"Você foi convidado a acessar o Seno.\n\n"+
				"Defina sua senha de acesso pelo link abaixo (válido por 7 dias, uso único):\n%s\n\n"+
				"Se você não esperava este convite, ignore este e-mail.", link),
	}
	return mail.SendWithFallback(ctx, r.sender, r.fallback, msg, r.sendTimout)
}

// AcceptInvite consome o token de convite, define a senha inicial e ativa
// o usuário pendente (uso único atômico: outro aceite simultâneo falha).
func (r *Recoverer) AcceptInvite(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < MinPasswordLength {
		return platform.NewAPIError(422, platform.CodeUnprocessable,
			fmt.Sprintf("A senha deve ter pelo menos %d caracteres.", MinPasswordLength))
	}
	novoHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	tokenHash := TokenHash(token)
	// Valida antes de consumir para não queimar o token se algo falhar; o
	// consumo atômico fecha a corrida entre dois aceites simultâneos.
	userID, err := r.store.Q.FindTokenUser(ctx, gen.FindTokenUserParams{
		TokenHash: tokenHash, Kind: TokenKindInvite,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound,
				"Token inválido ou expirado.")
		}
		return err
	}
	row, err := r.store.Q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if row.Status == domain.StatusDisabled || row.DisabledAt.Valid {
		return platform.NewAPIError(http.StatusForbidden, platform.CodeForbidden, "Conta bloqueada.")
	}
	if row.Status == domain.StatusActive {
		return platform.NewAPIError(http.StatusConflict, platform.CodeConflict,
			"Usuário já ativo: use a recuperação de senha.")
	}
	if _, err := r.store.Q.ConsumeToken(ctx, gen.ConsumeTokenParams{
		TokenHash: tokenHash, Kind: TokenKindInvite,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound,
				"Token inválido ou expirado.")
		}
		return err
	}
	if err := r.store.Q.UpdateUserPassword(ctx, gen.UpdateUserPasswordParams{
		ID: userID, PasswordHash: &novoHash,
	}); err != nil {
		return err
	}
	if err := r.store.Q.ActivateUser(ctx, userID); err != nil {
		return err
	}
	r.audit.Record(ctx, platform.LogEntry{
		ActorID: &userID,
		Actor:   normalizeEmail(row.Email),
		Kind:    LogKindInviteAccepted,
	})
	return nil
}
