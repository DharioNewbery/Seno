package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"seno/internal/config"
)

// Claims define o conteúdo do token JWT emitido pela aplicação.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// TokenPair representa o par de tokens de acesso e atualização.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type Manager struct {
	secret            []byte
	accessExpiration  time.Duration
	refreshExpiration time.Duration
	issuer            string
}

func NewManager(cfg config.JWTConfig) *Manager {
	return &Manager{
		secret:            []byte(cfg.Secret),
		accessExpiration:  cfg.AccessExpiration,
		refreshExpiration: cfg.RefreshExpiration,
		issuer:            cfg.Issuer,
	}
}

// GenerateTokenPair emite um novo par de tokens para o usuário informado.
func (m *Manager) GenerateTokenPair(userID uuid.UUID, email string) (*TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(m.accessExpiration)
	refreshExp := now.Add(m.refreshExpiration)

	accessToken, err := m.signedToken(userID, email, now, accessExp)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar token de acesso: %w", err)
	}

	refreshToken, err := m.signedToken(userID, email, now, refreshExp)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar token de atualização: %w", err)
	}

	return &TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
	}, nil
}

func (m *Manager) signedToken(userID uuid.UUID, email string, issuedAt, expiresAt time.Time) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse valida e decodifica um token, retornando suas claims.
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token inválido")
	}
	return claims, nil
}
