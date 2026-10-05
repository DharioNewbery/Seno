package auth

import (
	"sync"
	"time"
)

// LoginLimiter limita tentativas falhas de login por chave (e-mail+IP),
// janela sliding de 15 min com teto de falhas (ARQUITETURA §5.2).
// Estado em memória: sua perda ao reiniciar é irrelevante por princípio.
type LoginLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	maxFail int
	fails   map[string][]time.Time
}

// NewLoginLimiter cria o limitador.
func NewLoginLimiter(maxFail int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{window: window, maxFail: maxFail, fails: map[string][]time.Time{}}
}

// Blocked informa se a chave está bloqueada e por quanto tempo.
func (l *LoginLimiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(key, time.Now())
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

// RegisterFail registra uma falha de login na janela.
func (l *LoginLimiter) RegisterFail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(key, time.Now())
	l.fails[key] = append(l.fails[key], time.Now())
}

// Reset limpa as falhas da chave (login com sucesso).
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// prune remove marcas fora da janela (chamado com lock em vigor).
func (l *LoginLimiter) prune(key string, now time.Time) {
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
