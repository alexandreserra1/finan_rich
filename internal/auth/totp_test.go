package auth

import (
	"encoding/base32"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Vetores do apêndice B da RFC 6238 (SHA-1), com os 6 últimos dígitos que os apps mostram.
func TestTOTPRFCVectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1111111111: "050471", 1234567890: "005924", 2000000000: "279037"} {
		step, ok := CheckTOTP(secret, want, time.Unix(unix, 0), 0)
		if !ok || step != unix/30 {
			t.Errorf("t=%d: código %s recusado (passo %d)", unix, want, step)
		}
	}
}

func TestCheckTOTP(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil || len(secret) != 32 || strings.Contains(secret, "=") {
		t.Fatalf("segredo = %q, %v", secret, err)
	}
	now := time.Unix(1_800_000_000, 0)
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	code := func(step int64) string { return totpCode(key, step) }
	step := now.Unix() / 30
	// O relógio do celular pode estar 30 s adiantado ou atrasado.
	for _, s := range []int64{step - 1, step, step + 1} {
		if _, ok := CheckTOTP(secret, code(s), now, 0); !ok {
			t.Errorf("passo %+d recusado", s-step)
		}
	}
	for _, s := range []int64{step - 2, step + 2} {
		if _, ok := CheckTOTP(secret, code(s), now, 0); ok {
			t.Errorf("passo %+d aceito: janela larga demais", s-step)
		}
	}
	// Espaços e traço do jeito que a pessoa digita valem; o mesmo código não vale duas vezes.
	c := code(step)
	got, ok := CheckTOTP(secret, " "+c[:3]+"-"+c[3:]+" ", now, 0)
	if !ok || got != step {
		t.Fatalf("código com espaço e traço recusado")
	}
	if _, ok := CheckTOTP(secret, c, now, got); ok {
		t.Error("código já usado aceito de novo")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if _, ok := CheckTOTP(secret, bad, now, 0); ok {
			t.Errorf("%q aceito", bad)
		}
	}
	if _, ok := CheckTOTP("não é base32!", c, now, 0); ok {
		t.Error("segredo inválido aceito")
	}
}

func TestTOTPURI(t *testing.T) {
	u, err := url.Parse(TOTPURI("ABC234", "Meu PC"))
	if err != nil || u.Scheme != "otpauth" || u.Host != "totp" || u.Path != "/Rich:Meu PC" {
		t.Fatalf("URI = %v, %v", u, err)
	}
	q := u.Query()
	if q.Get("secret") != "ABC234" || q.Get("issuer") != "Rich" || q.Get("digits") != "6" || q.Get("period") != "30" {
		t.Errorf("parâmetros = %v", q)
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := NewRecoveryCodes(8)
	if err != nil || len(codes) != 8 || len(hashes) != 8 {
		t.Fatalf("códigos = %v, %v", codes, err)
	}
	seen := map[string]bool{}
	for i, c := range codes {
		if len(c) != 14 || strings.Count(c, "-") != 2 || seen[c] || strings.Contains(hashes[i], c) {
			t.Errorf("código %q (hash %q)", c, hashes[i])
		}
		seen[c] = true
	}
	// Vale uma vez, sem diferenciar maiúsculas e sem os traços.
	rest, ok := UseRecoveryCode(strings.ToUpper(strings.ReplaceAll(codes[3], "-", "")), hashes)
	if !ok || len(rest) != 7 {
		t.Fatalf("código de recuperação recusado (%d sobrando)", len(rest))
	}
	if _, ok := UseRecoveryCode(codes[3], rest); ok {
		t.Error("código de recuperação aceito duas vezes")
	}
	if _, ok := UseRecoveryCode("", hashes); ok {
		t.Error("código vazio aceito")
	}
}
