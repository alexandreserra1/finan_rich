package advisor

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// BuildFacts calcula tudo o que o consultor recebe, a partir do livro-caixa.
func BuildFacts(l domain.Ledger, today time.Time) Facts {
	m := domain.NewMonth(today.Year(), today.Month())
	sum := finance.Summarize(m, l)
	debts := finance.PlanDebts(l.ActiveDebts())
	plan := finance.PlanMonth(sum, debts)
	var recent []finance.Totals
	all := finance.RecentTotals(l, m, 6)
	for _, t := range all {
		if t.Key != m.Key() {
			recent = append(recent, t)
		}
	}
	notes := l.ActiveNotes()
	if len(notes) > maxNotes {
		notes = notes[:maxNotes]
	}
	return Facts{
		Today: today, Month: sum, Recent: recent, Debts: debts, Plan: plan,
		Emergency: finance.PlanEmergencyFund(all, plan.PendingMinCents),
		Notes:     notes,
		Goals:     finance.Goals(l, today),
		Recurring: l.ActiveRecurring(),
		Merchants: finance.TopMerchants(l, today, maxMerchants),
		Alerts:    finance.Alerts(l, today),
		Insights:  finance.Insights(l, today),
		Names:     categoryNames(l),
	}
}

// Facts reúne os dados que o consultor recebe a cada pergunta.
type Facts struct {
	Today     time.Time
	Month     finance.Summary
	Recent    []finance.Totals // meses anteriores com movimento
	Debts     finance.DebtPlan
	Emergency finance.EmergencyFund
	Plan      finance.MonthPlan
	Notes     []domain.Note // as mais recentes (fixadas primeiro)
	Goals     []finance.GoalView
	Recurring []domain.Recurring
	Merchants []finance.Merchant // onde mais gastou nos últimos 12 meses
	Alerts    []finance.Alert
	Insights  finance.InsightsView // tela Análise: ritmo, pode gastar por dia, onde economizar
	Names     map[string]string    // categoria: ID → nome
}

// maxMerchants: quantos estabelecimentos vão para o modelo.
const maxMerchants = 25

func categoryNames(l domain.Ledger) map[string]string {
	out := map[string]string{}
	for _, c := range l.Categories {
		out[c.ID] = c.Name
	}
	return out
}

// Quantas notas e quanto de cada uma vão para o modelo: o suficiente para ele
// conhecer os planos da pessoa sem encher o contexto de modelos pequenos.
const (
	maxNotes     = 5
	maxNoteRunes = 400
)

