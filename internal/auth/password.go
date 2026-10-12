// Package auth cuida da senha de acesso e das sessões de login.
//
// O app é de um usuário só. A senha protege o acesso pela rede (celular no
// Wi-Fi de casa ou servidor); quem usa no próprio computador não precisa dela.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MinPasswordLen é o tamanho mínimo da senha.
const MinPasswordLen = 8

// iterations segue a recomendação da OWASP para PBKDF2-HMAC-SHA256.
const iterations = 600_000

var (
	ErrPasswordTooShort = fmt.Errorf("a senha precisa ter pelo menos %d caracteres", MinPasswordLen)
	errBadHash          = errors.New("hash de senha em formato inválido")
)

// HashPassword gera "pbkdf2-sha256$iterações$sal$hash" para guardar no banco.
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLen {
		return "", ErrPasswordTooShort
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// CheckPassword compara a senha com o hash em tempo constante.
func CheckPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false, errBadHash
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false, errBadHash
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false, errBadHash
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
