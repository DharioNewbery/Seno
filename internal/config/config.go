package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultDevSecret = "dev-only-secret-please-override-in-production-32bytes"

// defaultSuperPassword é a senha temporária de desenvolvimento do superusuário.
// Em produção, SUPER_PASSWORD é obrigatória e não pode ser este valor.
const defaultSuperPassword = "SUPER1234"

type Config struct {
	App   AppConfig
	DB    DBConfig
	JWT   JWTConfig
	CORS  CORSConfig
	Super SuperConfig
}

type AppConfig struct {
	Env  string
	Host string
	Port string
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	Secret            string
	AccessExpiration  time.Duration
	RefreshExpiration time.Duration
	Issuer            string
}

type CORSConfig struct {
	AllowedOrigins []string
}

// SuperConfig define o superusuário semeado no primeiro startup.
type SuperConfig struct {
	Login    string
	Password string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "development"),
			Host: getEnv("APP_HOST", "0.0.0.0"),
			Port: getEnv("APP_PORT", "8085"),
		},
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "seno"),
			Password:        getEnv("DB_PASSWORD", "seno"),
			Name:            getEnv("DB_NAME", "seno"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		JWT: JWTConfig{
			Secret:            getEnv("JWT_SECRET", defaultDevSecret),
			AccessExpiration:  getEnvDuration("JWT_ACCESS_EXPIRATION", 15*time.Minute),
			RefreshExpiration: getEnvDuration("JWT_REFRESH_EXPIRATION", 168*time.Hour),
			Issuer:            getEnv("JWT_ISSUER", "seno"),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
		},
		Super: SuperConfig{
			Login:    getEnv("SUPER_LOGIN", "SUPER"),
			Password: getEnv("SUPER_PASSWORD", defaultSuperPassword),
		},
	}

	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET não pode ser vazio")
	}
	if cfg.App.IsProduction() {
		if cfg.JWT.Secret == defaultDevSecret {
			return nil, fmt.Errorf("JWT_SECRET deve ser redefinido em produção")
		}
		if len(cfg.JWT.Secret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET deve ter pelo menos 32 caracteres em produção")
		}
		if cfg.Super.Password == "" || cfg.Super.Password == defaultSuperPassword {
			return nil, fmt.Errorf("SUPER_PASSWORD deve ser definida em produção")
		}
	}

	return cfg, nil
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

func (c AppConfig) IsProduction() bool {
	return strings.EqualFold(c.Env, "production")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
