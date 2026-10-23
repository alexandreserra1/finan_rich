package finance

import (
	"fmt"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Mesmo cenário da Análise (15/10/2026) com duas dívidas: o diagnóstico acha
// a dívida mais cara, o que mais pesou no mês e onde dá para guardar, sem
// contar a mesma economia duas vezes, e o efeito de cada corte no plano.
func TestDiagnose(t *testing.T) {
	l := insightsLedger()
	l.Debts = []domain.Debt{
		{Meta: domain.Meta{ID: "d1", UpdatedAt: 1}, Name: "Cartão", BalanceCents: 320000, MonthlyRateBP: 1290, MinPaymentCents: 48000},
		{Meta: domain.Meta{ID: "d2", UpdatedAt: 1}, Name: "Empréstimo", BalanceCents: 850000, MonthlyRateBP: 349, MinPaymentCents: 61000},
	}
	d := Diagnose(l, time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	if d.Costliest == nil || d.Costliest.Name != "Cartão" || d.Costliest.MonthlyInterestCents != 41280 {
		t.Errorf("dívida mais cara = %+v", d.Costliest)
	}
	if d.TopCategory == nil || d.TopCategory.Name != "Restaurantes e delivery" || d.TopCategory.Cents != 50000 {
		t.Errorf("o que mais pesou = %+v", d.TopCategory)
	}
	// iFood pela metade (R$ 300) e Restaurantes de volta à média (R$ 200) são a
	// mesma economia: fica a maior. O aluguel (necessidade) não vira corte.
	got := []string{}
	for _, lv := range d.Levers {
		got = append(got, fmt.Sprintf("%s %s %d", lv.Kind, lv.Title, lv.MonthlyCents))
	}
	want := []string{"sobra Sobra dos últimos meses 314410", "frequente iFood 30000", "assinatura NETFLIX.COM 5590"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("cortes = %v\nwant %v", got, want)
	}
	base := PayoffSchedule(l.Debts, 0, true)
	for _, lv := range d.Levers {
		with := PayoffSchedule(l.Debts, lv.MonthlyCents, true)
		if lv.MonthsSooner != base.Months-with.Months || lv.InterestSavedCents != base.InterestCents-with.InterestCents || lv.MonthsSooner <= 0 {
			t.Errorf("%s: %d meses antes e %d de juros, want %d e %d", lv.Title, lv.MonthsSooner, lv.InterestSavedCents, base.Months-with.Months, base.InterestCents-with.InterestCents)
		}
		if lv.ID == "" || lv.Text == "" {
			t.Errorf("corte sem id ou texto: %+v", lv)
		}
	}
}

// Sem dívidas, sem histórico: nada quebra e nada é inventado.
func TestDiagnoseEmpty(t *testing.T) {
	d := Diagnose(domain.Ledger{}, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if d.Costliest != nil || d.TopCategory != nil || len(d.Levers) != 0 {
		t.Errorf("vazio = %+v", d)
	}
}
