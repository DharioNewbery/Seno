package sessao

import (
	"sync"
	"time"
)

// Limiter limita tentativas falhas de entrada por IP — janela sliding com
// teto de falhas. Estado em memória: perda ao reiniciar é irrelevante por
// princípio (mesma mecânica de api/internal/auth/ratelimit.go).
type Limiter struct {
	mu      sync.Mutex
	window  time.Duration
	maxFail int
	fails   map[string][]time.Time
}

// NovoLimiter cria o limitador.
func NovoLimiter(maxFail int, window time.Duration) *Limiter {
	return &Limiter{window: window, maxFail: maxFail, fails: map[string][]time.Time{}}
}

// Bloqueado informa se a chave atingiu o teto e por quanto tempo.
func (l *Limiter) Bloqueado(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.poda(key, time.Now())
	fl := l.fails[key]
	if len(fl) >= l.maxFail {
		wait := l.window - time.Since(fl[0])
		if wait < 0 {
			wait = 0
		}
		return true, wait
	}
	return false, 0
}

// Falha registra uma falha de entrada na janela.
func (l *Limiter) Falha(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.poda(key, time.Now())
	l.fails[key] = append(l.fails[key], time.Now())
}

// poda remove marcas fora da janela (chamado com lock em vigor).
func (l *Limiter) poda(key string, now time.Time) {
	cut := now.Add(-l.window)
	old := l.fails[key]
	kept := old[:0]
	for _, t := range old {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
		return
	}
	l.fails[key] = kept
}
