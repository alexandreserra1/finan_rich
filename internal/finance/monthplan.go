package finance

import "github.com/alexandreserra1/finan_rich/internal/domain"

// MonthPlan é o plano do mês calculado pelo app: quanto ainda falta pagar das
// dívidas, quanto sobra livre e para onde mandar o extra (método avalanche).
type MonthPlan struct {
	SurplusCents      int64 // entradas - saídas lançadas
	DebtPaidCents     int64 // pagamentos de dívida já lançados no mês
	PendingMinCents   int64 // parcelas mínimas que ainda faltam lançar
	PendingFixedCents int64
	FreeCents         int64 // sobra depois dos fixos pendentes e parcelas mínimas (pode ser negativa)

	// Dívida que deve receber o dinheiro extra (a de maior juros).
	Target             *DebtView
	TargetBudgetCents  int64
	TargetPaymentCents int64 // estimativa limitada ao saldo com juros do primeiro período
	TargetMonths       int   // meses para quitar pagando TargetPaymentCents
	TargetInterest     int64 // juros totais nesse ritmo
	TargetPayable      bool
}

// PlanMonth monta o plano do mês a partir do resumo e das dívidas.
func PlanMonth(sum Summary, debts DebtPlan, pendingFixedCents int64) MonthPlan {
	p := MonthPlan{SurplusCents: sum.BalanceCents(), PendingFixedCents: max(pendingFixedCents, 0)}
	for _, c := range sum.Categories {
		if c.Name == domain.DebtPaymentCategory {
			p.DebtPaidCents = c.SpentCents
		}
	}
	var minimums int64
	for i := range debts.Debts {
		d := &debts.Debts[i]
		if d.BalanceCents <= 0 {
			continue
		}
		minimums += min(d.MinPaymentCents, d.BalanceCents+d.MonthlyInterestCents)
		if p.Target == nil || d.MonthlyRateBP > p.Target.MonthlyRateBP {
			p.Target = d
		}
	}
	p.PendingMinCents = max(minimums-p.DebtPaidCents, 0)
	p.FreeCents = p.SurplusCents - p.PendingFixedCents - p.PendingMinCents
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
