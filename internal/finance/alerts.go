package finance

import (
	"fmt"
	"sort"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Alert é um aviso do painel (e, no celular, base das notificações).
type Alert struct {
	Kind  string `json:"kind"`  // debt-due, debt-growing, bucket-over, unusual, goal-late, goal-done
	Level string `json:"level"` // danger, warn, ok
	Title string `json:"title"`
	Text  string `json:"text"`
	Link  string `json:"link"` // tela onde agir
}

const maxAlerts = 6

// Alerts levanta o que merece atenção hoje.
func Alerts(l domain.Ledger, today time.Time) []Alert {
	var out []Alert
	brl := money.Format
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	current := domain.NewMonth(today.Year(), today.Month())

	for _, d := range PlanDebts(l.ActiveDebts()).Debts {
		if d.BalanceCents <= 0 {
			continue
		}
		if d.DueDay > 0 {
			due := current.Day(d.DueDay)
			if due.Before(day) {
				due = current.Next().Day(d.DueDay)
			}
			switch days := int(due.Sub(day).Hours() / 24); {
			case days == 0:
				out = append(out, Alert{"debt-due", "danger", d.Name + " vence hoje", "Parcela mínima: " + brl(d.MinPaymentCents) + ".", "/dividas"})
			case days <= 3:
				out = append(out, Alert{"debt-due", "warn", fmt.Sprintf("%s vence em %d dia%s (dia %d)", d.Name, days, plural(days), d.DueDay), "Parcela mínima: " + brl(d.MinPaymentCents) + ".", "/dividas"})
			}
		}
		if d.MinPaymentCents > 0 && !d.Payable {
			out = append(out, Alert{"debt-growing", "danger", d.Name + " está crescendo",
				"A parcela mínima (" + brl(d.MinPaymentCents) + ") não cobre os juros do mês (" + brl(d.MonthlyInterestCents) + ").", "/dividas"})
		}
	}

	s := Summarize(current, l)
	if s.InCents > 0 {
		for _, b := range s.Buckets {
			if b.PlannedCents > 0 && b.SpentCents > b.PlannedCents {
				out = append(out, Alert{"bucket-over", "warn", fmt.Sprintf("%s passou da meta do mês", bucketName(b.Bucket)),
					fmt.Sprintf("Gasto %s de %s planejados (%d%%).", brl(b.SpentCents), brl(b.PlannedCents), b.UsedPct()), "/"})
			}
		}
	}
	out = append(out, unusualSpending(l, current)...)

	for _, g := range Goals(l, today) {
		switch {
		case g.Late:
			out = append(out, Alert{"goal-late", "warn", "Prazo da meta \"" + g.Name + "\" passou", "Faltam " + brl(g.RemainingCents) + ". Que tal ajustar o prazo?", "/metas"})
		case g.Done && g.Deadline != "" && g.Deadline >= current.Key():
			out = append(out, Alert{"goal-done", "ok", "Meta \"" + g.Name + "\" atingida!", "Você juntou " + brl(g.ProgressCents) + ".", "/metas"})
		}
	}

	rank := map[string]int{"danger": 0, "warn": 1, "ok": 2}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Level] < rank[out[j].Level] })
	if len(out) > maxAlerts {
		out = out[:maxAlerts]
	}
	if out == nil {
		out = []Alert{}
	}
	return out
}

// unusualSpending: categorias em que o mês já passou bem da média dos três
// meses anteriores (pelo menos 50% e R$ 100 acima).
func unusualSpending(l domain.Ledger, m domain.Month) []Alert {
	now := Summarize(m, l)
	avg := map[string]int64{}
	for _, prev := range []domain.Month{m.Prev(), m.Prev().Prev(), m.Prev().Prev().Prev()} {
		for _, c := range Summarize(prev, l).Categories {
			avg[c.Name] += c.SpentCents
		}
	}
	var out []Alert
	for _, c := range now.Categories {
		a := avg[c.Name] / 3
		if a > 0 && c.SpentCents*2 >= a*3 && c.SpentCents-a >= 10000 {
			out = append(out, Alert{"unusual", "warn", c.Name + " acima do normal",
				fmt.Sprintf("%s este mês, %d%% acima da média dos últimos 3 meses (%s).", money.Format(c.SpentCents), (c.SpentCents-a)*100/a, money.Format(a)), "/lancamentos"})
		}
	}
	return out
}

func bucketName(b string) string {
	switch b {
	case domain.BucketNecessidades:
		return "Necessidades"
	case domain.BucketDesejos:
		return "Desejos"
	case domain.BucketInvestimentos:
		return "Investimentos"
	}
	return b
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
