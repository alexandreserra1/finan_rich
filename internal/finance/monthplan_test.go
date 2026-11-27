package finance

import (
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestPlanMonthLimitaPagamentoAoPrimeiroPeriodo(test *testing.T) {
	debtPlan := PlanDebts([]domain.Debt{{Name: "Empréstimo", BalanceCents: 240000, MonthlyRateBP: 900, MinPaymentCents: 30000}})
	plan := PlanMonth(Summary{InCents: 360000, OutCents: 43990}, debtPlan, 0, 0)
	if plan.TargetPaymentCents != 261600 {
		test.Fatalf("pagamento = %d; esperado saldo 240000 + juros 21600", plan.TargetPaymentCents)
	}
	if !plan.TargetPayable || plan.TargetMonths != 1 || plan.TargetInterest != 21600 {
		test.Fatalf("simulação inesperada: %+v", plan)
	}
}

func TestPendingRecurringExpensesRespeitaEstadoERegistro(test *testing.T) {
	base := domain.Recurring{Meta: domain.Meta{ID: "fixa"}, Kind: domain.KindSaida, AmountCents: 9000, Day: 5, Start: "2026-01"}
	for _, scenario := range []struct {
		name string
		edit func(*domain.Ledger)
		want int64
	}{
		{"pendente", func(book *domain.Ledger) {}, 9000},
		{"pausada", func(book *domain.Ledger) { book.Recurring[0].Paused = true }, 0},
		{"futura", func(book *domain.Ledger) { book.Recurring[0].Start = "2026-11" }, 0},
		{"encerrada", func(book *domain.Ledger) { book.Recurring[0].End = "2026-09" }, 0},
		{"apagada", func(book *domain.Ledger) { book.Recurring[0].Deleted = true }, 0},
		{"renda", func(book *domain.Ledger) { book.Recurring[0].Kind = domain.KindEntrada }, 0},
		{"lançada", func(book *domain.Ledger) {
			book.Transactions = []domain.Transaction{{Meta: domain.Meta{ID: domain.RecurringTxID("fixa", oct)}}}
		}, 0},
		{"lançamento apagado", func(book *domain.Ledger) {
			book.Transactions = []domain.Transaction{{Meta: domain.Meta{ID: domain.RecurringTxID("fixa", oct), Deleted: true}}}
		}, 0},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			book := domain.Ledger{Recurring: []domain.Recurring{base}}
			scenario.edit(&book)
			if got := PendingRecurringExpenses(book, oct); got != scenario.want {
				test.Fatalf("reserva = %d; esperado %d", got, scenario.want)
			}
		})
	}
}

func TestPlanMonthNaoRepeteMinimoPago(test *testing.T) {
	debtPlan := PlanDebts([]domain.Debt{{Name: "Empréstimo", BalanceCents: 240000, MonthlyRateBP: 900, MinPaymentCents: 30000}})
	summary := Summary{InCents: 100000, OutCents: 60000,
		Categories: []CategoryLine{{Name: domain.DebtPaymentCategory, SpentCents: 30000}}}
	plan := PlanMonth(summary, debtPlan, 0, 0)
	if plan.TargetPaymentCents != 40000 {
		test.Fatalf("pagamento = %d; só há 40000 disponíveis e o mínimo já foi lançado", plan.TargetPaymentCents)
	}
}

func TestPlanMonthReservaMinimosSemAtribuirPagamentoAgregado(test *testing.T) {
	debts := PlanDebts([]domain.Debt{
		{Name: "Prioridade", BalanceCents: 240000, MonthlyRateBP: 900, MinPaymentCents: 30000},
		{Name: "Outra", BalanceCents: 100000, MonthlyRateBP: 200, MinPaymentCents: 20000},
	})
	summary := Summary{InCents: 200000, OutCents: 30000,
		Categories: []CategoryLine{{Name: domain.DebtPaymentCategory, SpentCents: 30000}}}
	plan := PlanMonth(summary, debts, 0, 0)
	if plan.PendingMinCents != 20000 || plan.TargetBudgetCents != 150000 {
		test.Fatalf("pagamento agregado não identifica a dívida; preserve os mínimos restantes: %+v", plan)
	}
}

