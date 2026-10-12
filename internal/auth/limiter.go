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

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, failures: map[string][]time.Time{}, now: time.Now}
}

// Allowed diz se o endereço ainda pode tentar.
func (l *Limiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) < l.max
}

// Fail registra uma tentativa errada.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[key] = append(l.recent(key), l.now())
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
