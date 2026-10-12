package finance

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// ledger monta um livro-caixa de teste com categorias fixas.
func ledger(txs ...domain.Transaction) domain.Ledger {
	cat := func(id, name, bucket string) domain.Category {
		return domain.Category{Meta: domain.Meta{ID: id, UpdatedAt: 1}, Name: name, Bucket: bucket}
	}
	for i := range txs {
		txs[i].ID = string(rune('a' + i))
		txs[i].UpdatedAt = 1
	}
	return domain.Ledger{
		Categories: []domain.Category{
			cat("sal", "Salário", domain.BucketRenda),
			cat("mor", "Moradia", domain.BucketNecessidades),
			cat("mer", "Mercado", domain.BucketNecessidades),
			cat("laz", "Lazer", domain.BucketDesejos),
			cat("inv", "Investimentos", domain.BucketInvestimentos),
			cat("div", domain.DebtPaymentCategory, domain.BucketNecessidades),
		},
		Transactions: txs,
	}
}

func tx(kind, cat string, cents int64, date string) domain.Transaction {
	return domain.Transaction{Kind: kind, Description: cat, AmountCents: cents, CategoryID: cat, Date: date}
}

var oct = domain.NewMonth(2026, time.October)

func TestSummarize(t *testing.T) {
	l := ledger(
		tx(domain.KindEntrada, "sal", 500000, "2026-10-05"),
		tx(domain.KindSaida, "mor", 150000, "2026-10-06"),
		tx(domain.KindSaida, "mer", 100000, "2026-10-07"),
		tx(domain.KindSaida, "laz", 200000, "2026-10-08"),
		tx(domain.KindSaida, "inv", 50000, "2026-10-09"),
		tx(domain.KindSaida, "laz", 999999, "2026-09-30"), // outro mês
	)
	deleted := tx(domain.KindSaida, "laz", 777, "2026-10-10")
	deleted.ID, deleted.UpdatedAt, deleted.Deleted = "x", 1, true
	l.Transactions = append(l.Transactions, deleted)

	s := Summarize(oct, l)
	if s.InCents != 500000 || s.OutCents != 500000 || s.BalanceCents() != 0 {
		t.Fatalf("totais errados: %+v", s)
	}
	want := []BucketLine{
		{Bucket: "necessidades", Pct: 50, PlannedCents: 250000, SpentCents: 250000},
		{Bucket: "desejos", Pct: 30, PlannedCents: 150000, SpentCents: 200000},
		{Bucket: "investimentos", Pct: 20, PlannedCents: 100000, SpentCents: 50000},
	}
	for i, w := range want {
		if s.Buckets[i] != w {
			t.Errorf("bucket %d = %+v, want %+v", i, s.Buckets[i], w)
		}
	}
	if s.Buckets[1].UsedPct() != 133 || s.Buckets[1].RemainingCents() != -50000 {
		t.Errorf("desejos estourado calculado errado")
	}
	if s.Categories[0].Name != "Lazer" || s.Categories[0].SharePct != 40 {
		t.Errorf("maior categoria = %+v", s.Categories[0])
	}
	if s.SavingsRatePct() != 10 {
		t.Errorf("taxa de poupança = %d, want 10", s.SavingsRatePct())
	}
}

func TestSummarizeUsesSavedSplit(t *testing.T) {
	l := ledger(tx(domain.KindEntrada, "sal", 100000, "2026-10-01"))
	v, _ := json.Marshal(domain.Split{"necessidades": 70, "desejos": 10, "investimentos": 20})
	l.Prefs = []domain.Pref{{Meta: domain.Meta{ID: domain.PrefSplit, UpdatedAt: 1}, Value: v}}
	if b := Summarize(oct, l).Buckets[0]; b.Pct != 70 || b.PlannedCents != 70000 {
		t.Errorf("divisão salva não aplicada: %+v", b)
	}
}

func TestYearAndRecentTotals(t *testing.T) {
	l := ledger(
		tx(domain.KindEntrada, "sal", 450000, "2026-09-05"),
		tx(domain.KindSaida, "mor", 180000, "2026-09-10"),
		tx(domain.KindEntrada, "sal", 450000, "2026-10-05"),
		tx(domain.KindSaida, "mor", 100, "2025-12-31"),
	)
	year := YearTotals(l, 2026)
	if len(year) != 12 || year[8].InCents != 450000 || year[8].OutCents != 180000 || year[8].BalanceCents != 270000 || year[8].Key != "2026-09" {
		t.Errorf("setembro = %+v", year[8])
	}
	recent := RecentTotals(l, oct, 10) // dez/2025 a out/2026: atravessa a virada do ano
	if len(recent) != 3 || recent[0].Key != "2025-12" || recent[2].Key != "2026-10" {
		t.Errorf("recentes = %+v", recent)
	}
}

func TestPayoff(t *testing.T) {
	if m, i, ok := Payoff(100000, 0, 10000); !ok || m != 10 || i != 0 {
		t.Errorf("sem juros: %d meses, %d juros, ok=%v", m, i, ok)
	}
	if m, i, ok := Payoff(100000, 500, 10000); !ok || m <= 10 || i <= 0 {
		t.Errorf("com juros: %d meses, %d juros, ok=%v", m, i, ok)
	}
	if _, _, ok := Payoff(1000000, 1500, 10000); ok {
		t.Error("esperava ok=false quando o pagamento não cobre os juros")
	}
}

