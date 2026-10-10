package advisor

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// Snapshot descreve em texto os dados atuais do usuário para o modelo.
func Snapshot(today time.Time, sum finance.Summary, recent []store.Totals, plan finance.DebtPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Data de hoje: %s\n\n", today.Format("02/01/2006"))

	fmt.Fprintf(&b, "## Mês atual (%s)\n", sum.Month.Label())
	fmt.Fprintf(&b, "- Entradas: %s\n- Saídas: %s\n- Saldo: %s\n",
		money.Format(sum.InCents), money.Format(sum.OutCents), money.Format(sum.BalanceCents()))
	b.WriteString("\nDivisão do salário (planejado x realizado):\n")
	for _, l := range sum.Buckets {
		fmt.Fprintf(&b, "- %s: meta %d%% = %s; gasto %s (%d%% da meta)\n",
			l.Bucket, l.Pct, money.Format(l.PlannedCents), money.Format(l.SpentCents), l.UsedPct())
	}
	if len(sum.Categories) > 0 {
		b.WriteString("\nSaídas por categoria:\n")
		for _, c := range sum.Categories {
			fmt.Fprintf(&b, "- %s (%s): %s — %d%% das saídas\n", c.Name, c.Bucket, money.Format(c.SpentCents), c.SharePct)
		}
	} else {
		b.WriteString("\nNenhuma saída lançada neste mês ainda.\n")
	}

	if len(recent) > 0 {
		b.WriteString("\n## Últimos meses\n")
		for _, t := range recent {
			fmt.Fprintf(&b, "- %s: entradas %s, saídas %s, saldo %s\n",
				t.Month.Short(), money.Format(t.InCents), money.Format(t.OutCents), money.Format(t.BalanceCents))
		}
	}

	b.WriteString("\n## Dívidas\n")
	if len(plan.Debts) == 0 {
		b.WriteString("Nenhuma dívida cadastrada.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "Total devido: %s; soma das parcelas mínimas: %s/mês; juros gerados hoje: ~%s/mês\n",
		money.Format(plan.TotalCents), money.Format(plan.TotalMinCents), money.Format(plan.MonthlyInterest))
	for _, d := range plan.Debts {
		fmt.Fprintf(&b, "- %s", d.Name)
		if d.Creditor != "" {
			fmt.Fprintf(&b, " (%s)", d.Creditor)
		}
		fmt.Fprintf(&b, ": saldo %s, juros %s a.m., pagamento mínimo %s",
			money.Format(d.BalanceCents), RatePct(d.MonthlyRateBP), money.Format(d.MinPaymentCents))
		if d.Payable {
			fmt.Fprintf(&b, ", quitada em ~%d meses pagando só o mínimo (juros totais ~%s)", d.Months, money.Format(d.InterestCents))
		} else if d.MinPaymentCents > 0 {
			b.WriteString(", o pagamento mínimo NÃO cobre os juros: a dívida cresce")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// RatePct formata pontos-base como "1,99%".
func RatePct(bp int) string {
	return strings.Replace(fmt.Sprintf("%d.%02d%%", bp/100, bp%100), ".", ",", 1)
}
