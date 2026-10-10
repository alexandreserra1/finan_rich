package finance

import (
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/store"
)

func TestSummarize(t *testing.T) {
	m := store.NewMonth(2026, time.October)
	txs := []store.Transaction{
		{Kind: store.KindEntrada, AmountCents: 500000, CategoryName: "Salário", Bucket: store.BucketRenda},
		{Kind: store.KindSaida, AmountCents: 150000, CategoryName: "Moradia", Bucket: store.BucketNecessidades},
		{Kind: store.KindSaida, AmountCents: 100000, CategoryName: "Mercado", Bucket: store.BucketNecessidades},
		{Kind: store.KindSaida, AmountCents: 200000, CategoryName: "Lazer", Bucket: store.BucketDesejos},
		{Kind: store.KindSaida, AmountCents: 50000, CategoryName: "Investimentos", Bucket: store.BucketInvestimentos},
	}
	s := Summarize(m, txs, map[string]int{"necessidades": 50, "desejos": 30, "investimentos": 20})

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
		t.Errorf("desejos estourado calculado errado: %d%% %d", s.Buckets[1].UsedPct(), s.Buckets[1].RemainingCents())
	}
	if s.Categories[0].Name != "Lazer" || s.Categories[0].SharePct != 40 {
		t.Errorf("maior categoria = %+v", s.Categories[0])
	}
	if s.SavingsRatePct() != 10 {
		t.Errorf("taxa de poupança = %d, want 10", s.SavingsRatePct())
	}
}

func TestPayoff(t *testing.T) {
	// R$ 1.000 sem juros pagando R$ 100: 10 meses.
	if m, i, ok := Payoff(100000, 0, 10000); !ok || m != 10 || i != 0 {
		t.Errorf("sem juros: %d meses, %d juros, ok=%v", m, i, ok)
	}
	// Com juros leva mais tempo e cobra juros.
	if m, i, ok := Payoff(100000, 500, 10000); !ok || m <= 10 || i <= 0 {
		t.Errorf("com juros: %d meses, %d juros, ok=%v", m, i, ok)
	}
	// Pagamento menor que os juros nunca quita (rotativo de cartão).
	if _, _, ok := Payoff(1000000, 1500, 10000); ok {
		t.Error("esperava ok=false quando o pagamento não cobre os juros")
	}
}

func TestPlanDebts(t *testing.T) {
	p := PlanDebts([]store.Debt{
		{Name: "Cartão", BalanceCents: 300000, MonthlyRateBP: 1200, MinPaymentCents: 50000},
		{Name: "Empréstimo", BalanceCents: 100000, MonthlyRateBP: 300, MinPaymentCents: 20000},
	})
	if p.TotalCents != 400000 || p.TotalMinCents != 70000 {
		t.Errorf("totais: %+v", p)
	}
	if p.Avalanche[0].Name != "Cartão" || p.Snowball[0].Name != "Empréstimo" {
		t.Errorf("ordem: avalanche=%s snowball=%s", p.Avalanche[0].Name, p.Snowball[0].Name)
	}
}
