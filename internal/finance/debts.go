package finance

import (
	"sort"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// maxMonths limita a simulação: acima disso consideramos que a dívida não é quitada.
const maxMonths = 600

// Payoff estima em quantos meses a dívida é quitada pagando monthlyCents por mês,
// e quanto de juros é pago no total. ok=false se o pagamento não cobre os juros.
func Payoff(balanceCents int64, rateBP int, monthlyCents int64) (months int, interestCents int64, ok bool) {
	bal := balanceCents
	for bal > 0 {
		if months >= maxMonths {
			return months, interestCents, false
		}
		interest := bal * int64(rateBP) / 10000
		if monthlyCents <= interest {
			return 0, 0, false
		}
		interestCents += interest
		bal += interest - monthlyCents
		months++
	}
	return months, interestCents, true
}

type DebtView struct {
	domain.Debt
	MonthlyInterestCents int64 // juros que o saldo atual gera em um mês
	Months               int   // meses para quitar pagando só o mínimo
	InterestCents        int64 // juros totais pagando só o mínimo
	Payable              bool  // o pagamento mínimo cobre os juros?
}

type DebtPlan struct {
	Debts           []DebtView
	TotalCents      int64
	TotalMinCents   int64
	MonthlyInterest int64         // juros que as dívidas geram por mês hoje
	Avalanche       []domain.Debt // maior juros primeiro (economiza mais)
	Snowball        []domain.Debt // menor saldo primeiro (vitórias rápidas)
}

func PlanDebts(debts []domain.Debt) DebtPlan {
	var p DebtPlan
	for _, d := range debts {
		months, interest, ok := Payoff(d.BalanceCents, d.MonthlyRateBP, d.MinPaymentCents)
		monthly := d.BalanceCents * int64(d.MonthlyRateBP) / 10000
		p.Debts = append(p.Debts, DebtView{Debt: d, MonthlyInterestCents: monthly, Months: months, InterestCents: interest, Payable: ok})
		p.TotalCents += d.BalanceCents
		p.TotalMinCents += d.MinPaymentCents
		p.MonthlyInterest += monthly
	}
	p.Avalanche = append([]domain.Debt(nil), debts...)
	sort.SliceStable(p.Avalanche, func(i, j int) bool { return p.Avalanche[i].MonthlyRateBP > p.Avalanche[j].MonthlyRateBP })
	p.Snowball = append([]domain.Debt(nil), debts...)
	sort.SliceStable(p.Snowball, func(i, j int) bool { return p.Snowball[i].BalanceCents < p.Snowball[j].BalanceCents })
	return p
}
