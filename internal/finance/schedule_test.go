package finance

import (
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
