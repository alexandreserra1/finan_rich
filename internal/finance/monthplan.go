package finance

import "github.com/alexandreserra1/finan_rich/internal/domain"

// MonthPlan é o plano do mês calculado pelo app: quanto ainda falta pagar das
// dívidas, quanto sobra livre e para onde mandar o extra (método avalanche).
type MonthPlan struct {
	SurplusCents      int64 // entradas - saídas lançadas
	DebtPaidCents     int64 // pagamentos de dívida já lançados no mês
	PendingMinCents   int64 // parcelas mínimas que ainda faltam lançar
	PendingFixedCents int64
	// PendingVariableCents: gasto do dia a dia que ainda deve vir até o fim do mês
	// (o que os meses anteriores gastaram depois de hoje, como na Análise).
	PendingVariableCents int64
	FreeCents            int64 // sobra depois dos fixos, das parcelas mínimas e do gasto que ainda vem (pode ser negativa)

	// Dívida que deve receber o dinheiro extra (a de maior juros).
	Target             *DebtView
	TargetBudgetCents  int64
	TargetPaymentCents int64 // estimativa limitada ao saldo com juros do primeiro período
	TargetMonths       int   // meses para quitar pagando TargetPaymentCents
	TargetInterest     int64 // juros totais nesse ritmo
	TargetPayable      bool
}

// PlanMonth monta o plano do mês a partir do resumo e das dívidas, reservando os fixos
// que ainda vão cair e o gasto do dia a dia que ainda deve vir até o fim do mês.
func PlanMonth(sum Summary, debts DebtPlan, pendingFixedCents, pendingVariableCents int64) MonthPlan {
	p := MonthPlan{SurplusCents: sum.BalanceCents(), PendingFixedCents: max(pendingFixedCents, 0),
		PendingVariableCents: max(pendingVariableCents, 0), DebtPaidCents: debtPaid(sum)}
	for i := range debts.Debts {
		d := &debts.Debts[i]
		if d.BalanceCents > 0 && (p.Target == nil || d.MonthlyRateBP > p.Target.MonthlyRateBP) {
			p.Target = d
		}
	}
	p.PendingMinCents = pendingMinimums(debts, p.DebtPaidCents)
	p.FreeCents = p.SurplusCents - p.PendingFixedCents - p.PendingMinCents - p.PendingVariableCents
	if p.Target != nil && p.FreeCents > 0 {
		p.TargetBudgetCents = p.FreeCents
		if p.DebtPaidCents == 0 {
			p.TargetBudgetCents += min(p.Target.MinPaymentCents, p.PendingMinCents)
		}
		p.TargetPaymentCents = min(p.TargetBudgetCents, p.Target.BalanceCents+p.Target.MonthlyInterestCents)
		p.TargetMonths, p.TargetInterest, p.TargetPayable = Payoff(p.Target.BalanceCents, p.Target.MonthlyRateBP, p.TargetPaymentCents)
	}
	return p
}

// pendingMinimums: parcelas mínimas que ainda faltam no mês, cada uma limitada ao
// saldo com os juros do mês, menos o que já foi lançado em "Pagamento de dívidas".
func pendingMinimums(debts DebtPlan, paidCents int64) int64 {
	var minimums int64
	for _, d := range debts.Debts {
		if d.BalanceCents > 0 {
			minimums += min(d.MinPaymentCents, d.BalanceCents+d.MonthlyInterestCents)
		}
	}
	return max(minimums-paidCents, 0)
}

// debtPaid: o que foi lançado em "Pagamento de dívidas" no mês do resumo.
func debtPaid(sum Summary) int64 {
	for _, c := range sum.Categories {
		if c.Name == domain.DebtPaymentCategory {
			return c.SpentCents
		}
	}
	return 0
}
