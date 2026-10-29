// Package finance tem as regras de negócio: resumo do mês, divisão do salário e dívidas.
// Não acessa banco nem HTTP, então é fácil de testar.
package finance

import (
	"sort"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// BucketLine compara o planejado (percentual da renda) com o realizado no mês.
type BucketLine struct {
	Bucket       string
	Pct          int
	PlannedCents int64
	SpentCents   int64
}

func (b BucketLine) RemainingCents() int64 { return b.PlannedCents - b.SpentCents }

// UsedPct é quanto do planejado já foi usado (pode passar de 100).
func (b BucketLine) UsedPct() int {
	if b.PlannedCents == 0 {
		if b.SpentCents > 0 {
			return 100
		}
		return 0
	}
	return int(b.SpentCents * 100 / b.PlannedCents)
}

type CategoryLine struct {
	Name       string
	Bucket     string
	SpentCents int64
	SharePct   int // fatia do total de saídas
}

type Summary struct {
	Month      domain.Month
	InCents    int64
	OutCents   int64
	Buckets    []BucketLine
	Categories []CategoryLine // saídas por categoria, maior primeiro
}

func (s Summary) BalanceCents() int64 { return s.InCents - s.OutCents }

// SavingsRatePct é quanto da renda sobrou + foi investido.
func (s Summary) SavingsRatePct() int {
	if s.InCents == 0 {
		return 0
	}
	var invested int64
	for _, b := range s.Buckets {
		if b.Bucket == domain.BucketInvestimentos {
			invested = b.SpentCents
		}
	}
	return int((s.BalanceCents() + invested) * 100 / s.InCents)
}

// Summarize monta o resumo do mês a partir do livro-caixa.
func Summarize(m domain.Month, l domain.Ledger) Summary {
	s := summarize(l, m.Contains)
	s.Month = m
	return s
}

// SummarizePeriod soma os meses de from a to (inclusive), para relatórios de
// vários meses. O planejado de cada grupo usa a renda do período inteiro.
func SummarizePeriod(from, to domain.Month, l domain.Ledger) Summary {
	first, last := from.Key(), to.Key()
	s := summarize(l, func(date string) bool {
		return len(date) >= 7 && date[:7] >= first && date[:7] <= last
	})
	s.Month = from
	return s
}

func summarize(l domain.Ledger, in func(date string) bool) Summary {
	var s Summary
	split := l.Split()
	cats := l.CategoryIndex()
	spentByBucket := map[string]int64{}
	byCategory := map[string]*CategoryLine{}
	for _, t := range l.Transactions { // soma: a ordem não importa (ActiveTransactions copia e ordena tudo)
		if t.Deleted {
			continue
		}
		if !in(t.Date) {
			continue
		}
		if t.Kind == domain.KindEntrada {
			s.InCents += t.AmountCents
			continue
		}
		cat := cats[t.CategoryID]
		s.OutCents += t.AmountCents
		spentByBucket[cat.Bucket] += t.AmountCents
		c, ok := byCategory[t.CategoryID]
		if !ok {
			c = &CategoryLine{Name: cat.Name, Bucket: cat.Bucket}
			byCategory[t.CategoryID] = c
		}
		c.SpentCents += t.AmountCents
	}
	for _, b := range domain.SpendingBuckets {
		s.Buckets = append(s.Buckets, BucketLine{
			Bucket:       b,
			Pct:          split[b],
			PlannedCents: s.InCents * int64(split[b]) / 100,
			SpentCents:   spentByBucket[b],
		})
	}
	for _, c := range byCategory {
		if s.OutCents > 0 {
			c.SharePct = int(c.SpentCents * 100 / s.OutCents)
		}
		s.Categories = append(s.Categories, *c)
	}
	sort.Slice(s.Categories, func(i, j int) bool {
		if s.Categories[i].SpentCents != s.Categories[j].SpentCents {
			return s.Categories[i].SpentCents > s.Categories[j].SpentCents
		}
		return s.Categories[i].Name < s.Categories[j].Name
	})
	return s
}

// Totals são os totais de um mês.
type Totals struct {
	Month        domain.Month `json:"-"`
	Key          string       `json:"month"` // "2026-10"
	InCents      int64        `json:"inCents"`
	OutCents     int64        `json:"outCents"`
	BalanceCents int64        `json:"balanceCents"`
}

// YearTotals devolve entradas e saídas de cada um dos 12 meses do ano.
func YearTotals(l domain.Ledger, year int) []Totals {
	out := make([]Totals, 12)
	for i := range out {
		out[i].Month = domain.NewMonth(year, time.Month(i+1))
		out[i].Key = out[i].Month.Key()
	}
	for _, t := range l.Transactions { // soma: a ordem não importa (ActiveTransactions copia e ordena tudo)
		if t.Deleted {
			continue
		}
		d := t.Day()
		if d.Year() != year {
			continue
		}
		m := &out[d.Month()-1]
		if t.Kind == domain.KindEntrada {
			m.InCents += t.AmountCents
		} else {
			m.OutCents += t.AmountCents
		}
	}
	for i := range out {
		out[i].BalanceCents = out[i].InCents - out[i].OutCents
	}
	return out
}

// RecentTotals devolve os meses com movimento entre (m - n meses) e m, inclusive.
func RecentTotals(l domain.Ledger, m domain.Month, n int) []Totals {
	from := m.AddDate(0, -n, 0)
	var out []Totals
	for year := from.Year(); year <= m.Year(); year++ {
		for _, t := range YearTotals(l, year) {
			if t.Month.Before(from) || t.Month.After(m.Time) || (t.InCents == 0 && t.OutCents == 0) {
				continue
			}
			out = append(out, t)
		}
	}
	return out
}
