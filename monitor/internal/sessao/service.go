// Service de sessão e entrada por SSO do Monitor (spec §3).
//
// Fluxo: API emite `st` assinado → Monitor valida (assinatura/exp/aud no
// contract; mid/sub contra o banco) → cria sessão própria (cookie hm_sess,
// token 32 B CSPRNG, banco guarda apenas SHA-256). O Monitor nunca valida
// matrícula viva na API — a autoridade dele é o snapshot atividade_aluno +
// janelas recalculadas no servidor.
package sessao

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
)

const (
	CookieNome = "hm_sess"

	// PousoExtra é o acréscimo do spec §3 ao TTL: fim da janela do aluno
	// (fim_real) + 10 min. Máximo absoluto do cookie.
	PousoExtra = 10 * time.Minute

	tamanhoToken = 32 // bytes CSPRNG
)

// hashDe é SHA-256 hex (padrão da casa: banco nunca guarda o token cru).
func hashDe(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Erros que o handler mapeia para status HTTP.
var (
	ErrAtividadeFora = fmt.Errorf("atividade não está ativa")
	ErrAlunoFora     = fmt.Errorf("aluno não listado na atividade")
	ErrJti           = fmt.Errorf("st já consumido")
)

// Service concede sessões a alunos com `st` válido.
type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Entrar valida mid/sub contra o banco e cria a sessão. Devolve o
// token do cookie e a expiração (para Set-Cookie com MaxAge coerente).
func (s *Service) Entrar(ctx context.Context, c *contract.STClaims) (token string, expira time.Duration, err error) {
	now := time.Now()

	var status string
	if err := s.pool.QueryRow(ctx,
		`SELECT status FROM atividade WHERE monitor_id = $1`, c.Mid,
	).Scan(&status); err != nil {
		return "", 0, fmt.Errorf("sessao: atividade %s não encontrada", c.Mid)
	}
	if status != "ativa" {
		return "", 0, fmt.Errorf("sessao: %w (%s)", ErrAtividadeFora, status)
	}

	var listado bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM atividade_aluno
		   WHERE monitor_id = $1 AND aluno_api_id = $2)`, c.Mid, c.Sub,
	).Scan(&listado); err != nil {
		return "", 0, fmt.Errorf("sessao: verificando matrícula: %w", err)
	}
	if !listado {
		return "", 0, fmt.Errorf("sessao: %w (aluno %d)", ErrAlunoFora, c.Sub)
	}

	// Limpeza de consumo/sessões vencidos (barato; roda por entrada).
	_, _ = s.pool.Exec(ctx,
		`DELETE FROM sso_consumo WHERE expira_em < now();
		 DELETE FROM sessao   WHERE expira_em < now();`)

	// jti consumido? INSERT idempotente; conflito = replay do st.
	stHash := hashDe(fmt.Sprintf("%s:%d:%d", c.Mid, c.Sub, c.Jti))
	ctag, err := s.pool.Exec(ctx,
		`INSERT INTO sso_consumo (token_hash, expira_em) VALUES ($1, $2)
		 ON CONFLICT (token_hash) DO NOTHING`,
		stHash, time.Unix(c.Exp, 0))
	if err != nil {
		return "", 0, fmt.Errorf("sessao: registrando st: %w", err)
	}
	if ctag.RowsAffected() == 0 {
		return "", 0, ErrJti
	}

	// Token de sessão: 32 B CSPRNG; banco guarda só o SHA-256.
	b := make([]byte, tamanhoToken)
	if _, err := rand.Read(b); err != nil {
		return "", 0, fmt.Errorf("sessao: rand: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)

	// TTL = tempo restante da janela do aluno (recalculado server-side) + 10 min.
	expira = s.tempoJanelaRecalculada(ctx, c.Mid, now) + PousoExtra
	if expira < PousoExtra {
		expira = PousoExtra
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO sessao (token_hash, aluno_api_id, monitor_id, expira_em)
		 VALUES ($1, $2, $3, $4)`,
		hashDe(token), c.Sub, c.Mid, now.Add(expira)); err != nil {
		return "", 0, fmt.Errorf("sessao: gravando sessão: %w", err)
	}

	return token, expira, nil
}

// tempoJanelaRecalculada calcula fim_real = min(prazo, inicio + duração)
// a partir da carga da atividade e devolve quanto falta da janela.
// Encerrada devolve <= 0 (o MaxAge é então só o pouso extra).
func (s *Service) tempoJanelaRecalculada(ctx context.Context, mid string, now time.Time) time.Duration {
	var janelaJSON []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT carga->'janela' FROM atividade WHERE monitor_id = $1`, mid,
	).Scan(&janelaJSON); err != nil {
		return 0
	}
	var j contract.Janela
	if err := json.Unmarshal(janelaJSON, &j); err != nil {
		return 0
	}
	fim := j.Prazo
	if j.DuracaoSeg != nil {
		dur := j.Inicio.Add(time.Duration(*j.DuracaoSeg) * time.Second)
		if dur.Before(fim) {
			fim = dur
		}
	}
	return fim.Sub(now)
}
