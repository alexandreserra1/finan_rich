// Package finance tem as regras de negócio: resumo do mês, divisão do salário e dívidas.
// Não acessa banco nem HTTP, então é fácil de testar.
package finance

import (
	"sort"

	"github.com/alexandreserra1/finan_rich/internal/store"
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
	Month      store.Month
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
		if b.Bucket == store.BucketInvestimentos {
			invested = b.SpentCents
		}
	}
	return int((s.BalanceCents() + invested) * 100 / s.InCents)
}

// Summarize monta o resumo do mês a partir dos lançamentos e da divisão planejada.
func Summarize(m store.Month, txs []store.Transaction, split map[string]int) Summary {
	s := Summary{Month: m}
	spentByBucket := map[string]int64{}
	byCategory := map[string]*CategoryLine{}
	for _, t := range txs {
		if t.Kind == store.KindEntrada {
			s.InCents += t.AmountCents
			continue
		}
		s.OutCents += t.AmountCents
		spentByBucket[t.Bucket] += t.AmountCents
		c, ok := byCategory[t.CategoryName]
		if !ok {
			c = &CategoryLine{Name: t.CategoryName, Bucket: t.Bucket}
			byCategory[t.CategoryName] = c
		}
		c.SpentCents += t.AmountCents
	}
	for _, b := range store.SpendingBuckets {
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
