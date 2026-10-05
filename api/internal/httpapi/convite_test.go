package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

func TestNucleoConvite(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)
	ctx := context.Background()
	st := store.New(pool)
	rec := deps.Cargo

	// Emissão para e-mail inexistente: cria pendente e envia link (Log).
	if err := rec.Invite(ctx, "  Convidado@Seno.dev ", "Nina", "Souza", nil); err != nil {
		t.Fatalf("invite: %v", err)
	}
	token := tokenDoLog(t, pool)

	// Usuário pendente não entra: login genérico 401 (anti-enumeração,
	// service.go) — não revela se o e-mail existe ou está pendente.
	w, body := chama(t, r, "POST", "/v1/auth/login", `{"email":"convidado@seno.dev","senha":"x"}`)
	if w.Code != http.StatusUnauthorized || falhaDoEnvelope(t, body) != platform.CodeUnauthorized {
		t.Fatalf("pendente deveria dar 401 genérico: status %d corpo %v", w.Code, body)
	}

	// Reenvio invalida o token anterior: antigo morre, novo é o válido.
	if err := rec.Invite(ctx, "convidado@seno.dev", "Nina", "Souza", nil); err != nil {
		t.Fatalf("reenvio: %v", err)
	}
	w, body = chama(t, r, "POST", "/v1/auth/invite/accept", `{"token":"`+token+`","senha":"senha-valida-1"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("convite antigo deveria estar invalidado: status %d corpo %v", w.Code, body)
	}
	token = tokenDoLog(t, pool)

	// Senha fraca → 422; token inexistente → 404.
	w, body = chama(t, r, "POST", "/v1/auth/invite/accept", `{"token":"`+token+`","senha":"curta"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("senha fraca: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "POST", "/v1/auth/invite/accept", `{"token":"invalido","senha":"senha-valida-1"}`)
	if w.Code != http.StatusNotFound || falhaDoEnvelope(t, body) != "NAO_ENCONTRADO" {
		t.Errorf("token inválido: status %d corpo %v", w.Code, body)
	}

	// Aceite correto → 200; pendente vira ativo e loga com a nova senha.
	w, body = chama(t, r, "POST", "/v1/auth/invite/accept", `{"token":"`+token+`","senha":"senha-valida-1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("aceite: status %d corpo %v", w.Code, body)
	}
	w, body = chama(t, r, "POST", "/v1/auth/login", `{"email":"convidado@seno.dev","senha":"senha-valida-1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login após aceite: status %d corpo %v", w.Code, body)
	}
	if pessoa, _ := body["pessoa"].(map[string]any); pessoa["first_name"] != "Nina" {
		t.Errorf("pessoa inesperada: %v", pessoa)
	}

	// Token é de uso único.
	w, _ = chama(t, r, "POST", "/v1/auth/invite/accept", `{"token":"`+token+`","senha":"outra-senha-9"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("reuso do convite: status %d", w.Code)
	}

	// Emissão para usuário ativo → 409 CONFLITO_DE_ESTADO (serviço).
	var apiErr *platform.APIError
	err := rec.Invite(ctx, "convidado@seno.dev", "Nina", "Souza", nil)
	if err == nil || !errors.As(err, &apiErr) ||
		apiErr.Code != platform.CodeConflict || apiErr.Status != http.StatusConflict {
		t.Errorf("emissão para ativo: esperado 409 CONFLITO_DE_ESTADO, obtido %v", err)
	}

	// Emissão para conta bloqueada → 403 SEM_PERMISSAO.
	if err := criarBloqueado(ctx, st, "bloqueado@seno.dev"); err != nil {
		t.Fatalf("criar bloqueado: %v", err)
	}
	err = rec.Invite(ctx, "bloqueado@seno.dev", "Bia", "Lima", nil)
	if err == nil || !errors.As(err, &apiErr) ||
		apiErr.Code != platform.CodeForbidden || apiErr.Status != http.StatusForbidden {
		t.Errorf("emissão para bloqueado: esperado 403 SEM_PERMISSAO, obtido %v", err)
	}

	// O núcleo de convite é auditado: emissão e aceite presentes no Log.
	var kinds int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM log_entries
		WHERE kind IN ('auth.invite_sent','auth.invite_accepted')`).Scan(&kinds); err != nil {
		t.Fatalf("log: %v", err)
	}
	if kinds < 2 {
		t.Errorf("eventos de convite ausentes no Log: %d", kinds)
	}
}

// criarBloqueado cadastra um usuário e o desativa logo em seguida
// (CHECK: disabled exige disabled_at, garantido pelo DisableUser).
func criarBloqueado(ctx context.Context, st *store.Store, email string) error {
	return st.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		personID, err := q.InsertPerson(ctx, gen.InsertPersonParams{
			FirstName: "Bia", LastName: "Lima",
		})
		if err != nil {
			return err
		}
		user, err := q.InsertUser(ctx, gen.InsertUserParams{
			PersonID: personID,
			Email:    email,
			Status:   domain.StatusActive,
		})
		if err != nil {
			return err
		}
		return q.DisableUser(ctx, gen.DisableUserParams{ID: user.ID})
	})
}
