// Package db inclui o iofs das migrações (go:embed) e é a raiz do
// código gerado pelo sqlc (ver sqlc.yaml em api/ como referência).
package db

import "embed"

//go:embed all:migrations
var Migrations embed.FS