// Snapshot descreve os dados em texto para o modelo. Todas as contas vêm
// prontas: modelos pequenos erram aritmética, então o app calcula e o modelo
// só interpreta.
func Snapshot(f Facts) string {
	var b strings.Builder
	brl := money.Format
	sum := f.Month
	fmt.Fprintf(&b, "Data de hoje: %s\n\n", f.Today.Format("02/01/2006"))

	fmt.Fprintf(&b, "## Mês atual (%s)\n", sum.Month.Label())
	fmt.Fprintf(&b, "- Entradas: %s\n- Saídas lançadas: %s\n- Sobra do mês (entradas - saídas lançadas): %s\n",
		brl(sum.InCents), brl(sum.OutCents), brl(sum.BalanceCents()))
	b.WriteString("\nDivisão do salário (planejado x realizado):\n")
	for _, l := range sum.Buckets {
		fmt.Fprintf(&b, "- %s: meta %d%% = %s; gasto %s (%d%% da meta); ", l.Bucket, l.Pct, brl(l.PlannedCents), brl(l.SpentCents), l.UsedPct())
		if l.RemainingCents() < 0 {
			fmt.Fprintf(&b, "passou %s da meta\n", brl(-l.RemainingCents()))
		} else {
			fmt.Fprintf(&b, "ainda cabem %s\n", brl(l.RemainingCents()))
		}
	}
	if len(sum.Categories) > 0 {
		b.WriteString("\nSaídas por categoria:\n")
		for _, c := range sum.Categories {
			fmt.Fprintf(&b, "- %s (%s): %s — %d%% das saídas\n", c.Name, c.Bucket, brl(c.SpentCents), c.SharePct)
		}
	} else {
		b.WriteString("\nNenhuma saída lançada neste mês ainda.\n")
	}

	if len(f.Recent) > 0 {
		b.WriteString("\n## Últimos meses\n")
		for _, t := range f.Recent {
			fmt.Fprintf(&b, "- %s: entradas %s, saídas %s, sobra %s\n", t.Month.Short(), brl(t.InCents), brl(t.OutCents), brl(t.BalanceCents))
		}
	}

	if e := f.Emergency; e.AvgMonthlyOutCents > 0 {
		b.WriteString("\n## Reserva de emergência (já calculada)\n")
		fmt.Fprintf(&b, "- Gasto médio por mês: %s\n- Reserva mínima (3 meses de gastos): %s\n- Reserva ideal (6 meses de gastos): %s\n",
			brl(e.AvgMonthlyOutCents), brl(e.MinCents), brl(e.IdealCents))
		if e.MonthsToMin > 0 {
			fmt.Fprintf(&b, "- Guardando toda a sobra livre (%s/mês, já descontadas as parcelas de dívidas): mínima em %d meses, ideal em %d meses\n",
				brl(e.MonthlySavingCents), e.MonthsToMin, e.MonthsToIdeal)
			if f.Debts.TotalCents > 0 {
				b.WriteString("- Com dívidas de juros altos, o comum é juntar só uma reserva pequena e quitar essas dívidas antes de completar a reserva\n")
			}
		} else {
			b.WriteString("- Hoje não sobra dinheiro no mês, então não dá para formar reserva sem cortar gastos\n")
		}
	}

	b.WriteString("\n## Dívidas\n")
	if len(f.Debts.Debts) == 0 {
		b.WriteString("Nenhuma dívida cadastrada.\n")
	} else {
		fmt.Fprintf(&b, "Total devido: %s; soma das parcelas mínimas: %s/mês; juros somados de todas as dívidas: %s/mês\n",
			brl(f.Debts.TotalCents), brl(f.Debts.TotalMinCents), brl(f.Debts.MonthlyInterest))
		for _, d := range f.Debts.Debts {
			fmt.Fprintf(&b, "- %s", d.Name)
			if d.Creditor != "" {
				fmt.Fprintf(&b, " (%s)", d.Creditor)
			}
			fmt.Fprintf(&b, ": saldo %s, juros de %s a.m. = %s de juros por mês, pagamento mínimo %s",
				brl(d.BalanceCents), RatePct(d.MonthlyRateBP), brl(d.MonthlyInterestCents), brl(d.MinPaymentCents))
			if d.Payable {
				fmt.Fprintf(&b, "; pagando só o mínimo, quita em %d meses e paga %s de juros no total", d.Months, brl(d.InterestCents))
			} else if d.MinPaymentCents > 0 {
				fmt.Fprintf(&b, "; o mínimo NÃO cobre os juros (%s < %s): a dívida cresce todo mês", brl(d.MinPaymentCents), brl(d.MonthlyInterestCents))
			}
			b.WriteString("\n")
		}
		names := func(ds []domain.Debt) string {
			out := make([]string, len(ds))
			for i, d := range ds {
				out[i] = d.Name
			}
			return strings.Join(out, " → ")
		}
		fmt.Fprintf(&b, "Ordem pelo método avalanche (maior juros primeiro, economiza mais): %s\n", names(f.Debts.Avalanche))
		fmt.Fprintf(&b, "Ordem pelo método bola de neve (menor saldo primeiro): %s\n", names(f.Debts.Snowball))
	}

	writeMonthPlan(&b, f.Plan)

	if len(f.Goals) > 0 {
		b.WriteString("\n## Metas do usuário\n")
		for _, g := range f.Goals {
			fmt.Fprintf(&b, "- %s: juntou %s de %s (%d%%)", g.Name, brl(g.ProgressCents), brl(g.TargetCents), g.Pct)
			switch {
			case g.Done:
				b.WriteString("; meta atingida")
			case g.Late:
				fmt.Fprintf(&b, "; prazo (%s) já passou, faltam %s", domain.ParseMonth(g.Deadline).Label(), brl(g.RemainingCents))
			case g.PerMonthCents > 0:
				fmt.Fprintf(&b, "; prazo %s; precisa guardar %s por mês nos próximos %d meses", domain.ParseMonth(g.Deadline).Label(), brl(g.PerMonthCents), g.MonthsLeft)
			default:
				fmt.Fprintf(&b, "; sem prazo, faltam %s", brl(g.RemainingCents))
			}
			b.WriteString("\n")
		}
	}

	if len(f.Recurring) > 0 {
		b.WriteString("\n## Lançamentos que se repetem todo mês\n")
		for _, r := range f.Recurring {
			kind := "saída"
			if r.Kind == domain.KindEntrada {
				kind = "entrada"
			}
			status := ""
			if r.Paused {
				status = " (pausado)"
			}
			fmt.Fprintf(&b, "- %s: %s de %s todo dia %d, categoria %s%s\n", r.Description, kind, brl(r.AmountCents), r.Day, f.Names[r.CategoryID], status)
		}
	}

	if len(f.Merchants) > 0 {
		fmt.Fprintf(&b, "\n## Gastos por estabelecimento (últimos 12 meses, os %d maiores)\n", len(f.Merchants))
		for _, m := range f.Merchants {
			fmt.Fprintf(&b, "- %s: %s em %d compra(s) nos últimos 12 meses; %s em %d; %s neste mês; última em %s\n",
				m.Name, brl(m.Last12Cents), m.Count, brl(m.YearCents), f.Today.Year(), brl(m.MonthCents), m.LastDate[8:10]+"/"+m.LastDate[5:7]+"/"+m.LastDate[:4])
		}
	}

	if in := f.Insights; in.Day > 0 {
		fmt.Fprintf(&b, "\n## Ritmo do mês (dia %d de %d; os mesmos números da tela Análise)\n", in.Day, in.DaysInMonth)
		fmt.Fprintf(&b, "- Gastos variáveis até hoje: %s (no mês passado, até o mesmo dia: %s)\n", brl(in.VariableCents), brl(in.LastMonthSameDayCents))
		fmt.Fprintf(&b, "- Projeção do mês no ritmo atual: %s, com %s de contas fixas (mês passado: %s; média de 3 meses: %s)\n",
			brl(in.ProjectedCents), brl(in.FixedCents), brl(in.LastMonthCents), brl(in.Avg3Cents))
		if in.FreeCents >= 0 {
			fmt.Fprintf(&b, "- Pode gastar %s por dia até o fim do mês (sobram %s depois do investimento planejado e das contas fixas)\n", brl(in.PerDayCents), brl(in.FreeCents))
		} else {
			fmt.Fprintf(&b, "- O mês já passou %s do que cabe na renda (depois do investimento planejado e das contas fixas)\n", brl(-in.FreeCents))
		}
		for _, s := range in.Savings {
			fmt.Fprintf(&b, "- Onde economizar: %s — %s\n", s.Title, s.Text)
		}
	}

	if len(f.Insights.Budgets) > 0 {
		b.WriteString("\n## Orçamentos por categoria (teto do mês definido pelo usuário)\n")
		for _, o := range f.Insights.Budgets {
			fmt.Fprintf(&b, "- %s: teto %s; usado %s (%d%%); ", o.Name, brl(o.BudgetCents), brl(o.SpentCents), o.UsedPct)
			if o.RemainingCents < 0 {
				fmt.Fprintf(&b, "estourou %s\n", brl(-o.RemainingCents))
			} else {
				fmt.Fprintf(&b, "restam %s\n", brl(o.RemainingCents))
			}
		}
	}

	if len(f.Notes) > 0 {
		b.WriteString("\n## Anotações do usuário (escritas por ele no app)\n")
		for _, n := range f.Notes {
			title := n.Title
			if title == "" {
				title = "Sem título"
			}
			fmt.Fprintf(&b, "- %s: %s\n", title, clip(strings.Join(strings.Fields(n.Body), " "), maxNoteRunes))
		}
	}

	alerts := attentionPoints(f)
	for _, a := range f.Alerts {
		if a.Kind == "debt-due" || a.Kind == "unusual" || a.Kind == "goal-late" || a.Kind == "budget-near" || a.Kind == "budget-over" {
			alerts = append(alerts, a.Title+": "+a.Text)
		}
	}
	if len(alerts) > 0 {
		b.WriteString("\n## Pontos de atenção\n")
		for _, a := range alerts {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	return b.String()
}

// clip corta o texto em n caracteres, com reticências.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// writeMonthPlan descreve para onde vai o dinheiro do mês, com as contas feitas.
func writeMonthPlan(b *strings.Builder, p finance.MonthPlan) {
	if p.PendingMinCents == 0 && p.Target == nil {
		return
	}
	brl := money.Format
	b.WriteString("\n## Plano do mês (calculado pelo app)\n")
	if p.DebtPaidCents > 0 {
		fmt.Fprintf(b, "- Pagamentos de dívidas já lançados: %s\n", brl(p.DebtPaidCents))
	}
	fmt.Fprintf(b, "- Parcelas mínimas que ainda faltam pagar este mês (não estão nas saídas): %s\n", brl(p.PendingMinCents))
	fmt.Fprintf(b, "- Sobra livre depois de pagar as parcelas mínimas: %s - %s = %s\n",
		brl(p.SurplusCents), brl(p.PendingMinCents), brl(p.FreeCents))
	switch {
	case p.FreeCents <= 0:
		fmt.Fprintf(b, "- Não sobra nada depois das parcelas mínimas: faltam %s, então é preciso cortar gastos\n", brl(-p.FreeCents))
	case p.Target != nil && p.TargetPayable:
		fmt.Fprintf(b, "- Sugestão (avalanche): mandar toda a sobra livre para %s, pagando %s + %s = %s por mês; assim ela é quitada em %d meses, com %s de juros no total\n",
			p.Target.Name, brl(p.Target.MinPaymentCents), brl(p.FreeCents), brl(p.TargetPaymentCents), p.TargetMonths, brl(p.TargetInterest))
	case p.Target != nil:
		fmt.Fprintf(b, "- Mesmo mandando toda a sobra livre para %s (%s por mês), o pagamento não cobre os juros: é preciso cortar gastos ou renegociar\n",
			p.Target.Name, brl(p.TargetPaymentCents))
	}
}

// attentionPoints lista os problemas mais importantes, já em linguagem simples.
func attentionPoints(f Facts) []string {
	var out []string
	if f.Month.InCents > 0 && f.Month.BalanceCents() < 0 {
		out = append(out, fmt.Sprintf("O mês está no vermelho: saiu %s a mais do que entrou", money.Format(-f.Month.BalanceCents())))
	}
	for _, d := range f.Debts.Debts {
		if !d.Payable && d.MinPaymentCents > 0 {
			out = append(out, fmt.Sprintf("%s cresce todo mês: os juros (%s) são maiores que o pagamento mínimo (%s)",
				d.Name, money.Format(d.MonthlyInterestCents), money.Format(d.MinPaymentCents)))
		}
	}
	for _, l := range f.Month.Buckets {
		if l.PlannedCents > 0 && l.RemainingCents() < 0 {
			out = append(out, fmt.Sprintf("Gastos com %s passaram da meta em %s", l.Bucket, money.Format(-l.RemainingCents())))
		}
	}
	return out
}

// RatePct formata pontos-base como "1,99%".
func RatePct(bp int) string {
	return strings.Replace(fmt.Sprintf("%d.%02d%%", bp/100, bp%100), ".", ",", 1)
}
