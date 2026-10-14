// Package platform — leitura de deploy/.env para o dev nativo: Go não lê
// .env (os.Getenv lê apenas variáveis do processo) e no método B o compose
// injeta tudo via env_file — por isso a busca é silenciosa quando o
// arquivo não existe (imagem distroless não o contém) e só PREENCHE
// variáveis ausentes (o que o compose/chamador setou sempre vence).
package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv procura deploy/.env à partir do diretório corrente (até 6
// níveis acima, cobre desde api/, monitor/ ou a raiz) e aplica primeira
// ocorrência vence. Falta do arquivo: no-op.
func LoadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 6; i++ {
		if dados, err := os.ReadFile(filepath.Join(dir, "deploy", ".env")); err == nil {
			aplicaDotEnv(dados)
			return
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			return
		}
		dir = pai
	}
}

// aplicaDotEnv só insere chaves das configurações do Seno (SENO_*/MAIL_DEV)
// e apenas quando ausentes — variáveis do ambiente já setadas vencem.
func aplicaDotEnv(dados []byte) {
	for _, linha := range strings.Split(string(dados), "\n") {
		linha = strings.TrimSuffix(linha, "\r")
		linha = strings.TrimSpace(linha)
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		chave, valor, ok := strings.Cut(linha, "=")
		if !ok {
			continue
		}
		chave = strings.TrimSpace(chave)
		if !strings.HasPrefix(chave, "SENO_") && chave != "MAIL_DEV" {
			continue
		}
		if _, existe := os.LookupEnv(chave); !existe {
			_ = os.Setenv(chave, strings.TrimSpace(valor))
		}
	}
}
