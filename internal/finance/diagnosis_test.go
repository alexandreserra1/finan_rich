package finance

import (
	"fmt"
	"strings"
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
	// Sobra: R$ 3.144,10 por mês, menos as parcelas mínimas (R$ 480 + R$ 610) que não
	// foram lançadas; o plano das dívidas já conta com elas.
	want := []string{"sobra Sobra dos últimos meses 205410", "frequente iFood 30000", "assinatura NETFLIX.COM 5590"}
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

// Com um mês fechado só, o texto fala no singular.
func TestDiagnoseOneClosedMonth(t *testing.T) {
	l := ledger(tx(domain.KindEntrada, "sal", 600000, "2026-10-05"), tx(domain.KindSaida, "mer", 100000, "2026-10-06"))
	l.Debts = []domain.Debt{{Meta: domain.Meta{ID: "d", UpdatedAt: 1}, Name: "Cartão", BalanceCents: 100000, MonthlyRateBP: 1000, MinPaymentCents: 20000}}
	for _, lv := range Diagnose(l, time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)).Levers {
		if lv.Kind == "sobra" && !strings.HasPrefix(lv.Text, "No último mês sobraram R$ 4.800,00 (entradas menos saídas, menos R$ 200,00 de parcelas mínimas") {
			t.Errorf("texto = %q", lv.Text)
		}
	}
}

// A parcela mínima que não foi lançada não é sobra: o plano das dívidas já paga o
// mínimo e soma a sobra por cima (achado da revisão do corpus, 26/11/2026).
func TestDiagnoseUnpaidMinimumIsNotSurplus(t *testing.T) {
	m := func(id string) domain.Meta { return domain.Meta{ID: id, UpdatedAt: 1} }
	l := domain.Ledger{
		Categories: []domain.Category{
			{Meta: m("sal"), Name: "Salário", Bucket: domain.BucketRenda},
			{Meta: m("mer"), Name: "Mercado", Bucket: domain.BucketNecessidades},
			{Meta: m("pd"), Name: domain.DebtPaymentCategory, Bucket: domain.BucketNecessidades},
		},
		Debts: []domain.Debt{{Meta: m("emp"), Name: "Empréstimo Exemplo", BalanceCents: 240000, MonthlyRateBP: 900, MinPaymentCents: 30000, DueDay: 10}},
	}
	tx := func(id, kind, desc string, cents int64, cat, date string) {
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: m(id), Kind: kind, Description: desc, AmountCents: cents, CategoryID: cat, Date: date})
	}
	tx("sal", domain.KindEntrada, "Salário", 385000, "sal", "2026-09-05")
	tx("mer", domain.KindSaida, "Mercado Exemplo", 46990, "mer", "2026-09-08")
	today := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	surplus := func() (int64, string) {
		for _, lv := range Diagnose(l, today).Levers {
			if lv.Kind == "sobra" {
				return lv.MonthlyCents, lv.Text
			}
		}
		return 0, ""
	}
	if got, text := surplus(); got != 385000-46990-30000 || !strings.Contains(text, "R$ 300,00 de parcelas mínimas sem pagamento lançado") {
		t.Errorf("sem o pagamento lançado: sobra %d (%s)", got, text)
	}
	tx("pag", domain.KindSaida, "Pagamento: Empréstimo Exemplo", 30000, "pd", "2026-09-10")
	if got, text := surplus(); got != 385000-46990-30000 || strings.Contains(text, "sem pagamento lançado") {
		t.Errorf("com o pagamento lançado, não desconta de novo: sobra %d (%s)", got, text)
	}
}
