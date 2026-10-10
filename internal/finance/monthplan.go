package finance

import "github.com/alexandreserra1/finan_rich/internal/store"

// MonthPlan é o plano do mês calculado pelo app: quanto ainda falta pagar das
// dívidas, quanto sobra livre e para onde mandar o extra (método avalanche).
type MonthPlan struct {
	SurplusCents    int64 // entradas - saídas lançadas
	DebtPaidCents   int64 // pagamentos de dívida já lançados no mês
	PendingMinCents int64 // parcelas mínimas que ainda faltam lançar
	FreeCents       int64 // sobra depois de pagar as parcelas mínimas (pode ser negativa)

	// Dívida que deve receber o dinheiro extra (a de maior juros).
	Target             *DebtView
	TargetPaymentCents int64 // mínimo + toda a sobra livre
	TargetMonths       int   // meses para quitar pagando TargetPaymentCents
	TargetInterest     int64 // juros totais nesse ritmo
	TargetPayable      bool
}

// PlanMonth monta o plano do mês a partir do resumo e das dívidas.
func PlanMonth(sum Summary, debts DebtPlan) MonthPlan {
	p := MonthPlan{SurplusCents: sum.BalanceCents()}
	for _, c := range sum.Categories {
		if c.Name == store.DebtPaymentCategory {
			p.DebtPaidCents = c.SpentCents
		}
	}
	p.PendingMinCents = max(debts.TotalMinCents-p.DebtPaidCents, 0)
	p.FreeCents = p.SurplusCents - p.PendingMinCents

	for i := range debts.Debts {
		d := &debts.Debts[i]
		if p.Target == nil || d.MonthlyRateBP > p.Target.MonthlyRateBP {
			p.Target = d
		}
	}
	if p.Target != nil && p.FreeCents > 0 {
		p.TargetPaymentCents = p.Target.MinPaymentCents + p.FreeCents
		p.TargetMonths, p.TargetInterest, p.TargetPayable = Payoff(p.Target.BalanceCents, p.Target.MonthlyRateBP, p.TargetPaymentCents)
	}
	return p
}
