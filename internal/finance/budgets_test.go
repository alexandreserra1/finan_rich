package finance

import (
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestBudgetsAndAlerts(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	cat := func(id, name string, budget int64) domain.Category {
		return domain.Category{Meta: meta(id), Name: name, Bucket: domain.BucketDesejos, BudgetCents: budget}
	}
	spend := func(id, cat string, cents int64, date string) domain.Transaction {
		return domain.Transaction{Meta: meta(id), Kind: domain.KindSaida, Description: "x", AmountCents: cents, CategoryID: cat, Date: date}
	}
	l := domain.Ledger{
		Categories: []domain.Category{
			cat("res", "Restaurantes", 40000), // 80%: aviso
			cat("laz", "Lazer", 20000),        // 79%: sem aviso
			cat("com", "Compras", 10000),      // 100%: acabou
			cat("ass", "Assinaturas", 5000),   // 120%: estourou
			cat("mer", "Mercado", 0),          // sem orçamento
		},
		Transactions: []domain.Transaction{
			spend("r1", "res", 30000, "2026-10-02"), spend("r2", "res", 2000, "2026-10-10"),
			spend("r0", "res", 90000, "2026-09-10"), // mês passado não conta
			spend("l1", "laz", 15800, "2026-10-03"),
			spend("c1", "com", 10000, "2026-10-04"),
			spend("a1", "ass", 6000, "2026-10-05"),
			spend("m1", "mer", 80000, "2026-10-05"),
			{Meta: meta("e1"), Kind: domain.KindEntrada, Description: "Estorno", AmountCents: 50000, CategoryID: "res", Date: "2026-10-06"},
		},
	}
	got := Budgets(l, domain.NewMonth(2026, 10))
	if len(got) != 4 {
		t.Fatalf("orçamentos = %+v", got)
	}
	if b := got[0]; b.Name != "Assinaturas" || b.UsedPct != 120 || b.RemainingCents != -1000 {
		t.Errorf("mais usado primeiro: %+v", b)
	}
	if b := got[2]; b.Name != "Restaurantes" || b.SpentCents != 32000 || b.UsedPct != 80 || b.RemainingCents != 8000 {
		t.Errorf("restaurantes: %+v", b)
	}

	alerts := map[string]Alert{}
	for _, a := range Alerts(l, today) {
		if a.Kind == "budget-near" || a.Kind == "budget-over" {
			alerts[a.Title] = a
		}
	}
	want := map[string]string{
		"Orçamento de Restaurantes em 80%":  "warn",
		"Orçamento de Compras acabou":       "danger",
		"Orçamento de Assinaturas estourou": "danger",
	}
	if len(alerts) != len(want) {
		t.Errorf("avisos de orçamento = %+v", alerts)
	}
	for title, level := range want {
		if a, ok := alerts[title]; !ok || a.Level != level || a.Link == "" {
			t.Errorf("falta %q (%s): %+v", title, level, alerts)
		}
	}
	if a := alerts["Orçamento de Restaurantes em 80%"]; a.Text != "Restam R$ 80,00 de R$ 400,00 no mês." || a.Link != "/lancamentos?categoria=res" {
		t.Errorf("texto do aviso: %+v", a)
	}
	if d := Dashboard(l, domain.NewMonth(2026, 10)); len(d.Budgets) != 4 {
		t.Errorf("painel sem orçamentos: %+v", d.Budgets)
	}
	if v := Insights(l, today); len(v.Budgets) != 4 {
		t.Errorf("análise sem orçamentos: %+v", v.Budgets)
	}
}
