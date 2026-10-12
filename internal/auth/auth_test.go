package auth

import (
	"errors"
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
		if !l.Allowed("ip") {
			t.Fatal("bloqueou cedo demais")
		}
		l.Fail("ip")
	}
	if l.Allowed("ip") {
		t.Error("deveria bloquear após 3 erros")
	}
	if !l.Allowed("outro-ip") {
		t.Error("bloqueio deve ser por endereço")
	}
	now = now.Add(2 * time.Minute)
	if !l.Allowed("ip") {
		t.Error("bloqueio deveria expirar")
	}
	l.Fail("ip")
	l.Reset("ip")
	if !l.Allowed("ip") || len(l.failures) != 0 {
		t.Error("reset deveria limpar")
	}
}
