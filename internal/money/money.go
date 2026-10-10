// Package money trata valores em centavos (int64), evitando erros de ponto flutuante.
package money

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// milhar casa "3.500" e "1.250.000": sem vírgula, o ponto é separador de milhar.
var milhar = regexp.MustCompile(`^\d{1,3}(\.\d{3})+$`)

// Parse aceita "1.234,56", "1234,56", "1234.56" ou "1234" e devolve centavos.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "R$"))
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, errors.New("valor vazio")
	}
	if milhar.MatchString(s) {
		s = strings.ReplaceAll(s, ".", "")
	} else if strings.Contains(s, ",") {
		// formato brasileiro: ponto é milhar, vírgula é decimal
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("valor inválido: %q", s)
	}
	if f < 0 {
		return 0, errors.New("valor não pode ser negativo")
	}
	return int64(f*100 + 0.5), nil
}

// Format devolve "R$ 1.234,56".
func Format(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	reais := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, r := range reais {
		if i > 0 && (len(reais)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, b.String(), cents%100)
}

// Input devolve o valor no formato de formulário: "1234,56".
func Input(cents int64) string {
	return fmt.Sprintf("%d,%02d", cents/100, cents%100)
}
