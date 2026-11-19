package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Verificação em duas etapas (TOTP, RFC 6238): o código de 6 dígitos que o Google
// Authenticator, o Authy ou o app de senhas do celular mostram a cada 30 segundos.
// SHA-1 é o que todos esses apps usam por padrão; com HMAC, segue seguro para isso.

const totpPeriod = 30

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret gera o segredo compartilhado com o app (160 bits, em base32).
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// TOTPURI é o endereço otpauth:// que vai no QR code para o app ler.
func TOTPURI(secret, account string) string {
	q := url.Values{"secret": {secret}, "issuer": {"Rich"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + url.PathEscape("Rich:"+account) + "?" + q.Encode()
}

func totpCode(key []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(sum[off:off+4])&0x7fffffff%1_000_000)
}

// TOTPCode é o código do segredo no instante t (o que o app mostra naquele momento).
func TOTPCode(secret string, t time.Time) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", err
	}
	return totpCode(key, t.Unix()/totpPeriod), nil
}

// normalizeCode tira espaços e traços e põe em minúsculas, como a pessoa pode digitar.
func normalizeCode(code string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code)))
}

// CheckTOTP confere o código no passo atual ou num vizinho (o relógio do celular pode
// estar 30 s adiantado ou atrasado) e devolve o passo usado. Um passo até last já foi
// usado e não vale de novo: quem espiar o código não entra com ele depois.
func CheckTOTP(secret, code string, now time.Time, last int64) (int64, bool) {
	key, err := b32.DecodeString(strings.ToUpper(secret))
	code = normalizeCode(code)
	if err != nil || len(key) == 0 || len(code) != 6 {
		return 0, false
	}
	step := now.Unix() / totpPeriod
	for _, s := range []int64{step - 1, step, step + 1} {
		if s > last && subtle.ConstantTimeCompare([]byte(totpCode(key, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

// Letras e números sem os que se confundem (i, l, o, 0, 1).
const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// NewRecoveryCodes gera n códigos de recuperação ("abcd-efgh-jkmn", cerca de 59 bits) e
// os hashes que ficam guardados. SHA-256 basta: os códigos são aleatórios e longos, e as
// tentativas passam pelo limite de tentativas; não precisam do PBKDF2 da senha.
func NewRecoveryCodes(n int) (codes, hashes []string, err error) {
	for range n {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		for i := range b {
			b[i] = recoveryAlphabet[int(b[i])%len(recoveryAlphabet)]
		}
		code := string(b[:4]) + "-" + string(b[4:8]) + "-" + string(b[8:])
		codes = append(codes, code)
		hashes = append(hashes, hashRecovery(code))
	}
	return codes, hashes, nil
}

func hashRecovery(code string) string {
	sum := sha256.Sum256([]byte(normalizeCode(code)))
	return hex.EncodeToString(sum[:])
}

// UseRecoveryCode confere um código de recuperação e devolve os hashes sem ele: cada
// código vale uma vez.
func UseRecoveryCode(code string, hashes []string) ([]string, bool) {
	if normalizeCode(code) == "" {
		return hashes, false
	}
	h := hashRecovery(code)
	for i, saved := range hashes {
		if subtle.ConstantTimeCompare([]byte(saved), []byte(h)) == 1 {
			return append(append([]string{}, hashes[:i]...), hashes[i+1:]...), true
		}
	}
	return hashes, false
}
