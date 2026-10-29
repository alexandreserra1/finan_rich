package finance

import (
	"fmt"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Cenário: hoje é 15/10/2026. Jul–set iguais (salário, aluguel recorrente,
// 3 iFood de R$ 100, Netflix). Em outubro: 10 iFood de R$ 50, Netflix, Uber,
// um investimento e o aluguel de outubro ainda não lançado. Nos últimos 30 dias
// (desde 16/09) entram também o iFood de 20/09: 11 compras, R$ 600.
func insightsLedger() domain.Ledger {
	m := func(id string) domain.Meta { return domain.Meta{ID: id, UpdatedAt: 1} }
	l := domain.Ledger{
		Categories: []domain.Category{
			{Meta: m("sal"), Name: "Salário", Bucket: domain.BucketRenda},
			{Meta: m("res"), Name: "Restaurantes e delivery", Bucket: domain.BucketDesejos},
			{Meta: m("laz"), Name: "Lazer", Bucket: domain.BucketDesejos},
			{Meta: m("tra"), Name: "Transporte", Bucket: domain.BucketNecessidades},
			{Meta: m("mor"), Name: "Moradia", Bucket: domain.BucketNecessidades},
			{Meta: m("inv"), Name: "Investimentos", Bucket: domain.BucketInvestimentos},
		},
		Recurring: []domain.Recurring{{Meta: m("r1"), Kind: domain.KindSaida, Description: "Aluguel", AmountCents: 150000, CategoryID: "mor", Day: 5, Start: "2026-07"}},
	}
	tx := func(id, kind, desc string, cents int64, cat, date string) {
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: m(id), Kind: kind, Description: desc, AmountCents: cents, CategoryID: cat, Date: date})
	}
	for _, mk := range []string{"2026-07", "2026-08", "2026-09"} {
		tx("sal-"+mk, domain.KindEntrada, "Salário", 500000, "sal", mk+"-05")
		tx(domain.RecurringTxID("r1", domain.ParseMonth(mk)), domain.KindSaida, "Aluguel", 150000, "mor", mk+"-05")
		for i, d := range []string{"03", "10", "20"} {
			tx(fmt.Sprintf("if-%s-%d", mk, i), domain.KindSaida, "iFood pedido", 10000, "res", mk+"-"+d)
		}
		tx("nf-"+mk, domain.KindSaida, "NETFLIX.COM", 5590, "laz", mk+"-10")
	}
	tx("sal-out", domain.KindEntrada, "Salário", 500000, "sal", "2026-10-05")
	for i := 1; i <= 10; i++ {
		tx(fmt.Sprintf("if-out-%d", i), domain.KindSaida, "iFood pedido", 5000, "res", fmt.Sprintf("2026-10-%02d", i))
	}
	tx("nf-out", domain.KindSaida, "NETFLIX.COM", 5590, "laz", "2026-10-10")
	tx("uber", domain.KindSaida, "Uber *Trip", 2500, "tra", "2026-10-12")
	tx("cdb", domain.KindSaida, "CDB", 50000, "inv", "2026-10-06")
	return l
}

