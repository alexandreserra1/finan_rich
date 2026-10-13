package finance

import (
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func meta(id string) domain.Meta { return domain.Meta{ID: id, UpdatedAt: 1} }

func TestGoals(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	l := domain.Ledger{
		Transactions: []domain.Transaction{
			{Meta: meta("a"), Kind: domain.KindSaida, AmountCents: 30000, CategoryID: "res", Date: "2026-10-05"},
			{Meta: meta("antes"), Kind: domain.KindSaida, AmountCents: 99900, CategoryID: "res", Date: "2026-08-01"}, // antes do início
			{Meta: meta("outra"), Kind: domain.KindSaida, AmountCents: 5000, CategoryID: "laz", Date: "2026-10-05"},
		},
		Goals: []domain.Goal{
			{Meta: meta("g1"), Name: "Reserva", TargetCents: 600000, SavedCents: 100000, Deadline: "2026-12", CategoryID: "res", StartDate: "2026-09-01"},
			{Meta: meta("g2"), Name: "Viagem", TargetCents: 50000, SavedCents: 60000, StartDate: "2026-01-01"},
			{Meta: meta("g3"), Name: "Celular", TargetCents: 300000, Deadline: "2026-09", StartDate: "2026-01-01"},
		},
	}
	got := Goals(l, today)
	r := got[0]
	if r.Name != "Celular" || !r.Late {
		t.Errorf("meta vencida deveria vir primeiro e atrasada: %+v", r)
	}
	r = got[1]
	// 1.000 já guardados + 300 de aporte = 1.300 de 6.000; faltam 4.700 em 3 meses (out, nov, dez).
	if r.ContributedCents != 30000 || r.ProgressCents != 130000 || r.Pct != 21 || r.MonthsLeft != 3 || r.PerMonthCents != 156667 {
		t.Errorf("reserva = %+v", r)
	}
	if v := got[2]; !v.Done || v.Pct != 100 || v.RemainingCents != 0 {
		t.Errorf("meta atingida = %+v", v)
	}
}

func TestRecurringDue(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	rec := domain.Recurring{Meta: meta("r1"), Kind: domain.KindSaida, Description: "Aluguel", AmountCents: 140000, CategoryID: "mor", Day: 10, Start: "2026-08"}
	sal := domain.Recurring{Meta: meta("r2"), Kind: domain.KindEntrada, Description: "Salário", AmountCents: 500000, CategoryID: "sal", Day: 31, Start: "2026-09", End: "2026-09"}
	paused := domain.Recurring{Meta: meta("r3"), Kind: domain.KindSaida, Description: "Academia", AmountCents: 9900, CategoryID: "sau", Day: 1, Start: "2026-01", Paused: true}
	late := domain.Recurring{Meta: meta("r4"), Kind: domain.KindSaida, Description: "Internet", AmountCents: 9990, CategoryID: "cas", Day: 20, Start: "2026-10"}
	l := domain.Ledger{
		Recurring: []domain.Recurring{rec, sal, paused, late},
		// Setembro do aluguel já existe (foi gerado e depois apagado: não volta).
		Transactions: []domain.Transaction{{Meta: domain.Meta{ID: "rec-r1-2026-09", UpdatedAt: 2, Deleted: true}}},
	}
	var got []string
	for _, tx := range RecurringDue(l, today) {
		got = append(got, tx.ID+"@"+tx.Date)
	}
	want := "rec-r1-2026-08@2026-08-10,rec-r1-2026-10@2026-10-10,rec-r2-2026-09@2026-09-30"
	if strings.Join(got, ",") != want {
		t.Errorf("RecurringDue = %v\nquer %s", got, want)
	}
}

func TestAlerts(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	tx := func(id, kind, cat string, cents int64, date string) domain.Transaction {
		return domain.Transaction{Meta: meta(id), Kind: kind, Description: id, AmountCents: cents, CategoryID: cat, Date: date}
	}
	l := domain.Ledger{
		Categories: []domain.Category{
			{Meta: meta("sal"), Name: "Salário", Bucket: domain.BucketRenda},
			{Meta: meta("res"), Name: "Restaurantes", Bucket: domain.BucketDesejos},
		},
		Transactions: []domain.Transaction{
			tx("s", domain.KindEntrada, "sal", 300000, "2026-10-05"),
			tx("r10", domain.KindSaida, "res", 120000, "2026-10-06"), // 1.200 > meta de desejos (30% de 3.000 = 900)
			tx("r09", domain.KindSaida, "res", 40000, "2026-09-06"),
			tx("r08", domain.KindSaida, "res", 50000, "2026-08-06"),
			tx("r07", domain.KindSaida, "res", 60000, "2026-07-06"),
		},
		Debts: []domain.Debt{
			{Meta: meta("d1"), Name: "Cartão", BalanceCents: 100000, MinPaymentCents: 20000, MonthlyRateBP: 1000, DueDay: 17},
			{Meta: meta("d2"), Name: "Cheque especial", BalanceCents: 500000, MinPaymentCents: 10000, MonthlyRateBP: 800, DueDay: 15},
		},
	}
	var got []string
	for _, a := range Alerts(l, today) {
		got = append(got, a.Level+": "+a.Title+" — "+a.Text)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{
		"danger: Cheque especial vence hoje",
		"danger: Cheque especial está crescendo",
		"warn: Cartão vence em 2 dias (dia 17)",
		"warn: Desejos passou da meta do mês — Gasto R$ 1.200,00 de R$ 900,00 planejados (133%).",
		"warn: Restaurantes acima do normal — R$ 1.200,00 este mês, 140% acima da média dos últimos 3 meses (R$ 500,00).",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("sem o alerta %q:\n%s", want, all)
		}
	}
	if !strings.HasPrefix(got[0], "danger") || !strings.HasPrefix(got[len(got)-1], "warn") {
		t.Errorf("os alertas graves vêm primeiro:\n%s", all)
	}
	if a := Alerts(domain.Ledger{}, today); a == nil || len(a) != 0 {
		t.Error("sem dados, lista vazia (não nula)")
	}
}
