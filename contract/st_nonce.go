package contract

import (
	"crypto/rand"
	"errors"
)

// STNonce devolve um nonce aleatório positivo (jti) para o emissor de `st`.
// 6 bytes CSPRNG = 2^48 de espaço.
func STNonce() (int64, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, errors.New("contract: rand: " + err.Error())
	}
	return int64(b[0])<<40 | int64(b[1])<<32 | int64(b[2])<<24 |
		int64(b[3])<<16 | int64(b[4])<<8 | int64(b[5]), nil
}
