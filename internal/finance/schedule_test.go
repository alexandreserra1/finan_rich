package finance

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Conta feita à mão: A (R$ 100, sem juros, mínimo R$ 10) e B (R$ 50, 10% ao
// mês, mínimo R$ 10), com R$ 10 a mais por mês. Avalanche paga B primeiro; quando
// B zera (mês 4), o mínimo dela passa a ir para A.
func TestPayoffSchedule(t *testing.T) {
	ds := []domain.Debt{
		{Name: "A", BalanceCents: 10000, MonthlyRateBP: 0, MinPaymentCents: 1000},
		{Name: "B", BalanceCents: 5000, MonthlyRateBP: 1000, MinPaymentCents: 1000},
	}
	s := PayoffSchedule(ds, 1000, true)
	if !s.OK || s.Months != 6 || s.InterestCents != 1038 {
		t.Fatalf("plano = %d meses, %d de juros, ok=%v", s.Months, s.InterestCents, s.OK)
	}
	if want := []int64{15000, 12500, 9850, 7035, 4038, 1038, 0}; !slices.Equal(s.BalanceCents, want) {
		t.Errorf("saldo mês a mês = %v, want %v", s.BalanceCents, want)
	}
	if want := []PaidOff{{"B", 4}, {"A", 6}}; !slices.Equal(s.PaidOff, want) {
		t.Errorf("quitadas = %v", s.PaidOff)
	}

	// Só o mínimo, cada uma por si (sem passar o que sobra para a outra).
	m := MinimumOnly(ds)
	if !m.OK || m.Months != 10 || m.InterestCents != 2279 || m.BalanceCents[1] != 13500 {
		t.Errorf("só o mínimo = %d meses, %d de juros, ok=%v, %v", m.Months, m.InterestCents, m.OK, m.BalanceCents[:2])
	}
}

func TestPayoffScheduleOrderAndLimits(t *testing.T) {
	ds := []domain.Debt{
		{Name: "Grande barata", BalanceCents: 900000, MonthlyRateBP: 100, MinPaymentCents: 20000},
		{Name: "Pequena cara", BalanceCents: 100000, MonthlyRateBP: 1200, MinPaymentCents: 15000},
		{Name: "Pequena média", BalanceCents: 50000, MonthlyRateBP: 500, MinPaymentCents: 5000},
	}
	if a := PayoffSchedule(ds, 20000, true); a.PaidOff[0].Name != "Pequena cara" {
		t.Errorf("avalanche começa pela de maior juros: %v", a.PaidOff)
	}
	if b := PayoffSchedule(ds, 20000, false); b.PaidOff[0].Name != "Pequena média" {
		t.Errorf("bola de neve começa pela menor: %v", b.PaidOff)
	}
	// O mínimo não cobre os juros: sem dinheiro a mais, nunca acaba; com, acaba.
	grows := []domain.Debt{{Name: "Rotativo", BalanceCents: 300000, MonthlyRateBP: 1250, MinPaymentCents: 10000}}
	if s := PayoffSchedule(grows, 0, true); s.OK || len(s.BalanceCents) > maxMonths+1 {
		t.Errorf("sem dinheiro a mais a dívida cresce: ok=%v, %d meses", s.OK, len(s.BalanceCents))
	}
	if s := PayoffSchedule(grows, 50000, true); !s.OK || s.Months == 0 {
		t.Errorf("com R$ 500 a mais deveria quitar: %+v", s.Months)
	}
	if s := PayoffSchedule(nil, 1000, true); !s.OK || s.Months != 0 || len(s.BalanceCents) != 1 {
		t.Errorf("sem dívidas: %+v", s)
	}
}

// Simulações de começo ao fim com dívidas sorteadas (semente fixa): as contas
// do calendário fecham com os saldos, os juros e as quitações, e pagar mais
// nunca piora o plano.
func TestPayoffScheduleInvariants(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for n := range 300 {
		var ds []domain.Debt
		for i := range 1 + rng.IntN(5) {
			bal := int64(10000 + rng.IntN(5000000)) // R$ 100 a R$ 50 mil
			rate := rng.IntN(1500)                  // 0% a 15% ao mês
			ds = append(ds, domain.Debt{Name: fmt.Sprintf("d%d", i), BalanceCents: bal, MonthlyRateBP: rate, MinPaymentCents: bal/20 + int64(rng.IntN(20000))})
		}
		extra := int64(rng.IntN(100000))
		for _, s := range []Schedule{PayoffSchedule(ds, extra, true), PayoffSchedule(ds, extra, false), MinimumOnly(ds)} {
			if len(s.Calendar) != len(s.BalanceCents)-1 || len(s.Names) != len(ds) {
				t.Fatalf("caso %d: calendário com %d meses e %d saldos", n, len(s.Calendar), len(s.BalanceCents))
			}
			var paid int64
			for m, month := range s.Calendar {
				var p int64
				for _, c := range month.Payments {
					p += c
				}
				paid += p
				if s.BalanceCents[m+1] != s.BalanceCents[m]+month.InterestCents-p {
					t.Fatalf("caso %d, mês %d: saldo %d ≠ %d + juros %d − pago %d", n, m+1, s.BalanceCents[m+1], s.BalanceCents[m], month.InterestCents, p)
				}
			}
			if s.OK && paid != s.BalanceCents[0]+s.InterestCents {
				t.Fatalf("caso %d: pagou %d, devia %d + juros %d", n, paid, s.BalanceCents[0], s.InterestCents)
			}
			if s.OK && (s.BalanceCents[len(s.BalanceCents)-1] != 0 || len(s.PaidOff) != len(ds) || s.PaidOff[len(s.PaidOff)-1].Month != s.Months) {
				t.Fatalf("caso %d: terminou sem zerar tudo: %v", n, s.PaidOff)
			}
		}
		plan, more, min := PayoffSchedule(ds, extra, true), PayoffSchedule(ds, extra+50000, true), MinimumOnly(ds)
		if plan.OK && (!more.OK || more.Months > plan.Months || more.InterestCents > plan.InterestCents) {
			t.Fatalf("caso %d: pagar R$ 500 a mais piorou: %d→%d meses, %d→%d juros", n, plan.Months, more.Months, plan.InterestCents, more.InterestCents)
		}
		if plan.OK && min.OK && plan.InterestCents > min.InterestCents {
			t.Fatalf("caso %d: o plano pagou mais juros (%d) que só o mínimo (%d)", n, plan.InterestCents, min.InterestCents)
		}
	}
}

// Financiamento longo: anos de calendário, e o mês de cada pagamento.
func TestPayoffScheduleYears(t *testing.T) {
	ds := []domain.Debt{{Name: "Financiamento", BalanceCents: 5000000, MonthlyRateBP: 150, MinPaymentCents: 90000}}
	s := PayoffSchedule(ds, 0, true)
	// Pela fórmula da tabela Price: n = −ln(1 − 0,015·50000/900) / ln(1,015) ≈ 120,3 → 121 meses (10 anos e 1 mês).
	if !s.OK || s.Months != 121 {
		t.Fatalf("financiamento de R$ 50 mil a 1,5%% com R$ 900: %d meses, want 121", s.Months)
	}
	last := s.Calendar[len(s.Calendar)-1].Payments[0]
	if s.Calendar[0].Payments[0] != 90000 || last <= 0 || last > 90000 {
		t.Errorf("primeira parcela %d, última %d", s.Calendar[0].Payments[0], last)
	}
}