func debts() []domain.Debt {
	return []domain.Debt{
		{Meta: domain.Meta{ID: "d1"}, Name: "Empréstimo", BalanceCents: 500000, MonthlyRateBP: 250, MinPaymentCents: 40000},
		{Meta: domain.Meta{ID: "d2"}, Name: "Cartão", BalanceCents: 320000, MonthlyRateBP: 1400, MinPaymentCents: 32000},
	}
}

func TestPlanDebts(t *testing.T) {
	p := PlanDebts(debts())
	if p.TotalCents != 820000 || p.TotalMinCents != 72000 || p.MonthlyInterest != 57300 {
		t.Errorf("totais: %+v", p)
	}
	if p.Avalanche[0].Name != "Cartão" || p.Snowball[0].Name != "Cartão" {
		t.Errorf("ordem: avalanche=%s snowball=%s", p.Avalanche[0].Name, p.Snowball[0].Name)
	}
	for _, d := range p.Debts {
		if d.Name == "Cartão" && (d.MonthlyInterestCents != 44800 || d.Payable) {
			t.Errorf("cartão = %+v", d)
		}
	}
}

func TestPlanEmergencyFund(t *testing.T) {
	f := PlanEmergencyFund([]Totals{
		{InCents: 450000, OutCents: 355000, BalanceCents: 95000},
		{},
		{InCents: 450000, OutCents: 345000, BalanceCents: 105000},
	}, 0)
	want := EmergencyFund{AvgMonthlyOutCents: 350000, MinCents: 1050000, IdealCents: 2100000,
		MonthlySavingCents: 100000, MonthsToMin: 11, MonthsToIdeal: 21}
	if f != want {
		t.Errorf("reserva = %+v, want %+v", f, want)
	}
	if withDebts := PlanEmergencyFund([]Totals{{InCents: 450000, OutCents: 350000, BalanceCents: 100000}}, 72000); withDebts.MonthlySavingCents != 28000 || withDebts.MonthsToMin != 38 {
		t.Errorf("parcelas pendentes devem sair da sobra: %+v", withDebts)
	}
	if PlanEmergencyFund(nil, 0) != (EmergencyFund{}) {
		t.Error("sem dados deve ser zero")
	}
}

func TestPlanMonth(t *testing.T) {
	l := ledger(
		tx(domain.KindEntrada, "sal", 450000, "2026-10-05"),
		tx(domain.KindSaida, "mor", 355000, "2026-10-06"),
	)
	plan := PlanDebts(debts())
	p := PlanMonth(Summarize(oct, l), plan)
	if p.SurplusCents != 95000 || p.PendingMinCents != 72000 || p.FreeCents != 23000 {
		t.Fatalf("plano = %+v", p)
	}
	if p.Target == nil || p.Target.Name != "Cartão" || p.TargetPaymentCents != 55000 || !p.TargetPayable {
		t.Errorf("alvo = %+v", p)
	}
	// Parcelas já lançadas na categoria de pagamento de dívidas não contam como pendentes.
	l = ledger(
		tx(domain.KindEntrada, "sal", 450000, "2026-10-05"),
		tx(domain.KindSaida, "mor", 283000, "2026-10-06"),
		tx(domain.KindSaida, "div", 72000, "2026-10-10"),
	)
	if p := PlanMonth(Summarize(oct, l), plan); p.PendingMinCents != 0 || p.FreeCents != 95000 {
		t.Errorf("com parcelas lançadas = %+v", p)
	}
}

func TestViews(t *testing.T) {
	l := ledger(tx(domain.KindEntrada, "sal", 100000, "2026-10-01"), tx(domain.KindSaida, "laz", 40000, "2026-10-02"))
	l.Debts = debts()
	d := Dashboard(l, oct)
	if d.Month != "2026-10" || d.Label != "outubro de 2026" || d.BalanceCents != 60000 || len(d.Buckets) != 3 || len(d.Year) != 12 {
		t.Errorf("painel = %+v", d)
	}
	if d.Buckets[1].RemainingCents != -10000 || d.Buckets[1].UsedPct != 133 {
		t.Errorf("desejos = %+v", d.Buckets[1])
	}
	b, _ := json.Marshal(d)
	var back map[string]any
	json.Unmarshal(b, &back)
	if back["inCents"] != float64(100000) {
		t.Errorf("json do painel = %s", b)
	}

	dv := Debts(l)
	if len(dv.Debts) != 2 || dv.Avalanche[0] != "Cartão" || dv.Debts[0].Name != "Cartão" || dv.Debts[0].MonthlyInterestCents != 44800 {
		t.Errorf("dívidas = %+v", dv)
	}
	if empty := Dashboard(domain.Ledger{}, oct); empty.Buckets == nil || empty.Categories == nil {
		t.Error("listas vazias devem virar [] no JSON, não null")
	}
}
