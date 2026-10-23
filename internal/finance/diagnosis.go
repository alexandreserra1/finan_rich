package finance

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Diagnosis é o diagnóstico das dívidas, feito só com os números da pessoa
// (sem IA): o que mais custa, onde dá para guardar e quanto cada corte
// adianta o plano. O consultor recebe isso pronto e só conversa sobre ele.
type Diagnosis struct {
	Costliest   *CostlyDebt    `json:"costliest"`   // a dívida que mais gera juros por mês
	TopCategory *CategoryTrend `json:"topCategory"` // o que mais pesou nos gastos do mês
	MonthLabel  string         `json:"monthLabel"`
	Levers      []Lever        `json:"levers"` // onde dá para guardar, do maior para o menor
}

type CostlyDebt struct {
	Name                 string `json:"name"`
	MonthlyInterestCents int64  `json:"monthlyInterestCents"`
	MonthlyRateBP        int    `json:"monthlyRateBp"`
}

// Lever é um corte possível e o efeito dele, sozinho, no plano das dívidas.
type Lever struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"` // sobra, categoria, frequente, desejos, assinatura
	Title              string `json:"title"`
	Text               string `json:"text"`
	MonthlyCents       int64  `json:"monthlyCents"`
	MonthsSooner       int    `json:"monthsSooner"`       // quantos meses antes sai das dívidas
	InterestSavedCents int64  `json:"interestSavedCents"` // juros a menos no caminho
	Ends               bool   `json:"ends"`               // sem ele as dívidas não acabam; com ele, acabam
}

// Diagnose monta o diagnóstico do mês de hoje.
func Diagnose(l domain.Ledger, today time.Time) Diagnosis {
	in := Insights(l, today)
	d := Diagnosis{MonthLabel: in.Label, Levers: []Lever{}}
	debts := l.ActiveDebts()
	for _, v := range PlanDebts(debts).Debts {
		if v.BalanceCents > 0 && (d.Costliest == nil || v.MonthlyInterestCents > d.Costliest.MonthlyInterestCents) {
			d.Costliest = &CostlyDebt{Name: v.Name, MonthlyInterestCents: v.MonthlyInterestCents, MonthlyRateBP: v.MonthlyRateBP}
		}
	}
	if len(in.Categories) > 0 && in.Categories[0].Cents > 0 {
		d.TopCategory = &in.Categories[0]
	}

	// Sobra média dos últimos 3 meses fechados (entradas − saídas).
	m := domain.NewMonth(today.Year(), today.Month())
	if recent := RecentTotals(l, m.Prev(), 2); len(recent) > 0 {
		var sum int64
		for _, t := range recent {
			sum += t.BalanceCents
		}
		if avg := sum / int64(len(recent)); avg > 0 {
			d.Levers = append(d.Levers, Lever{Kind: "sobra", Title: "Sobra dos últimos meses", MonthlyCents: avg,
				Text: fmt.Sprintf("Nos últimos %d meses sobraram em média %s por mês (entradas menos saídas). Pôr parte disso nas dívidas acelera o plano.", len(recent), money.Format(avg))})
		}
	}

	// Economias da Análise: uma por categoria (a maior), para não somar duas
	// vezes o mesmo gasto (iFood pela metade e Restaurantes de volta à média).
	best := map[string]Saving{}
	for _, s := range in.Savings {
		if s.MonthlyCents <= 0 || (s.Kind != "categoria" && s.Kind != "frequente") {
			continue
		}
		if b, ok := best[s.Category]; !ok || s.MonthlyCents > b.MonthlyCents {
			best[s.Category] = s
		}
	}
	wishes := false
	for _, s := range best {
		wishes = wishes || s.Bucket == domain.BucketDesejos
		d.Levers = append(d.Levers, Lever{Kind: s.Kind, Title: s.Title, Text: s.Text, MonthlyCents: s.MonthlyCents})
	}
	for _, s := range in.Savings {
		if s.Kind == "desejos" && !wishes { // os desejos acima do planejado já estão nos cortes acima
			d.Levers = append(d.Levers, Lever{Kind: s.Kind, Title: s.Title, Text: s.Text, MonthlyCents: s.MonthlyCents})
		}
	}
	// Assinaturas de desejos (streaming, apps); aluguel e contas da casa não são corte.
	for _, s := range in.Subscriptions {
		if s.Bucket == domain.BucketDesejos && s.MonthlyCents > 0 {
			d.Levers = append(d.Levers, Lever{Kind: "assinatura", Title: s.Name, MonthlyCents: s.MonthlyCents,
				Text: fmt.Sprintf("%s todo mês (%s por ano). Cancelar ou trocar por um plano mais barato.", money.Format(s.MonthlyCents), money.Format(s.YearlyCents))})
		}
	}
	slices.SortStableFunc(d.Levers, func(a, b Lever) int {
		if a.MonthlyCents != b.MonthlyCents {
			return int(b.MonthlyCents - a.MonthlyCents)
		}
		return strings.Compare(a.Title, b.Title)
	})

	base := PayoffSchedule(debts, 0, true)
	for i := range d.Levers {
		lv := &d.Levers[i]
		lv.ID = lv.Kind + ":" + strings.ToLower(lv.Title)
		if len(debts) == 0 {
			continue
		}
		with := PayoffSchedule(debts, lv.MonthlyCents, true)
		switch {
		case base.OK && with.OK:
			lv.MonthsSooner, lv.InterestSavedCents = base.Months-with.Months, base.InterestCents-with.InterestCents
		case !base.OK && with.OK:
			lv.Ends = true
		}
	}
	return d
}
