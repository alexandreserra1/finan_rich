package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func categoryID(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	cats, err := s.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.Name == name {
			return c.ID
		}
	}
	t.Fatalf("categoria %q não encontrada", name)
	return 0
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.migrate(context.Background()); err != nil {
		t.Fatalf("segunda execução das migrations: %v", err)
	}
	split, err := s.Split(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if split["necessidades"] != 50 || split["desejos"] != 30 || split["investimentos"] != 20 {
		t.Errorf("divisão padrão = %v", split)
	}
}

func TestTransactionsAndTotals(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	salario, aluguel := categoryID(t, s, "Salário"), categoryID(t, s, "Moradia")
	sep := NewMonth(2026, time.September)

	for _, tx := range []Transaction{
		{Kind: KindEntrada, Description: "Salário", AmountCents: 500000, CategoryID: salario, OccurredOn: sep.Day(5)},
		{Kind: KindSaida, Description: "Aluguel", AmountCents: 180000, CategoryID: aluguel, OccurredOn: sep.Day(31)}, // 30/09
		{Kind: KindSaida, Description: "Outro mês", AmountCents: 1, CategoryID: aluguel, OccurredOn: sep.Next().Day(1)},
	} {
		if err := s.CreateTransaction(ctx, tx); err != nil {
			t.Fatal(err)
		}
	}

	txs, err := s.ListTransactions(ctx, sep)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 2 || txs[0].Description != "Aluguel" || txs[0].OccurredOn.Day() != 30 || txs[0].Bucket != BucketNecessidades {
		t.Fatalf("lançamentos de setembro = %+v", txs)
	}

	totals, err := s.MonthlyTotals(ctx, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if got := totals[8]; got.InCents != 500000 || got.OutCents != 180000 || got.BalanceCents != 320000 {
		t.Errorf("totais de setembro = %+v", got)
	}
	if totals[0].InCents != 0 || len(totals) != 12 {
		t.Errorf("janeiro deveria estar zerado: %+v", totals[0])
	}

	if err := s.DeleteCategory(ctx, aluguel); !errors.Is(err, ErrCategoryInUse) {
		t.Errorf("apagar categoria em uso: err = %v", err)
	}
}

func TestCopyFromPreviousMonth(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	cat := categoryID(t, s, "Moradia")
	jan := NewMonth(2026, time.January)
	if err := s.CreateTransaction(ctx, Transaction{Kind: KindSaida, Description: "Aluguel", AmountCents: 100, CategoryID: cat, OccurredOn: jan.Day(31)}); err != nil {
		t.Fatal(err)
	}

	feb := jan.Next()
	for i, want := range []int{1, 0} { // a segunda chamada não pode duplicar
		n, err := s.CopyFromPreviousMonth(ctx, feb)
		if err != nil || n != want {
			t.Fatalf("chamada %d: copiados=%d err=%v, want %d", i+1, n, err, want)
		}
	}
	txs, _ := s.ListTransactions(ctx, feb)
	if len(txs) != 1 || txs[0].OccurredOn.Format("2006-01-02") != "2026-02-28" {
		t.Errorf("cópia = %+v (dia 31 deveria virar 28/02)", txs)
	}
}

func TestDebtsAndChat(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	day := 10
	if err := s.SaveDebt(ctx, Debt{Name: "Cartão", BalanceCents: 1000, MonthlyRateBP: 1200, DueDay: &day}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveDebt(ctx, Debt{Name: "Empréstimo", BalanceCents: 5000, MonthlyRateBP: 200}); err != nil {
		t.Fatal(err)
	}
	debts, err := s.ListDebts(ctx)
	if err != nil || len(debts) != 2 || debts[0].Name != "Cartão" || *debts[0].DueDay != 10 || debts[1].DueDay != nil {
		t.Fatalf("dívidas = %+v, err = %v", debts, err)
	}
	d := debts[1]
	d.BalanceCents = 4000
	if err := s.SaveDebt(ctx, d); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetDebt(ctx, d.ID); got.BalanceCents != 4000 {
		t.Errorf("saldo após editar = %d", got.BalanceCents)
	}

	if err := s.AppendChatTurn(ctx, "oi", "olá!"); err != nil {
		t.Fatal(err)
	}
	msgs, _ := s.ListChat(ctx)
	if len(msgs) != 2 || msgs[0].Role != RoleUser || msgs[1].Content != "olá!" {
		t.Errorf("chat = %+v", msgs)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.SaveSettings(ctx, "ai_", map[string]string{"model": "qwen3:4b"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSettings(ctx, "ai_", map[string]string{"model": "qwen3:8b", "provider": "ollama"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Settings(ctx, "ai_")
	if err != nil || got["model"] != "qwen3:8b" || got["provider"] != "ollama" || len(got) != 2 {
		t.Errorf("settings = %v, err = %v", got, err)
	}
}
