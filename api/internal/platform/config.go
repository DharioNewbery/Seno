package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/seno-project/seno/contract"
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
	Judge0URL         string
	Judge0Token       string // X-Auth-Token (se exigido pela instância)
	Judge0Secret      string // chave compartilhada do callback (?key=)
	Judge0CallbackURL string // base da API VISTA PELO JUDGE0 (ex.: http://api:8080)

	// Monitor (Fase 9 do PLANO_MONITOR): vazio = integração desligada.
	MonitorURL       string        // base do Monitor vista pela API (ex.: http://monitor:8081)
	MonitorPublicURL string        // base do Monitor vista pelo NAVEGADOR (redirect do BFF)
	MonitorSecret    string        // HMAC compartilhado (rotas internas + token st)
	PollEstado       time.Duration // intervalo do job que consome GET /interna/.../estado
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
		Judge0URL:         os.Getenv("SENO_JUDGE0_URL"),
		Judge0Token:       os.Getenv("SENO_JUDGE0_TOKEN"),
		Judge0Secret:      os.Getenv("SENO_JUDGE0_CALLBACK_KEY"),
		Judge0CallbackURL: strings.TrimSuffix(strings.TrimSpace(os.Getenv("SENO_JUDGE0_CALLBACK_URL")), "/"),
		MonitorURL:       strings.TrimSuffix(strings.TrimSpace(os.Getenv("SENO_MONITOR_URL")), "/"),
		MonitorPublicURL: strings.TrimSuffix(strings.TrimSpace(os.Getenv("SENO_MONITOR_PUBLIC_URL")), "/"),
		MonitorSecret:    os.Getenv("SENO_MONITOR_SECRET"),
		PollEstado:       envDuration("SENO_POLL_ESTADO_S", 30*time.Second),
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
	if (c.MonitorURL != "" || c.MonitorPublicURL != "") && len(c.MonitorSecret) < contract.HMACMinSecretLen {
		return nil, fmt.Errorf("integração com o Monitor exige SENO_MONITOR_SECRET de pelo menos %d bytes", contract.HMACMinSecretLen)
	}
	if (c.MonitorURL == "") != (c.MonitorPublicURL == "") {
		return nil, fmt.Errorf("SENO_MONITOR_URL e SENO_MONITOR_PUBLIC_URL são configurados juntos (interno e navegador)")
	}
	if c.Judge0URL != "" {
		// Sem chave, o callback do Judge0 aceita vereditos forjados de
		// qualquer host da rede (nota cheia sem prova de execução).
		if c.Judge0Secret == "" {
			return nil, fmt.Errorf("Judge0 ligado exige SENO_JUDGE0_CALLBACK_KEY (valida o POST /v1/judge0/callback)")
		}
		// O callback_url é construído sobre esta base; a origem do navegador
		// (SENO_WEB_ORIGIN) não é alcançável pelo Judge0 na rede interna.
		if c.Judge0CallbackURL == "" {
			return nil, fmt.Errorf("Judge0 ligado exige SENO_JUDGE0_CALLBACK_URL (base da API vista pelo Judge0, ex.: http://api:8080)")
		}
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
