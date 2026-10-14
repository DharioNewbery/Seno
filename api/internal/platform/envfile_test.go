package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAplicaDotEnv(t *testing.T) {
	conteudo := "# comentário\r\n" +
		"SENO_SUPER_PASSWORD=senha-do-teste\r\n" +
		"\r\n" +
		"linha sem igual\r\n" +
		"POSTGRES_PASSWORD=ignorada\r\n" +
		"MAIL_DEV=true\r\n"
	aplicaDotEnv([]byte(conteudo))
	defer os.Unsetenv("SENO_SUPER_PASSWORD")
	defer os.Unsetenv("MAIL_DEV")
	defer os.Unsetenv("POSTGRES_PASSWORD")

	if os.Getenv("SENO_SUPER_PASSWORD") != "senha-do-teste" {
		t.Errorf("valor aplicado: %q", os.Getenv("SENO_SUPER_PASSWORD"))
	}
	if os.Getenv("MAIL_DEV") != "true" {
		t.Errorf("MAIL_DEV aplicado: %q", os.Getenv("MAIL_DEV"))
	}
	if v, ok := os.LookupEnv("POSTGRES_PASSWORD"); ok {
		t.Errorf("chave fora do domínio do Seno não pode ser aplicada: %q", v)
	}

	// já existente no ambiente: vence o arquivo (compose/CI sobrepõem).
	t.Setenv("SENO_SUPER_PASSWORD", "do-ambiente")
	aplicaDotEnv([]byte("SENO_SUPER_PASSWORD=do-arquivo\n"))
	defer os.Unsetenv("SENO_SUPER_PASSWORD")
	if os.Getenv("SENO_SUPER_PASSWORD") != "do-ambiente" {
		t.Errorf("ambiente deveria vencer: %q", os.Getenv("SENO_SUPER_PASSWORD"))
	}

	// LoadDotEnv não explode fora do monorepo (sem deploy/.env na árvore).
	tmp := t.TempDir()
	t.Chdir(tmp)
	LoadDotEnv() // no-op silencioso; só garante que não caçou nada acima
	if _, ok := os.LookupEnv("SENO_INEXISTENTE"); ok {
		t.Error("nada deveria ser aplicado fora da árvore do monorepo")
	}

	// e DENTRO: descobre deploy/.env na raiz acima do cwd
	raiz, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Skipf("caminho da raiz: %v", err)
	}
	if _, err := os.Stat(filepath.Join(raiz, "deploy", ".env")); err != nil {
		t.Skipf("deploy/.env ausente no checkout: %v", err)
	}
	resgrade, ok := os.LookupEnv("SENO_SUPER_EMAIL")
	_ = resgrade
	if ok {
		os.Unsetenv("SENO_SUPER_EMAIL") // zera p/ provar que vem do arquivo
	}
	t.Chdir(filepath.Join(raiz, "api"))
	LoadDotEnv()
	if _, ok := os.LookupEnv("SENO_SUPER_EMAIL"); !ok {
		t.Error("LoadDotEnv deveria preencher SENO_SUPER_EMAIL a partir da raiz")
	}
	if strings.TrimSpace(os.Getenv("SENO_SUPER_EMAIL")) == "" {
		t.Error("valor vazio do .env não deveria ser aplicado de outra forma")
	}
}
