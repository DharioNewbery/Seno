package platform

import "testing"

// TestLoadConfigJudge0ExigeChaveECallback — a integração Judge0 ligada
// sem chave aceita vereditos forjados no callback, e sem base própria
// apontaria o callback para o BFF (nunca alcançado). Valores obrigatórios.
func TestLoadConfigJudge0ExigeChaveECallback(t *testing.T) {
	resetar := func() {
		t.Setenv("SENO_ADDR", "")
		t.Setenv("SENO_DATABASE_URL", "postgres://x")
		t.Setenv("SENO_SUPER_EMAIL", "super@seno.dev")
		t.Setenv("SENO_SUPER_PASSWORD", "x")
		t.Setenv("SENO_WEB_ORIGIN", "http://localhost:5173")
		t.Setenv("SENO_JUDGE0_URL", "")
		t.Setenv("SENO_JUDGE0_CALLBACK_KEY", "")
		t.Setenv("SENO_JUDGE0_CALLBACK_URL", "")
	}

	// Judge0 desligado: nada é exigido.
	resetar()
	if _, err := LoadConfig(); err != nil {
		t.Fatalf("sem Judge0 não pode falhar: %v", err)
	}

	// Ligado sem chave e sem callback: os dois bloqueiam o boot.
	resetar()
	t.Setenv("SENO_JUDGE0_URL", "http://judge0:2358")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("Judge0 ligado sem chave deve falhar")
	}
	resetar()
	t.Setenv("SENO_JUDGE0_URL", "http://judge0:2358")
	t.Setenv("SENO_JUDGE0_CALLBACK_KEY", "chave")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("Judge0 ligado sem callback URL deve falhar")
	}

	// Ligado completo: passa.
	resetar()
	t.Setenv("SENO_JUDGE0_URL", "http://judge0:2358")
	t.Setenv("SENO_JUDGE0_CALLBACK_KEY", "chave")
	t.Setenv("SENO_JUDGE0_CALLBACK_URL", "http://api:8080")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("config completa falhou: %v", err)
	}
	if cfg.Judge0CallbackURL != "http://api:8080" {
		t.Errorf("callback URL: %q", cfg.Judge0CallbackURL)
	}
}
