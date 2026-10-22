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

// valor depois de normalizado: reais e, se houver, a parte decimal.
var valor = regexp.MustCompile(`^(\d{1,15})(?:\.(\d+))?$`)

// Parse aceita "1.234,56", "1234,56", "1234.56" ou "1234" e devolve centavos.
// Só usa inteiros: nada de "NaN", "1e3" ou estouro virando número.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "R$"))
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, errors.New("valor vazio")
	}
	if strings.HasPrefix(s, "-") {
		return 0, errors.New("valor não pode ser negativo")
	}
	if milhar.MatchString(s) {
		s = strings.ReplaceAll(s, ".", "")
	} else if strings.Contains(s, ",") {
		// formato brasileiro: ponto é milhar, vírgula é decimal
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	}
	m := valor.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("valor inválido: %q", s)
	}
	reais, _ := strconv.ParseInt(m[1], 10, 64) // até 15 dígitos: cabe em int64
	dec := m[2] + "000"
	cents, _ := strconv.ParseInt(dec[:2], 10, 64)
	if dec[2] >= '5' { // arredonda o terceiro decimal
		cents++
	}
	return reais*100 + cents, nil
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
