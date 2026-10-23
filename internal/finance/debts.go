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

// Schedule é a simulação de pagar várias dívidas ao mesmo tempo.
type Schedule struct {
	Months        int       `json:"months"`        // meses até zerar tudo (vale se OK)
	InterestCents int64     `json:"interestCents"` // juros pagos no caminho
	OK            bool      `json:"ok"`            // false: do jeito simulado, a dívida não acaba
	BalanceCents  []int64   `json:"balanceCents"`  // saldo total no fim de cada mês; [0] é hoje
	PaidOff       []PaidOff `json:"paidOff"`       // em que mês cada dívida zera, na ordem
	Names         []string  `json:"names"`         // as dívidas na ordem da fila (colunas do calendário)
	Calendar      []Month   `json:"calendar"`      // [i] é o mês i+1: quanto pagar em cada dívida
}

// Month é um mês do calendário de pagamentos.
type Month struct {
	Payments      []int64 `json:"payments"`      // na ordem de Names
	InterestCents int64   `json:"interestCents"` // juros que entraram no mês
}

type PaidOff struct {
	Name  string `json:"name"`
	Month int    `json:"month"`
}

// PayoffSchedule simula pagar todo mês a soma dos mínimos mais extraCents: o
// mínimo de cada dívida e todo o resto na primeira da fila (avalanche: maior
// juros; senão bola de neve: menor saldo). Quitada uma, o mínimo dela vai para a
// próxima da fila.
func PayoffSchedule(debts []domain.Debt, extraCents int64, avalanche bool) Schedule {
	order := append([]domain.Debt(nil), debts...)
	if avalanche {
		sort.SliceStable(order, func(i, j int) bool { return order[i].MonthlyRateBP > order[j].MonthlyRateBP })
	} else {
		sort.SliceStable(order, func(i, j int) bool { return order[i].BalanceCents < order[j].BalanceCents })
	}
	return simulate(order, extraCents, true)
}

// MinimumOnly simula pagar só o mínimo de cada dívida, cada uma por si.
func MinimumOnly(debts []domain.Debt) Schedule {
	return simulate(debts, 0, false)
}

// growthMonths: quando alguma dívida cresce (o mínimo não cobre os juros), a
// simulação para aqui; mais longe os números perdem sentido.
const growthMonths = 120

func simulate(order []domain.Debt, extraCents int64, rollover bool) Schedule {
	bal := make([]int64, len(order))
	done := make([]bool, len(order))
	budget, total := extraCents, int64(0)
	limit := maxMonths
	for i, d := range order {
		bal[i] = d.BalanceCents
		budget += d.MinPaymentCents
		total += d.BalanceCents
		if !rollover && d.BalanceCents > 0 && d.MinPaymentCents <= d.BalanceCents*int64(d.MonthlyRateBP)/10000 {
			limit = growthMonths
		}
	}
	s := Schedule{OK: true, BalanceCents: []int64{total}, PaidOff: []PaidOff{}, Names: make([]string, len(order)), Calendar: []Month{}}
	for i, d := range order {
		s.Names[i] = d.Name
	}
	for month := 1; total > 0; month++ {
		if month > limit {
			s.OK = false
			break
		}
		paid := int64(0)
		cal := Month{Payments: make([]int64, len(order))}
		for i, d := range order {
			interest := bal[i] * int64(d.MonthlyRateBP) / 10000
			cal.InterestCents += interest
			bal[i] += interest
		}
		s.InterestCents += cal.InterestCents
		for i, d := range order {
			p := min(d.MinPaymentCents, bal[i])
			bal[i] -= p
			paid += p
			cal.Payments[i] = p
		}
		if rollover {
			rest := budget - paid
			for i := range order {
				p := min(rest, bal[i])
				bal[i] -= p
				rest -= p
				cal.Payments[i] += p
			}
		}
		s.Calendar = append(s.Calendar, cal)
		prev := total
		total = 0
		for i, d := range order {
			total += bal[i]
			if bal[i] == 0 && !done[i] && d.BalanceCents > 0 {
				done[i] = true
				s.PaidOff = append(s.PaidOff, PaidOff{d.Name, month})
			}
		}
		s.BalanceCents = append(s.BalanceCents, total)
		if rollover && total >= prev { // o que se paga não cobre os juros: nunca acaba
			s.OK = false
			break
		}
		s.Months = month
	}
	return s
}
