// Package auth concentra senhas (Argon2id), sessões opacas, tokens de uso
// único, rate limit de login e o serviço de autenticação (ARQUITETURA §5).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parâmetros do perfil OWASP para Argon2id (ARQUITETURA §5.1):
// memória ≈ 19 MiB, 2 iterações, paralelismo 1.
const (
	argonMemory  uint32 = 19 * 1024 // KiB
	argonTime    uint32 = 2
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen uint32 = 16
)

// HashPassword deriva o hash Argon2id no formato PHC.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("gerando salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword confere a senha contra o hash PHC armazenado.
// Retorna false com erro para hash corrompido (trate como credencial inválida).
func VerifyPassword(hash, password string) (bool, error) {
	if !strings.HasPrefix(hash, "$argon2id$") {
		return false, fmt.Errorf("formato de hash não suportado")
	}
	parts := strings.Split(hash, "$")
	// ["", "argon2id", "v=19", "m=...", "salt", "key"]
	if len(parts) != 6 {
		return false, fmt.Errorf("hash PHC malformado")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("versão do hash ausente")
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, fmt.Errorf("parâmetros do hash ausentes")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("salt inválido")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("chave inválida")
	}
	derived := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(key)))
	if version != argon2.Version {
		return false, fmt.Errorf("versão do argon2 incompatível: %s", strconv.Itoa(version))
	}
	return constantTimeEqual(derived, key), nil
}

func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// NewToken gera um token opaco de 32 bytes (CSPRNG) em base64url
// e devolve também o hash SHA-256 a armazenar no banco (ARQUITETURA §5.2).
func NewToken() (token string, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("gerando token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	tokenHash = TokenHash(token)
	return token, tokenHash, nil
}

// TokenHash calcula o SHA-256 (hex) do token recebido.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
