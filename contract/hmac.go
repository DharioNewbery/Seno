package contract

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

//assinatura HTTP das rotas internas API <-> Monitor (spec §2):
//
//	X-Monitor-MAC:  hex(HMAC-SHA256(secret, tempo + "\n" + corpo))
//	X-Monitor-Time: RFC3339 UTC
//
// Desvio documentado da spec: o MAC cobre TAMBÉM o tempo — com MAC só do
// corpo, a janela anti-replay não protege (capturar corpo+MAC e reenviar com
// Time novo passaria). Assinando o tempo junto, a janela mordida real.

// Headers dos compromissos internos.
const (
	HeaderMAC  = "X-Monitor-MAC"
	HeaderTime = "X-Monitor-Time"
)

// Erros de assinatura: o handler mapeia para 401 (MAC) sem expor qual falhou.
var (
	ErrMACInvalido = errors.New("contract: MAC ausente ou inválido")
	ErrTempoFora   = errors.New("contract: X-Monitor-Time fora da janela")
	ErrTempo       = errors.New("contract: X-Monitor-Time ilegível")
)

// AssinarMAC devolve o MAC e o tempo (formato do header) para o corpo dado.
func AssinarMAC(secret []byte, agora time.Time, corpo []byte) (mac, tempo string) {
	tempo = agora.UTC().Format(time.RFC3339)
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(tempo))
	m.Write([]byte{'\n'})
	m.Write(corpo)
	return hex.EncodeToString(m.Sum(nil)), tempo
}

// VerificarMAC valida MAC e tempo de uma requisição interna. janela é a
// tolerância de relógio entre emissor e receptor (anti-replay).
func VerificarMAC(secret []byte, corpo []byte, mac, tempo string, janela time.Duration) error {
	if mac == "" || tempo == "" {
		return ErrMACInvalido
	}
	ts, err := time.Parse(time.RFC3339, tempo)
	if err != nil {
		return ErrTempo
	}
	if d := time.Since(ts); d < -janela || d > janela {
		return ErrTempoFora
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(tempo))
	m.Write([]byte{'\n'})
	m.Write(corpo)
	if !hmac.Equal([]byte(mac), []byte(hex.EncodeToString(m.Sum(nil)))) {
		return ErrMACInvalido
	}
	return nil
}

// HMACNomeDeRestricao Documenta o tamanho mínimo do segredo compartilhado.
const HMACMinSecretLen = 32

// Valida o segredo compartilhado (usado no startup e nos testes).
func VerificarSecret(sec string) error {
	if len(sec) < HMACMinSecretLen {
		return fmt.Errorf("contrato: secret de %d bytes; mínimo %d", len(sec), HMACMinSecretLen)
	}
	return nil
}
