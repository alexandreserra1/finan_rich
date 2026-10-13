package finance

import (
	"sort"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/importer"
)

// Merchant soma os gastos com um mesmo estabelecimento ou serviço ("iFood",
// "Uber"...), reconhecido pela descrição sem números e palavras genéricas.
type Merchant struct {
	Name        string // a descrição mais usada
	Count       int    // compras nos últimos 12 meses
	Last12Cents int64
	YearCents   int64 // no ano atual
	MonthCents  int64 // no mês atual
	LastDate    string
}

// TopMerchants devolve os n estabelecimentos com mais gasto nos últimos 12
// meses. É o que deixa o consultor responder "quanto gastei com iFood?".
func TopMerchants(l domain.Ledger, today time.Time, n int) []Merchant {
	from := today.AddDate(-1, 0, 0).Format(domain.DateLayout)
	todayKey := today.Format(domain.DateLayout)
	year, month := today.Format("2006"), today.Format("2006-01")
	type acc struct {
		Merchant
		names map[string]int
	}
	byKey := map[string]*acc{}
	for _, t := range l.ActiveTransactions() {
		if t.Kind != domain.KindSaida || t.Date < from || t.Date > todayKey {
			continue
		}
		key := importer.DescKey(t.Description)
		if key == "" {
			continue
		}
		a := byKey[key]
		if a == nil {
			a = &acc{names: map[string]int{}}
			byKey[key] = a
		}
		a.names[t.Description]++
		a.Count++
		a.Last12Cents += t.AmountCents
		if t.Date[:4] == year {
			a.YearCents += t.AmountCents
		}
		if t.Date[:7] == month {
			a.MonthCents += t.AmountCents
		}
		if t.Date > a.LastDate {
			a.LastDate = t.Date
		}
	}
	out := make([]Merchant, 0, len(byKey))
	for _, a := range byKey {
		best := 0
		for name, c := range a.names {
			if c > best || c == best && name < a.Name {
				a.Name, best = name, c
			}
		}
		out = append(out, a.Merchant)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Last12Cents != out[j].Last12Cents {
			return out[i].Last12Cents > out[j].Last12Cents
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
