// Package importer lê extratos bancários (OFX e CSV) e prepara os lançamentos
// para o usuário revisar: categoria sugerida, duplicados e o que ignorar.
//
// É Go puro, como o resto das regras: roda no aparelho (WebAssembly), então
// importar funciona sem conexão e o extrato não sai do aparelho.
package importer

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Row é um lançamento como veio do arquivo.
type Row struct {
	Date        string // AAAA-MM-DD
	Description string
	AmountCents int64  // com sinal: negativo = saiu dinheiro
	FITID       string // identificador do banco (OFX), se houver
	Category    string // nome da categoria, se o arquivo trouxer (CSV exportado pelo finan_rich)
}

// Statement é o extrato lido.
type Statement struct {
	Format  string // "ofx" ou "csv"
	Account string // conta ou cartão (OFX), para identificar os lançamentos
	Card    bool   // extrato de cartão de crédito (OFX CCSTMTRS)
	// Unsigned: os valores do CSV vêm sem sinal de entrada/saída e a maioria é
	// positiva; é o jeito dos extratos de cartão (positivo = gasto).
	Unsigned bool
	Rows     []Row
}

var ErrUnknownFile = errors.New("não reconheci o arquivo: envie o extrato em OFX ou CSV (o banco oferece em \"exportar extrato\")")

// Parse lê um extrato. O formato vem do conteúdo, não do nome do arquivo.
func Parse(data []byte) (Statement, error) {
	text := toUTF8(data)
	head := strings.ToUpper(text[:min(len(text), 2000)])
	if strings.Contains(head, "OFXHEADER") || strings.Contains(head, "<OFX>") || strings.Contains(strings.ToUpper(text), "<STMTTRN>") {
		return parseOFX(text)
	}
	if bytes.ContainsAny(data, ";,\t") {
		return parseCSV(text)
	}
	return Statement{}, ErrUnknownFile
}

// parseDate aceita os formatos que os bancos usam: 05/10/2026, 05/10/26,
// 2026-10-05, 05-10-2026, 05.10.2026 e 20261005 (OFX).
func parseDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 8 && isDigits(s[:8]) && (len(s) == 8 || !isDigit(s[8]) || len(s) >= 14) {
		if t, err := time.Parse("20060102", s[:8]); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	if len(s) > 10 { // "05/10/2026 14:32" ou "2026-10-05T14:32:00"
		if i := strings.IndexAny(s, " T"); i >= 8 {
			s = s[:i]
		}
	}
	for _, layout := range []string{"02/01/2006", "2/1/2006", "02/01/06", "2006-01-02", "02-01-2006", "02.01.2006", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil && t.Year() > 1990 && t.Year() < 2200 {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

// parseAmount lê valores como "-1.234,56", "R$ 1234.56", "(87,50)", "87,50 D".
// Devolve centavos com sinal.
func parseAmount(s string) (int64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\u00a0", " "))
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg, s = true, s[1:len(s)-1]
	}
	upper := strings.ToUpper(s)
	switch {
	case strings.HasSuffix(upper, " D"), strings.HasSuffix(upper, "D") && len(upper) > 1 && isDigit(upper[len(upper)-2]):
		neg, s = true, strings.TrimSpace(s[:len(s)-1])
	case strings.HasSuffix(upper, " C"):
		s = strings.TrimSpace(s[:len(s)-1])
	}
	s = strings.NewReplacer("R$", "", "r$", "", " ", "", "+", "").Replace(s)
	if strings.HasPrefix(s, "-") {
		neg, s = !neg, s[1:]
	}
	if strings.HasSuffix(s, "-") {
		neg, s = !neg, s[:len(s)-1]
	}
	if s == "" {
		return 0, false
	}
	lastComma, lastDot := strings.LastIndex(s, ","), strings.LastIndex(s, ".")
	var intPart, frac string
	switch {
	case lastComma >= 0 && lastDot >= 0: // os dois: o último é o decimal
		dec := max(lastComma, lastDot)
		intPart, frac = s[:dec], s[dec+1:]
	case lastComma >= 0:
		intPart, frac = s[:lastComma], s[lastComma+1:]
	case lastDot >= 0 && (strings.Count(s, ".") > 1 || len(s)-lastDot-1 == 3):
		intPart = s // "1.234.567" e "3.500": ponto de milhar, como no Brasil
	case lastDot >= 0:
		intPart, frac = s[:lastDot], s[lastDot+1:]
	default:
		intPart = s
	}
	intPart = strings.NewReplacer(".", "", ",", "").Replace(intPart)
	if intPart == "" {
		intPart = "0"
	}
	if len(frac) > 2 {
		frac = frac[:2] // "12.3456" → 12,34 (alguns OFX têm 4 casas)
	}
	for len(frac) < 2 {
		frac += "0"
	}
	if !isDigits(intPart) || !isDigits(frac) {
		return 0, false
	}
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, false
	}
	cents, _ := strconv.ParseInt(frac, 10, 64)
	v := whole*100 + cents
	if neg {
		v = -v
	}
	return v, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}
