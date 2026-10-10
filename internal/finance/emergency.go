package finance

import "github.com/alexandreserra1/finan_rich/internal/store"

// EmergencyFund é a reserva de emergência recomendada: de 3 (mínimo) a 6 meses
// (ideal) do gasto mensal médio.
type EmergencyFund struct {
	AvgMonthlyOutCents int64 // gasto médio por mês
	MinCents           int64 // 3 meses de gastos
	IdealCents         int64 // 6 meses de gastos
	MonthlySavingCents int64 // quanto sobra por mês em média
	MonthsToMin        int   // meses para juntar o mínimo no ritmo atual (0 = não dá com a sobra atual)
	MonthsToIdeal      int
}

// PlanEmergencyFund calcula a reserva a partir dos meses com movimento.
// pendingDebtCents são parcelas de dívida que ainda não aparecem nas saídas e
// por isso são descontadas da sobra mensal.
func PlanEmergencyFund(months []store.Totals, pendingDebtCents int64) EmergencyFund {
	var f EmergencyFund
	var n, totalOut, totalBalance int64
	for _, m := range months {
		if m.InCents == 0 && m.OutCents == 0 {
			continue
		}
		n++
		totalOut += m.OutCents
		totalBalance += m.BalanceCents
	}
	if n == 0 {
		return f
	}
	f.AvgMonthlyOutCents = totalOut / n
	f.MonthlySavingCents = totalBalance/n - pendingDebtCents
	f.MinCents = 3 * f.AvgMonthlyOutCents
	f.IdealCents = 6 * f.AvgMonthlyOutCents
	if f.MonthlySavingCents > 0 {
		f.MonthsToMin = ceilDiv(f.MinCents, f.MonthlySavingCents)
		f.MonthsToIdeal = ceilDiv(f.IdealCents, f.MonthlySavingCents)
	}
	return f
}

func ceilDiv(a, b int64) int { return int((a + b - 1) / b) }
