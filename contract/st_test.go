package contract

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const stAud = SSOAud

func TestSTRoundTrip(t *testing.T) {
	tok, err := AssinarST([]byte(seg), time.Now(), time.Minute, SSOIss, stAud, 12, "atv_abc", 1)
	if err != nil {
		t.Fatalf("AssinarST: %v", err)
	}
	c, err := VerificarST([]byte(seg), tok, stAud, time.Now())
	if err != nil {
		t.Fatalf("token válido rejeitado: %v", err)
	}
	if c.Sub != 12 || c.Mid != "atv_abc" || c.Iss != SSOIss || c.Aud != stAud {
		t.Fatalf("claims se perderam: %+v", c)
	}
}

func TestSTExpirado(t *testing.T) {
	// Token montado à mão com exp no passado (AssinarST recusa ttl negativo).
	c := STClaims{Alg: SSOAlg, Iss: SSOIss, Aud: stAud, Sub: 12, Mid: "atv_abc",
		Exp: time.Now().Add(-time.Second).Unix(), Jti: 1}
	payload, _ := json.Marshal(c)
	mac := hmac.New(sha256.New, []byte(seg))
	mac.Write(payload)
	tok := base64.RawURLEncoding.EncodeToString(payload) + "." + hex.EncodeToString(mac.Sum(nil))
	if _, err := VerificarST([]byte(seg), tok, stAud, time.Now()); !errors.Is(err, ErrSTExpirado) {
		t.Fatalf("esperava ErrSTExpirado, veio: %v", err)
	}
}

func TestSTAudDiferente(t *testing.T) {
	tok, err := AssinarST([]byte(seg), time.Now(), time.Minute, SSOIss, "outro-servico", 12, "atv_abc", 1)
	if err != nil {
		t.Fatalf("AssinarST: %v", err)
	}
	if _, err := VerificarST([]byte(seg), tok, stAud, time.Now()); !errors.Is(err, ErrSTAud) {
		t.Fatalf("esperava ErrSTAud, veio: %v", err)
	}
}

func TestSTSegredoDiferente(t *testing.T) {
	tok, err := AssinarST([]byte("outro-segredo-52-bytes-hmmmm-0123456789"), time.Now(), time.Minute, SSOIss, stAud, 12, "atv_abc", 1)
	if err != nil {
		t.Fatalf("AssinarST: %v", err)
	}
	if _, err := VerificarST([]byte(seg), tok, stAud, time.Now()); !errors.Is(err, ErrSTInvalido) {
		t.Fatalf("esperava ErrSTInvalido, veio: %v", err)
	}
}

func TestSTCorpoAlterado(t *testing.T) {
	tok, err := AssinarST([]byte(seg), time.Now(), time.Minute, SSOIss, stAud, 12, "atv_abc", 1)
	if err != nil {
		t.Fatalf("AssinarST: %v", err)
	}
	// troca de aluno no payload: sub 12 -> 13, sem re-assinar: quebra o MAC
	payload := strings.Split(tok, ".")[0]
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	trocado := strings.Replace(string(raw), `"sub":12`, `"sub":13`, 1)
	if strings.Contains(trocado, `"sub":12`) {
		t.Fatalf("payload não foi alterado no teste")
	}
	novo := base64.RawURLEncoding.EncodeToString([]byte(trocado)) + "." + strings.Split(tok, ".")[1]
	if _, err := VerificarST([]byte(seg), novo, stAud, time.Now()); !errors.Is(err, ErrSTInvalido) {
		t.Fatalf("esperava ErrSTInvalido, veio: %v", err)
	}
}

func TestSTMalFormado(t *testing.T) {
	if _, err := VerificarST([]byte(seg), "não-é-token", stAud, time.Now()); !errors.Is(err, ErrSTInvalido) {
		t.Fatalf("esperava ErrSTInvalido, veio: %v", err)
	}
	if _, err := VerificarST([]byte(seg), "", stAud, time.Now()); !errors.Is(err, ErrSTInvalido) {
		t.Fatalf("esperava ErrSTInvalido, veio: %v", err)
	}
}

func TestSTNonce(t *testing.T) {
	n1, err := STNonce()
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	n2, _ := STNonce()
	if n1 <= 0 || n2 <= 0 || n1 == n2 {
		t.Fatalf("nonce inválido/repetido: %d %d", n1, n2)
	}
}

func TestSTCamposObrigatorios(t *testing.T) {
	if _, err := AssinarST([]byte(seg), time.Now(), 0, SSOIss, stAud, 12, "atv", 1); err == nil {
		t.Fatal("ttl zero deveria ser recusado")
	}
	if _, err := AssinarST([]byte(seg), time.Now(), time.Minute, SSOIss, stAud, 0, "atv", 1); err == nil {
		t.Fatal("sub zero deveria ser recusado")
	}
	if _, err := AssinarST([]byte(seg), time.Now(), time.Minute, SSOIss, stAud, 12, "", 1); err == nil {
		t.Fatal("mid vazio deveria ser recusado")
	}
	if _, err := AssinarST([]byte(seg), time.Now(), time.Minute, SSOIss, stAud, 12, "atv", 0); err == nil {
		t.Fatal("jti zero deveria ser recusado")
	}
}
