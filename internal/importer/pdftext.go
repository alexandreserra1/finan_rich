package importer

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Fatura de cartão em PDF. O texto vem do aparelho (pdf.js lê o PDF e
// devolve uma linha de texto por linha da página); aqui reconhecemos as
// linhas de compra: data, descrição e valor. Cada banco tem seu layout, mas
// quase todos seguem "05/10 IFOOD *RESTAURANTE 52,30" ou "05 OUT Ifood 52,30".

var (
	// Uma compra: data (dia/mês, com ou sem ano; mês em número ou abreviado),
	// descrição e valor no fim da linha (sinal antes ou depois).
	pdfTx = regexp.MustCompile(`(?i)^\s*(?P<day>\d{1,2})\s*(?:/|-|\.|\s|\sde\s)\s*(?P<month>\d{1,2}|jan|fev|mar|abr|mai|jun|jul|ago|set|out|nov|dez)[a-zç.]*` +
		`(?:/(?P<year>\d{4}|\d{2})|\s+(?:de\s+)?(?P<year4>20\d{2}))?` + // ano só com "/" (05/10/26) ou com 4 dígitos (05 out 2026)
		`\s+(?P<desc>.+?)\s+(?P<sign>[-−–]\s*)?(?:R\$\s*)?(?P<amount>\d{1,3}(?:\.\d{3})*,\d{2})\s*(?P<suffix>[-−–]|D|C)?\s*$`)
	// Onde uma nova compra pode começar no meio da linha (faturas em duas colunas).
	pdfDateStart  = regexp.MustCompile(`(?i)(?:^|\s)(\d{1,2}\s*(?:/|-|\.|\s)\s*(?:\d{1,2}|jan|fev|mar|abr|mai|jun|jul|ago|set|out|nov|dez)[a-zç.]*)\s`)
	pdfEndsAmount = regexp.MustCompile(`\d,\d{2}\s*([-−–]|D|C)?\s*$`)
	pdfFullDate   = regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`)
	pdfDueDate    = regexp.MustCompile(`(?i)vencimento[^0-9]{0,40}(\d{2})/(\d{2})/(\d{4})`)
)

var monthAbbr = map[string]int{"jan": 1, "fev": 2, "mar": 3, "abr": 4, "mai": 5, "jun": 6, "jul": 7, "ago": 8, "set": 9, "out": 10, "nov": 11, "dez": 12}

// Linhas de resumo da fatura, que têm data e valor mas não são compras.
var pdfSkip = []string{
	"total", "saldo", "pagamento minimo", "valor minimo", "limite", "vencimento", "fatura anterior",
	"melhor dia", "encargos maximos", "cet", "taxa de juros", "juros do rotativo", "proxima fatura", "proximas faturas",
}

// ParseText lê as linhas de uma fatura em PDF. today serve de referência para
// o ano quando o PDF não traz a data de vencimento.
func ParseText(text string, today time.Time) (Statement, error) {
	st := Statement{Format: "pdf", Card: true, Unsigned: true}
	ref := referenceDate(text, today)
	for _, line := range strings.Split(text, "\n") {
		for _, seg := range splitColumns(line) {
			match := pdfTx.FindStringSubmatch(seg)
			if match == nil {
				continue
			}
			g := func(name string) string { return match[pdfTx.SubexpIndex(name)] }
			desc := cleanDescription(g("desc"))
			if skipLine(desc) {
				continue
			}
			date, ok := pdfDate(g("day"), g("month"), g("year")+g("year4"), ref)
			if !ok {
				continue
			}
			amount, ok := parseAmount(g("amount"))
			if !ok || amount == 0 {
				continue
			}
			// Na fatura, o valor positivo é compra; crédito vem com sinal.
			if suffix := g("suffix"); strings.TrimSpace(g("sign")) != "" || strings.ContainsAny(suffix, "-−–") || strings.EqualFold(suffix, "C") {
				amount = -amount
			}
			st.Rows = append(st.Rows, Row{Date: date, Description: desc, AmountCents: amount})
		}
	}
	if len(st.Rows) == 0 {
		return st, errors.New("não encontrei compras neste PDF. Ele é a fatura do cartão ou o extrato da conta? Se for, prefira exportar em OFX ou CSV no app do banco")
	}
	return st, nil
}

// splitColumns separa compras lado a lado na mesma linha (faturas em duas
// colunas): corta onde começa uma data logo depois de um valor.
func splitColumns(line string) []string {
	var out []string
	start := 0
	for _, loc := range pdfDateStart.FindAllStringSubmatchIndex(line, -1) {
		at := loc[2]
		if at > start && pdfEndsAmount.MatchString(line[start:at]) {
			out = append(out, line[start:at])
			start = at
		}
	}
	return append(out, line[start:])
}

func skipLine(desc string) bool {
	d := normalize(desc)
	for _, s := range pdfSkip {
		if strings.HasPrefix(d, s) || strings.Contains(d, " "+s+" ") && strings.HasPrefix(s, "total") {
			return true
		}
	}
	return false
}

// referenceDate é a data de vencimento da fatura (ou a maior data completa do
// texto, ou hoje): as compras são anteriores a ela.
func referenceDate(text string, today time.Time) time.Time {
	parse := func(m []string) (time.Time, bool) {
		t, err := time.Parse("02/01/2006", m[1]+"/"+m[2]+"/"+m[3])
		return t, err == nil
	}
	if m := pdfDueDate.FindStringSubmatch(text); m != nil {
		if t, ok := parse(m); ok {
			return t
		}
	}
	var best time.Time
	for _, m := range pdfFullDate.FindAllStringSubmatch(text, -1) {
		if t, ok := parse(m); ok && t.After(best) && t.Year() > 2000 {
			best = t
		}
	}
	if !best.IsZero() {
		return best
	}
	return today
}

// pdfDate monta a data da compra. Sem ano, usa o da referência; se a data
// ficar muito depois do vencimento, a compra é do ano anterior (fatura de
// janeiro com compras de dezembro).
func pdfDate(dayStr, monthStr, yearStr string, ref time.Time) (string, bool) {
	day, _ := strconv.Atoi(dayStr)
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		month = monthAbbr[strings.ToLower(monthStr[:3])]
	}
	if day < 1 || day > 31 || month < 1 || month > 12 {
		return "", false
	}
	year := ref.Year()
	if yearStr != "" {
		y, _ := strconv.Atoi(yearStr)
		if y < 100 {
			y += 2000
		}
		year = y
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Day() != day {
		return "", false // 31/02
	}
	if yearStr == "" && t.After(ref.AddDate(0, 0, 15)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t.Format("2006-01-02"), true
}