func TestPlanMonthNaoReservaMinimoAcimaDoDevidoSimulado(test *testing.T) {
	debts := PlanDebts([]domain.Debt{{Name: "Pequena", BalanceCents: 10000, MonthlyRateBP: 1000, MinPaymentCents: 50000}})
	plan := PlanMonth(Summary{InCents: 20000}, debts, 0, 0)
	if plan.PendingMinCents != 11000 || plan.FreeCents != 9000 || plan.TargetPaymentCents != 11000 {
		test.Fatalf("mínimo ou pagamento acima do saldo com juros simulados: %+v", plan)
	}
}

func TestPlanMonthIgnoraMinimoDeSaldoZerado(test *testing.T) {
	debts := PlanDebts([]domain.Debt{{Name: "Quitada", BalanceCents: 0, MonthlyRateBP: 1000, MinPaymentCents: 50000}})
	plan := PlanMonth(Summary{InCents: 20000}, debts, 0, 0)
	if plan.PendingMinCents != 0 || plan.FreeCents != 20000 || plan.Target != nil {
		test.Fatalf("saldo zerado não gera mínimo pendente: %+v", plan)
	}
}

func TestPlanMonthNaoExcedeCaixaNemDevidoSimulado(test *testing.T) {
	debts := PlanDebts([]domain.Debt{
		{Name: "Prioridade", BalanceCents: 240000, MonthlyRateBP: 900, MinPaymentCents: 30000},
		{Name: "Outra", BalanceCents: 100000, MonthlyRateBP: 200, MinPaymentCents: 20000},
	})
	for _, income := range []int64{0, 10000, 240000, 400000} {
		for _, paid := range []int64{0, 15000, 50000, 60000} {
			for _, fixed := range []int64{0, 9000, 400000} {
				summary := Summary{InCents: income, OutCents: paid,
					Categories: []CategoryLine{{Name: domain.DebtPaymentCategory, SpentCents: paid}}}
				plan := PlanMonth(summary, debts, fixed, 0)
				if plan.TargetBudgetCents > max(income-paid-fixed, 0) || plan.TargetPaymentCents > 261600 || plan.TargetPaymentCents < 0 {
					test.Fatalf("excedeu caixa ou saldo com juros; renda=%d pago=%d fixos=%d: %+v", income, paid, fixed, plan)
				}
				if paid > 0 && plan.TargetBudgetCents > max(plan.FreeCents, 0) {
					test.Fatalf("redistribuiu mínimo sem identificar o pagamento: %+v", plan)
				}
			}
		}
	}
}

// No dia 22, o que costuma ser gasto até o fim do mês fica reservado antes do extra
// para a dívida: sem isso, o plano mandava toda a sobra e não sobrava nada para o
// mercado da semana (revisão do corpus, 26/11/2026).
func TestPlanMonthReservaGastosDoRestoDoMes(test *testing.T) {
	debts := PlanDebts([]domain.Debt{{Meta: domain.Meta{ID: "emp", UpdatedAt: 1}, Name: "Empréstimo", BalanceCents: 600000, MonthlyRateBP: 900, MinPaymentCents: 30000}})
	plan := PlanMonth(Summary{InCents: 385000, OutCents: 50000}, debts, 9000, 120000) // sobra de R$ 3.350,00, parcela ainda não paga
	if plan.PendingVariableCents != 120000 || plan.FreeCents != 335000-9000-30000-120000 {
		test.Fatalf("plano = %+v", plan)
	}
	if plan.TargetBudgetCents != plan.FreeCents+30000 || plan.TargetPaymentCents != plan.TargetBudgetCents {
		test.Errorf("orçamento %d, pagamento %d", plan.TargetBudgetCents, plan.TargetPaymentCents)
	}
}
