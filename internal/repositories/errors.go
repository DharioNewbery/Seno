package repositories

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUserNotFound         = errors.New("usuário não encontrado")
	ErrCredentialNotFound   = errors.New("credencial não encontrada")
	ErrUserAlreadyExists    = errors.New("usuário já cadastrado com este email ou nome de usuário")
	ErrRoleNotFound         = errors.New("papel não encontrado")
	ErrPermissionNotFound   = errors.New("permissão não encontrada")
	ErrRefreshTokenNotFound = errors.New("token de atualização não encontrado")
	ErrClassNotFound        = errors.New("turma não encontrada")
	// ErrJoinCodeCollision é interno: o service gera outro código e repete.
	ErrJoinCodeCollision = errors.New("colisão de código de turma")
)

// isUniqueViolation verifica se o erro representa uma violação de unicidade (código 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