func TestInsights(t *testing.T) {
	v := Insights(insightsLedger(), time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	check := func(name string, got, want int64) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %d, esperado %d", name, got, want)
		}
	}
	check("variáveis até hoje", v.VariableCents, 58090)             // 10×50 + 55,90 + 25 (investimento não conta)
	check("fixos (aluguel a cair)", v.FixedCents, 150000)           // recorrente de outubro ainda sem lançamento
	check("projeção", v.ProjectedCents, 58090*31/15+150000)         // ritmo × 31 dias + fixos
	check("mês passado", v.LastMonthCents, 185590)                  // 300 + 55,90 + 1.500
	check("mês passado até dia 15", v.LastMonthSameDayCents, 25590) // dias 3 e 10 + Netflix (sem o aluguel fixo)
	check("média 3 meses", v.Avg3Cents, 185590)
	check("livre", v.FreeCents, 500000-100000-58090-150000) // renda − 20% planejado − gastos − aluguel a cair
	check("por dia", v.PerDayCents, (500000-100000-58090-150000)/17)
	if len(v.Pace) != 15 || v.Pace[14] != 58090 || v.Pace[0] != 5000 || len(v.LastPace) != 30 {
		t.Errorf("ritmo = %v (mês passado %d dias)", v.Pace, len(v.LastPace))
	}
	if len(v.Places) != 3 || v.Places[0].Name != "iFood" || v.Places[0].Cents != 50000 || v.Places[0].Count != 10 || v.Places[0].Avg3Cents != 30000 {
		t.Errorf("lugares = %+v", v.Places)
	}
	if c := v.Categories[0]; c.Name != "Restaurantes e delivery" || c.Cents != 50000 || c.Avg3Cents != 30000 || c.DiffPct != 66 {
		t.Errorf("categorias = %+v", v.Categories)
	}
	kinds := []string{}
	for _, s := range v.Savings {
		kinds = append(kinds, fmt.Sprintf("%s:%d", s.Kind, s.MonthlyCents))
	}
	if fmt.Sprint(kinds) != fmt.Sprint([]string{"ritmo:84462", "frequente:30000", "categoria:20000", "assinaturas:0"}) {
		t.Errorf("economias = %v", kinds)
	}
	if len(v.Subscriptions) != 2 || v.Subscriptions[0].Name != "Aluguel" || !v.Subscriptions[0].Registered ||
		v.Subscriptions[1].Name != "NETFLIX.COM" || v.Subscriptions[1].YearlyCents != 67080 {
		t.Errorf("assinaturas = %+v", v.Subscriptions)
	}
	if len(v.Biggest) != 5 || v.Biggest[0].Description != "NETFLIX.COM" {
		t.Errorf("maiores = %+v", v.Biggest)
	}
}

// Sem histórico e sem renda: nada quebra, e não inventa sugestões.
func TestInsightsEmpty(t *testing.T) {
	v := Insights(domain.Ledger{}, time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC))
	if v.DaysInMonth != 28 || v.PerDayCents != 0 || len(v.Savings) != 0 || len(v.Pace) != 1 || v.Places == nil || v.Biggest == nil {
		t.Errorf("vazio = %+v", v)
	}
}

// Com só 2 meses de histórico, a média é desses 2 (e não dividida por 3):
// aluguel igual todo mês não pode virar "acima do normal".
func TestUnusualSpendingShortHistory(t *testing.T) {
	l := insightsLedger()
	var keep []domain.Transaction
	for _, tx := range l.Transactions {
		if tx.Date[:7] != "2026-07" {
			keep = append(keep, tx)
		}
	}
	l.Transactions = keep
	// Aluguel de outubro lançado, igual aos outros meses.
	l.Transactions = append(l.Transactions, domain.Transaction{Meta: domain.Meta{ID: domain.RecurringTxID("r1", domain.ParseMonth("2026-10")), UpdatedAt: 1},
		Kind: domain.KindSaida, Description: "Aluguel", AmountCents: 150000, CategoryID: "mor", Date: "2026-10-05"})
	for _, a := range Alerts(l, time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)) {
		if a.Kind == "unusual" && a.Title == "Moradia acima do normal" {
			t.Errorf("alarme falso: %+v", a)
		}
	}
	if c := Insights(l, time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)).Categories; c[0].Name != "Moradia" || c[0].DiffPct != 0 {
		t.Errorf("categorias = %+v", c)
	}
}

func TestInsightsAvgPaceAndSavingsRate(t *testing.T) {
	v := Insights(insightsLedger(), time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if len(v.Avg3Pace) != 31 || v.Avg3Pace[2] != 10000 || v.Avg3Pace[9] != 25590 || v.Avg3Pace[30] != 35590 {
		t.Errorf("ritmo médio de 3 meses = %v", v.Avg3Pace)
	}
	var got []string
	for _, r := range v.SavingsRates {
		got = append(got, fmt.Sprintf("%s:%d:%v", r.Month, r.Pct, r.HasIncome))
	}
	if fmt.Sprint(got) != "[2026-05:0:false 2026-06:0:false 2026-07:62:true 2026-08:62:true 2026-09:62:true 2026-10:88:true]" {
		t.Errorf("taxa por mês = %v", got)
	}
	if v.SavingsPct != 88 || v.SavingsAvgPct != 62 || v.SavingsTargetPct != 20 {
		t.Errorf("taxa = %d, média = %d, meta = %d", v.SavingsPct, v.SavingsAvgPct, v.SavingsTargetPct)
	}
}
