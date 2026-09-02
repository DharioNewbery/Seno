package password

import "golang.org/x/crypto/bcrypt"

// Hash gera um hash bcrypt da senha em texto plano.
func Hash(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Compare valida a senha em texto plano contra o hash armazenado.
func Compare(hashed, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
}
