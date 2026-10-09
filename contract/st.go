package contract

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Token de SSO da API para o Monitor (spec §3): prova assinada de identidade,
// vida curta, emitido no REDIRECT (`GET /v1/atribuicoes/:id/entrar`) e
// consumido no Monitor (`GET /entrar?st=...`). Formato: b64url(json claims) +
// "." + hex(HMAC-SHA256(secret, json claims)).
//
// O Monitor valida, na ordem do ENTER handler: MAC (tempo const) → tokens
// aceitos (aud) → expiração. Quem assina é a API (produção) ou helper de dev
// monitor-st (`dev`); quem valida neste pacote é o mesmo código do Monitor.

// SSOIss de quem emite (produção: "seno-api").
const (
	SSOIss = "seno-api"
	SSOAud = "seno-monitor" // aud validado no receptor
	SSOAlg = "seno-st-v1"
)

// Erros do token `st`.
var (
	ErrSTInvalido = errors.New("contract: st ausente ou inválido")
	ErrSTExpirado = errors.New("contract: st expirado")
	ErrSTAud      = errors.New("contract: st não é para este serviço (aud)")
)

// STClaims do token.
type STClaims struct {
	Alg string `json:"alg"` // identificador do formato (rejeita outro)
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Sub int64  `json:"sub"` // api_id do aluno
	Mid string `json:"mid"` // monitor_id da atividade
	Exp int64  `json:"exp"` // unix
	Jti int64  `json:"jti"` // nonce (número aleatório) — consumo único
}

// AssinarST devolve o token assinado. Deve ser usado com exp curta e jti único.
func AssinarST(secret []byte, now time.Time, ttl time.Duration, iss, aud string, sub int64, mid string, jti int64) (string, error) {
	if err := VerificarSecret(string(secret)); err != nil {
		return "", err
	}
	if ttl <= 0 || sub == 0 || mid == "" || jti == 0 {
		return "", fmt.Errorf("contract: ttl, sub, mid e jti são obrigatórios")
	}
	payload, err := json.Marshal(STClaims{
		Alg: SSOAlg, Iss: iss, Aud: aud, Sub: sub,
		Mid: mid, Exp: now.Add(ttl).Unix(), Jti: jti,
	})
	if err != nil {
		return "", err
	}
	m := hmac.New(sha256.New, secret)
	m.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + hex.EncodeToString(m.Sum(nil)), nil
}

// VerificarST valida assinatura, formato, aud e exp. NÃO valida iss/mid/sub
// contra repositório — isso é do receptor (Monitor), que conhece seus dados.
func VerificarST(secret []byte, token, audEsperado string, agora time.Time) (STClaims, error) {
	var z STClaims
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return z, ErrSTInvalido
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return z, ErrSTInvalido
	}
	sig, err := hex.DecodeString(parts[1])
	if err != nil || len(sig) != sha256.Size {
		return z, ErrSTInvalido
	}
	m := hmac.New(sha256.New, secret)
	m.Write(payload)
	if !hmac.Equal(sig, m.Sum(nil)) {
		return z, ErrSTInvalido
	}
	var c STClaims
	if err := json.Unmarshal(payload, &c); err != nil || c.Alg != SSOAlg {
		return z, ErrSTInvalido
	}
	if c.Aud != audEsperado {
		return z, ErrSTAud
	}
	if agora.After(time.Unix(c.Exp, 0)) {
		return z, ErrSTExpirado
	}
	return c, nil
}
