package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config carrega toda a configuração do Monitor a partir de variáveis de
// ambiente (spec §7, MONITOR_FLUXO_ALUNO.md). Nenhum segredo vive no código.
type Config struct {
	Addr             string // endereço HTTP escutado
	DatabaseURL      string // DSN do PostgreSQL do Monitor
	Secret           string // HMAC compartilhado com a API (rotas internas + SSO)
	ShutdownDeadline time.Duration

	// Limiares (valores são defaults de ambiente, usados nas fases seguintes).
	MaxPayloadBytes    int           // envelope de transferência
	SnapshotIntervalo  time.Duration // intervalo default de snapshot (rascunho_ponto)
	MaxSnapshotBytes   int           // corpo de autosave
	TokenSSOTTL        time.Duration // vida do token `st`
	PreAvaTTL          time.Duration // janela de escolha após pré-avaliação
	AlunoExecMaxPerMin int           // limite de ensaios por minuto
	ExecucaoRetencao   time.Duration // retenção da tabela execucao
	HMACJanela         time.Duration // tolerância de relógio nas rotas internas (anti-replay)
	CookieSecure       bool          // Secure do hm_sess (produção true; dev HTTP localhost false)

	// Judge0 (Fase 5; placeholder para validar o .env desde o começo).
	Judge0URL    string
	Judge0Token  string // X-Auth-Token (se exigido pela instância)
	Judge0Secret string // segredo do callback
}

// LoadConfig lê o ambiente e devolve a configuração validada.
func LoadConfig() (*Config, error) {
	c := &Config{
		Addr:               env("SENO_MONITOR_ADDR", ":8081"),
		Secret:             os.Getenv("SENO_MONITOR_SECRET"),
		ShutdownDeadline:   envDuration("SENO_SHUTDOWN_DEADLINE", 10*time.Second),
		MaxPayloadBytes:    envInt("SENO_MONITOR_MAX_PAYLOAD_BYTES", 1024*1024),
		SnapshotIntervalo:  envDuration("SENO_SNAPSHOT_INTERVALO_S", 60*time.Second),
		MaxSnapshotBytes:   envInt("SENO_MONITOR_MAX_SNAPSHOT_BYTES", 256*1024),
		TokenSSOTTL:        envDuration("SENO_TOKEN_SSO_TTL_S", 5*time.Minute),
		PreAvaTTL:          envDuration("SENO_PRE_AVA_TTL_S", 15*time.Minute),
		AlunoExecMaxPerMin: envInt("SENO_ALUNO_EXEC_MAX_PER_MIN", 5),
		ExecucaoRetencao:   envDuration("SENO_EXECUCAO_RETENCAO_D", 7*24*time.Hour),
		HMACJanela:         envDuration("SENO_MONITOR_HMAC_JANELA_S", 60*time.Second),
		CookieSecure:       envBool("SENO_COOKIE_SECURE", true),
		Judge0URL:          os.Getenv("SENO_JUDGE0_URL"),
		Judge0Token:        os.Getenv("SENO_JUDGE0_TOKEN"),
		Judge0Secret:       os.Getenv("SENO_JUDGE0_CALLBACK_SECRET"),
	}

	c.DatabaseURL = os.Getenv("SENO_DATABASE_URL")
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("SENO_DATABASE_URL é obrigatório")
	}
	if len(c.Secret) < 32 {
		return nil, fmt.Errorf("SENO_MONITOR_SECRET é obrigatório e precisa de pelo menos 32 bytes")
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
