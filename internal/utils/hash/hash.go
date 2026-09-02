package hash

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256 retorna o hash SHA-256 em hexadecimal da string informada.
func SHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
