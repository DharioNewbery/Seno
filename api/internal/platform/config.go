package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config carrega toda a configuração da API a partir de variáveis de
// ambiente. Nenhum segredo vive no código: tudo vem do ambiente (ver
// deploy/.env.example).
type Config struct {
	Addr             string        // endereço HTTP escutado
	DatabaseURL      string        // DSN do PostgreSQL
	SessionTTL       time.Duration // janela sliding da sessão
	SessionMaxTTL    time.Duration // teto de vida total da sessão
	SuperEmail       string        // e-mail do SUPER admin (bootstrap)
	SuperPassword    string        // senha do SUPER admin (bootstrap)
	MailDev          bool          // MAIL_DEV: captura de e-mails no Log local
	SMTPHost         string
	SMTPPort         int
	SMTPUser         string
	SMTPPassword     string
	SMTPFrom         string
	WebOrigin        string // origem válida para verificação de Origin no BFF/API
	ShutdownDeadline time.Duration

	// Judge0 (§Execução do código): vazio = integração desligada
	// (correções pendentes p/ grade manual).
	Judge0URL    string
	Judge0Token  string // X-Auth-Token (se exigido pela instância)
	Judge0Secret string // chave compartilhada do callback (?key=)
}

// LoadConfig lê o ambiente e devolve a configuração validada.
func LoadConfig() (*Config, error) {
	c := &Config{
		Addr:             env("SENO_ADDR", ":8080"),
		SessionTTL:       envDuration("SENO_SESSION_TTL", 12*time.Hour),
		SessionMaxTTL:    envDuration("SENO_SESSION_MAX_TTL", 7*24*time.Hour),
		SuperEmail:       strings.ToLower(strings.TrimSpace(os.Getenv("SENO_SUPER_EMAIL"))),
		SuperPassword:    os.Getenv("SENO_SUPER_PASSWORD"),
		MailDev:          envBool("MAIL_DEV", false),
		SMTPHost:         os.Getenv("SENO_SMTP_HOST"),
		SMTPPort:         envInt("SENO_SMTP_PORT", 587),
		SMTPUser:         os.Getenv("SENO_SMTP_USER"),
		SMTPPassword:     os.Getenv("SENO_SMTP_PASSWORD"),
		SMTPFrom:         env("SENO_SMTP_FROM", "seno@ufpa.br"),
		WebOrigin:        os.Getenv("SENO_WEB_ORIGIN"),
		ShutdownDeadline: envDuration("SENO_SHUTDOWN_DEADLINE", 10*time.Second),
		Judge0URL:        os.Getenv("SENO_JUDGE0_URL"),
		Judge0Token:      os.Getenv("SENO_JUDGE0_TOKEN"),
		Judge0Secret:     os.Getenv("SENO_JUDGE0_CALLBACK_KEY"),
	}

	c.DatabaseURL = os.Getenv("SENO_DATABASE_URL")
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("SENO_DATABASE_URL é obrigatório")
	}
	if c.SuperEmail == "" || c.SuperPassword == "" {
		return nil, fmt.Errorf("SENO_SUPER_EMAIL e SENO_SUPER_PASSWORD são obrigatórios (bootstrap do super admin)")
	}
	if c.WebOrigin == "" {
		return nil, fmt.Errorf("SENO_WEB_ORIGIN é obrigatório (origem do BFF, ex.: https://seno.ufpa.br)")
	}
	return c, nil
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v == "true" || v == "1" || v == "yes"
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
