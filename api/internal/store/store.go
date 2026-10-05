// Package store é a camada de acesso a dados: embrulha as queries
// geradas pelo sqlc e utilitários de transação.
package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/seno-project/seno/api/db/gen"
)

// Store é a fachada única de persistência da API.
type Store struct {
	Pool *pgxpool.Pool
	Q    *gen.Queries
}

// New cria o store sobre o pool do pgx (statements cacheados por ele).
func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool, Q: gen.New(pool)}
}

// InTx executa fn em uma transação, com commit/rollback automáticos.
func (s *Store) InTx(ctx context.Context, fn func(tx pgx.Tx, q *gen.Queries) error) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		return fn(tx, s.Q.WithTx(tx))
	})
}
