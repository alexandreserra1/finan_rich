package auth

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	if _, err := HashPassword("curta"); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("senha curta: err = %v", err)
	}
	hash, err := HashPassword("minha-senha-boa")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := HashPassword("minha-senha-boa")
	if hash == other {
		t.Error("o sal deve deixar cada hash diferente")
	}
	for pw, want := range map[string]bool{"minha-senha-boa": true, "minha-senha-ruim": false, "": false} {
		if ok, err := CheckPassword(pw, hash); err != nil || ok != want {
			t.Errorf("CheckPassword(%q) = %v, %v", pw, ok, err)
		}
	}
	if _, err := CheckPassword("x", "texto-qualquer"); err == nil {
		t.Error("hash inválido deveria dar erro")
	}
}

func TestToken(t *testing.T) {
	a, _ := NewToken()
	b, _ := NewToken()
	if a == b || len(a) < 40 {
		t.Errorf("tokens fracos: %q %q", a, b)
	}
	if HashToken(a) == a || HashToken(a) != HashToken(a) {
		t.Error("hash do token inconsistente")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(3, time.Minute)
	l.now = func() time.Time { return now }
	for range 3 {
		if !l.Take("ip") {
			t.Fatal("bloqueou cedo demais")
		}
	}
	if l.Take("ip") {
		t.Error("deveria bloquear após 3 tentativas sem acerto")
	}
	if !l.Take("outro-ip") {
		t.Error("bloqueio deve ser por endereço")
	}
	now = now.Add(2 * time.Minute)
	if !l.Take("ip") {
		t.Error("bloqueio deveria expirar")
	}
	l.Reset("ip")
	l.Reset("outro-ip")
	if len(l.failures) != 0 {
		t.Error("reset deveria limpar")
	}
}

// Muitos pedidos ao mesmo tempo não passam do limite: a vaga é reservada antes
// de conferir a senha (que demora), não depois.
func TestLimiterParallel(t *testing.T) {
	l := NewLimiter(5, time.Minute)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 200 {
		wg.Go(func() {
			if l.Take("ip") {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if ok.Load() != 5 {
		t.Errorf("passaram %d tentativas, limite 5", ok.Load())
	}
}
