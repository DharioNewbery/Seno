package auth

import (
	"testing"
	"time"
)

func TestLoginLimiterBloqueiaAposMax(t *testing.T) {
	l := NewLoginLimiter(2, 50*time.Millisecond)

	if blocked, _ := l.Blocked("chave"); blocked {
		t.Fatal("bloqueado sem falhas")
	}
	l.RegisterFail("chave")
	if blocked, _ := l.Blocked("chave"); blocked {
		t.Fatal("bloqueado antes do máximo")
	}
	l.RegisterFail("chave")
	blocked, wait := l.Blocked("chave")
	if !blocked {
		t.Fatal("não bloqueou após o máximo de falhas")
	}
	if wait <= 0 {
		t.Error("janela de espera deveria ser positiva")
	}
}

func TestLoginLimiterJanelaExpira(t *testing.T) {
	l := NewLoginLimiter(1, 30*time.Millisecond)
	l.RegisterFail("chave")
	if blocked, _ := l.Blocked("chave"); !blocked {
		t.Fatal("deveria estar bloqueado após a falha")
	}
	time.Sleep(50 * time.Millisecond)
	if blocked, _ := l.Blocked("chave"); blocked {
		t.Fatal("bloqueio deveria expirar com a janela")
	}
}

func TestLoginLimiterReset(t *testing.T) {
	l := NewLoginLimiter(1, time.Minute)
	l.RegisterFail("a")
	if blocked, _ := l.Blocked("a"); !blocked {
		t.Fatal("deveria estar bloqueado após a falha")
	}
	l.Reset("a")
	if blocked, _ := l.Blocked("a"); blocked {
		t.Fatal("Reset deveria desbloquear")
	}
	// Chaves diferentes não interferem.
	l.RegisterFail("b")
	if blocked, _ := l.Blocked("c"); blocked {
		t.Error("chave sem falhas não deveria estar bloqueada")
	}
}
