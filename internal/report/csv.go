package report

import (
	"bytes"
	"encoding/csv"
	"strconv"
)

// CSV no padrão do Excel em português: separador ";", decimal com vírgula,
// UTF-8 com BOM (para os acentos abrirem certo) e quebra de linha CRLF.
// No pandas: pd.read_csv(arquivo, sep=";", decimal=",").
func writeCSV(t Table) []byte {
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf") // BOM do UTF-8
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	w.UseCRLF = true
	header := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		header[i] = c.Title
	}
	w.Write(header)
	for _, row := range t.Rows {
		rec := make([]string, len(row))
		for i, cell := range row {
			rec[i] = csvCell(t.Columns[i].Kind, cell)
		}
		w.Write(rec)
	}
	w.Flush()
	return buf.Bytes()
}

func csvCell(k Kind, v Cell) string {
	switch x := v.(type) {
	case int64:
		if k == Money {
			return decimalComma(x)
		}
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case string:
		return x
	}
	return ""
}

// decimalComma escreve centavos como "-1234,56" (sem separador de milhar,
// que atrapalha a leitura por programas).
func decimalComma(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	frac := strconv.FormatInt(cents%100, 10)
	if len(frac) == 1 {
		frac = "0" + frac
	}
	return sign + strconv.FormatInt(cents/100, 10) + "," + frac
}
