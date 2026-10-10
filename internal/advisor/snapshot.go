package advisor

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// Facts reúne os dados que o consultor recebe a cada pergunta.
type Facts struct {
	Today     time.Time
	Month     finance.Summary
	Recent    []store.Totals // meses anteriores com movimento
	Debts     finance.DebtPlan
	Emergency finance.EmergencyFund
}

// Snapshot descreve os dados em texto para o modelo. Todas as contas vêm
// prontas: modelos pequenos erram aritmética, então o app calcula e o modelo
// só interpreta.
func Snapshot(f Facts) string {
	var b strings.Builder
	brl := money.Format
	sum := f.Month
	fmt.Fprintf(&b, "Data de hoje: %s\n\n", f.Today.Format("02/01/2006"))

	fmt.Fprintf(&b, "## Mês atual (%s)\n", sum.Month.Label())
	fmt.Fprintf(&b, "- Entradas: %s\n- Saídas: %s\n- Sobra do mês (entradas - saídas): %s\n",
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
			fmt.Fprintf(&b, "- Guardando toda a sobra média (%s/mês): mínima em %d meses, ideal em %d meses\n",
				brl(e.MonthlySavingCents), e.MonthsToMin, e.MonthsToIdeal)
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
		names := func(ds []store.Debt) string {
			out := make([]string, len(ds))
			for i, d := range ds {
				out[i] = d.Name
			}
			return strings.Join(out, " → ")
		}
		fmt.Fprintf(&b, "Ordem pelo método avalanche (maior juros primeiro, economiza mais): %s\n", names(f.Debts.Avalanche))
		fmt.Fprintf(&b, "Ordem pelo método bola de neve (menor saldo primeiro): %s\n", names(f.Debts.Snowball))
	}

	if alerts := attentionPoints(f); len(alerts) > 0 {
		b.WriteString("\n## Pontos de atenção\n")
		for _, a := range alerts {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	return b.String()
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
