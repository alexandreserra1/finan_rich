package finance

import (
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestGoalPlan(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		remaining int64
		deadline  string
		months    int
		perMonth  int64
		late      bool
	}{
		{600000, "2027-03", 6, 100000, false}, // outubro a março: 6 meses, contando o atual
		{100000, "2026-12", 3, 33334, false},  // arredonda para cima
		{100000, "2026-10", 1, 100000, false}, // o mês atual ainda conta
		{100000, "2026-09", 0, 0, true},
		{100000, "", 0, 0, false},
		{0, "2027-03", 0, 0, false}, // nada falta
	} {
		months, per, late := GoalPlan(c.remaining, c.deadline, today)
		if months != c.months || per != c.perMonth || late != c.late {
			t.Errorf("GoalPlan(%d, %q) = %d, %d, %v; quer %d, %d, %v", c.remaining, c.deadline, months, per, late, c.months, c.perMonth, c.late)
		}
	}
}

func TestContribution(t *testing.T) {
	manual := domain.Goal{Meta: meta("g1"), Name: "Viagem", TargetCents: 500000, SavedCents: 10000}
	typ, rec := Contribution(manual, 25000, "2026-10-15")
	if g, ok := rec.(domain.Goal); typ != domain.TypeGoal || !ok || g.SavedCents != 35000 || g.ID != "g1" || manual.SavedCents != 10000 {
		t.Errorf("sem categoria, soma no guardado: %s %+v", typ, rec)
	}
	withCat := domain.Goal{Meta: meta("g2"), Name: "Reserva", TargetCents: 500000, CategoryID: "res"}
	typ, rec = Contribution(withCat, 25000, "2026-10-15")
	if tx, ok := rec.(domain.Transaction); typ != domain.TypeTransaction || !ok || tx.Kind != domain.KindSaida || tx.CategoryID != "res" ||
		tx.AmountCents != 25000 || tx.Date != "2026-10-15" || tx.Description != "Aporte: Reserva" || tx.ID != "" {
		t.Errorf("com categoria, vira saída nela: %s %+v", typ, rec)
	}
}

func TestGoalsPage(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	l := domain.Ledger{
		Categories: []domain.Category{
			{Meta: meta("sal"), Name: "Salário", Bucket: domain.BucketRenda},
			{Meta: meta("mer"), Name: "Mercado", Bucket: domain.BucketNecessidades},
		},
		Transactions: []domain.Transaction{
			{Meta: meta("s"), Kind: domain.KindEntrada, AmountCents: 500000, CategoryID: "sal", Date: "2026-10-05"},
			{Meta: meta("m9"), Kind: domain.KindSaida, AmountCents: 200000, CategoryID: "mer", Date: "2026-09-10"},
			{Meta: meta("m8"), Kind: domain.KindSaida, AmountCents: 100000, CategoryID: "mer", Date: "2026-08-10"},
		},
		Goals: []domain.Goal{
			{Meta: meta("g1"), Name: "Viagem", TargetCents: 600000, Deadline: "2027-03", StartDate: "2026-10-01"},
			{Meta: meta("g2"), Name: "Notebook", TargetCents: 300000, SavedCents: 300000},
		},
	}
	p := GoalsPage(l, today)
	if len(p.Goals) != 2 || p.SavedCents != 300000 || p.RemainingCents != 600000 || p.PerMonthCents != 100000 {
		t.Errorf("somas = %+v", p)
	}
	if p.InvestPlannedCents != 100000 { // 20% de R$ 5.000 na divisão padrão
		t.Errorf("investimentos planejados = %d", p.InvestPlannedCents)
	}
	if p.ReserveCents <= 0 {
		t.Errorf("sugestão de reserva = %d", p.ReserveCents)
	}
}
