package auth

import (
	"sync"
	"time"
)

// Limiter bloqueia temporariamente um endereço depois de muitas senhas erradas,
// para dificultar tentativas em massa.
type Limiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	failures map[string][]time.Time
	now      func() time.Time
}

const maxKeys = 10000

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, failures: map[string][]time.Time{}, now: time.Now}
}

// Take reserva uma tentativa para o endereço e diz se ele ainda pode tentar.
// Conta antes de conferir a senha (que demora), para pedidos em paralelo não
// passarem todos do limite; acertou, chame Reset.
func (l *Limiter) Take(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.failures) > maxKeys { // endereços que não voltaram não ficam para sempre na memória
		for k := range l.failures {
			l.recent(k)
		}
	}
	recent := l.recent(key)
	if len(recent) >= l.max {
		return false
	}
	l.failures[key] = append(recent, l.now())
	return true
}

// Reset limpa o histórico após um login correto.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

func (l *Limiter) recent(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	var out []time.Time
	for _, t := range l.failures[key] {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		delete(l.failures, key)
	} else {
		l.failures[key] = out
	}
	return out
}
