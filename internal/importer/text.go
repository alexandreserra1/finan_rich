package importer

import (
	"strings"
	"unicode/utf8"
)

// toUTF8 lê o arquivo como UTF-8; se não for (muitos bancos ainda exportam
// em Windows-1252, o "ANSI" do Windows), converte. Também tira o BOM.
func toUTF8(data []byte) string {
	if s := string(data); utf8.ValidString(s) {
		return strings.TrimPrefix(s, "\ufeff")
	}
	var b strings.Builder
	for _, c := range data {
		if r, ok := cp1252Extra[c]; ok {
			b.WriteRune(r)
		} else {
			b.WriteRune(rune(c)) // 0x00-0x7F e 0xA0-0xFF são iguais ao Unicode
		}
	}
	return b.String()
}

var cp1252Extra = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x84: '„', 0x85: '…', 0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”',
	0x95: '•', 0x96: '–', 0x97: '—', 0x99: '™', 0x8a: 'Š', 0x9a: 'š', 0x8c: 'Œ', 0x9c: 'œ',
}

// normalize deixa o texto comparável: minúsculas, sem acentos, só letras,
// números e espaços simples. "Pão de Açúcar - SP" → "pao de acucar sp".
func normalize(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if base, ok := unaccent[r]; ok {
			r = base
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			space = false
		case !space:
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

var unaccent = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'ç': 'c', 'ñ': 'n',
}

// cleanDescription tira espaços repetidos e limita o tamanho.
func cleanDescription(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120])
	}
	return s
}

// Normalize é o normalize para outros pacotes (a busca de lançamentos usa para
// achar "acucar" em "Pão de Açúcar").
func Normalize(s string) string { return normalize(s) }
