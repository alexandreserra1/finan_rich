package importer

import (
	"encoding/csv"
	"errors"
	"strings"
)

// CSV de banco: cada um tem suas colunas. Reconhecemos pelo nome do
// cabeçalho (data, descrição, valor...), que pode não estar na primeira linha.

type csvColumns struct {
	date, amount, credit, debit, kind, category int
	desc                                        []int // pode haver várias (ex.: histórico e descrição)
}

var (
	dateNames   = []string{"data", "date", "data lancamento", "data do lancamento", "data da compra", "data movimento", "data mov", "dt", "data transacao", "data da transacao", "data de lancamento"}
	descNames   = []string{"descricao", "historico", "title", "titulo", "estabelecimento", "lancamento", "memo", "nome", "detalhe", "detalhes", "description", "descricao do lancamento", "transacao"}
	amountNames = []string{"valor", "amount", "value", "quantia", "montante", "valor em", "valor da transacao", "valor lancamento"}
	creditNames = []string{"credito", "entrada", "entradas", "creditos", "valor credito"}
	debitNames  = []string{"debito", "saida", "saidas", "debitos", "valor debito"}
	kindNames   = []string{"tipo", "tipo de transacao", "natureza", "c d", "d c"}
	catNames    = []string{"categoria", "category"}
)

func parseCSV(text string) (Statement, error) {
	st := Statement{Format: "csv"}
	records, err := readCSV(text)
	if err != nil {
		return st, err
	}
	header, cols, ok := findHeader(records)
	if !ok {
		return st, errors.New("não reconheci as colunas do CSV: ele precisa ter um cabeçalho com data, descrição e valor")
	}
	positives, negatives := 0, 0
	for _, rec := range records[header+1:] {
		get := func(i int) string {
			if i < 0 || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		date, ok := parseDate(get(cols.date))
		if !ok {
			continue // linhas de saldo, totais e rodapés
		}
		var amount int64
		switch {
		case cols.amount >= 0:
			if amount, ok = parseAmount(get(cols.amount)); !ok {
				continue
			}
		default: // colunas separadas de crédito e débito
			c, okC := parseAmount(get(cols.credit))
			d, okD := parseAmount(get(cols.debit))
			if !okC && !okD {
				continue
			}
			amount = abs(c) - abs(d)
		}
		if cols.kind >= 0 {
			if k := normalize(get(cols.kind)); k != "" {
				switch {
				case strings.HasPrefix(k, "sai"), strings.HasPrefix(k, "deb"), k == "d":
					amount = -abs(amount)
				case strings.HasPrefix(k, "ent"), strings.HasPrefix(k, "cred"), k == "c":
					amount = abs(amount)
				}
			}
		}
		var parts []string
		for _, i := range cols.desc {
			if v := get(i); v != "" && !contains(parts, v) {
				parts = append(parts, v)
			}
		}
		desc := cleanDescription(strings.Join(parts, " - "))
		if desc == "" || amount == 0 {
			continue
		}
		if amount > 0 {
			positives++
		} else {
			negatives++
		}
		st.Rows = append(st.Rows, Row{Date: date, Description: desc, AmountCents: amount, Category: get(cols.category)})
	}
	if len(st.Rows) == 0 {
		return st, errors.New("não encontrei lançamentos no CSV")
	}
	// Sem coluna de tipo e quase tudo positivo: é fatura de cartão, onde o
	// valor positivo é gasto (conta corrente tem mais saídas que entradas).
	st.Unsigned = cols.kind < 0 && cols.amount >= 0 && positives >= 4*negatives && positives > 0
	return st, nil
}

func readCSV(text string) ([][]string, error) {
	first := text
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		first = text[:min(len(text), i+2000)]
	}
	comma := ','
	best := strings.Count(first, ",")
	for _, c := range []rune{';', '\t'} {
		if n := strings.Count(first, string(c)); n > best {
			comma, best = c, n
		}
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = comma
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil && len(records) == 0 {
		return nil, errors.New("não consegui ler o CSV: " + err.Error())
	}
	return records, nil
}

// findHeader procura, nas primeiras linhas, o cabeçalho com data, descrição
// e valor (ou crédito e débito).
func findHeader(records [][]string) (int, csvColumns, bool) {
	for i, rec := range records[:min(len(records), 15)] {
		cols := csvColumns{-1, -1, -1, -1, -1, -1, nil}
		for j, cell := range rec {
			// "Crédito (R$)" → "credito"
			name := strings.TrimSuffix(strings.TrimSuffix(normalize(cell), " rs"), " r")
			pick := func(target *int, names []string) {
				if *target < 0 && contains(names, name) {
					*target = j
				}
			}
			pick(&cols.date, dateNames)
			if contains(descNames, name) {
				cols.desc = append(cols.desc, j)
			}
			pick(&cols.amount, amountNames)
			pick(&cols.credit, creditNames)
			pick(&cols.debit, debitNames)
			pick(&cols.kind, kindNames)
			pick(&cols.category, catNames)
		}
		if cols.date >= 0 && len(cols.desc) > 0 && (cols.amount >= 0 || cols.credit >= 0 && cols.debit >= 0) {
			return i, cols, true
		}
	}
	return 0, csvColumns{}, false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
