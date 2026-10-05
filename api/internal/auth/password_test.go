package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHashPasswordRoundtrip(t *testing.T) {
	hash, err := HashPassword("senha-segura-123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword(hash, "senha-segura-123")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("senha correta não verificou")
	}
	ok, _ = VerifyPassword(hash, "senha-errada")
	if ok {
		t.Error("senha errada verificou")
	}
}

func TestHashPasswordSaltUnico(t *testing.T) {
	h1, err := HashPassword("mesma-senha")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	h2, err := HashPassword("mesma-senha")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if h1 == h2 {
		t.Error("hashes idênticos: salt não é aleatório")
	}
}

func TestVerifyMalformedHash(t *testing.T) {
	if _, err := VerifyPassword("não-é-phc", "qualquer"); err == nil {
		t.Error("hash malformado deveria retornar erro")
	}
}

func TestNewToken(t *testing.T) {
	t1, h1, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	t2, h2, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if t1 == t2 || h1 == h2 {
		t.Error("tokens/hashes repetidos")
	}
	want := sha256.Sum256([]byte(t1))
	if got := TokenHash(t1); got != hex.EncodeToString(want[:]) {
		t.Errorf("TokenHash != SHA-256 do token: %s", got)
	}
	if TokenHash(t1) != h1 {
		t.Error("NewToken/TokenHash divergentes")
	}
}
