package finance

import (
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func day(s string) time.Time {
	d, _ := time.Parse(domain.DateLayout, s)
	return d
}

func TestInvoiceByClosingDay(t *testing.T) {
	card := domain.Account{Kind: domain.AccountCartao, ClosingDay: 3, DueDay: 10}
	late := domain.Account{Kind: domain.AccountCartao, ClosingDay: 25, DueDay: 5}
	end := domain.Account{Kind: domain.AccountCartao, ClosingDay: 31, DueDay: 8}
	for _, c := range []struct {
		card       domain.Account
		date, want string
	}{
		{card, "2026-10-02", "2026-10"},
		{card, "2026-10-03", "2026-10"}, // no dia do fechamento ainda entra
		{card, "2026-10-04", "2026-11"}, // depois do fechamento: mês seguinte
		{card, "2026-12-15", "2027-01"},
		{late, "2026-10-26", "2026-11"},
		{end, "2027-02-28", "2027-02"}, // fechamento 31 vira 28 em fevereiro
		{end, "2027-03-31", "2027-03"},
	} {
		if got := InvoiceMonth(c.card, day(c.date)).Key(); got != c.want {
			t.Errorf("fechamento %d, compra %s: fatura %s, quer %s", c.card.ClosingDay, c.date, got, c.want)
		}
	}
	// Vencimento depois do fechamento: mesmo mês; antes: mês seguinte.
	if d := invoiceDue(card, domain.ParseMonth("2026-10")).Format(domain.DateLayout); d != "2026-10-10" {
		t.Errorf("vencimento = %s", d)
	}
	if d := invoiceDue(late, domain.ParseMonth("2026-11")).Format(domain.DateLayout); d != "2026-12-05" {
		t.Errorf("vencimento = %s", d)
	}
}

func accountsLedger() domain.Ledger {
	tx := func(id, kind, acc, date string, cents int64) domain.Transaction {
		return domain.Transaction{Meta: meta(id), Kind: kind, Description: id, AmountCents: cents, CategoryID: "c", Date: date, AccountID: acc}
	}
	gone := tx("apagado", domain.KindSaida, "bank", "2026-10-01", 1)
	gone.Deleted = true
	return domain.Ledger{
		Accounts: []domain.Account{
			{Meta: meta("card"), Name: "Cartão", Kind: domain.AccountCartao, ClosingDay: 3, DueDay: 10},
			{Meta: meta("bank"), Name: "Banco", Kind: domain.AccountCorrente, InitialCents: 100000},
			{Meta: domain.Meta{ID: "old", UpdatedAt: 1, Deleted: true}, Name: "Antiga", Kind: domain.AccountCarteira, InitialCents: 999},
		},
		Transactions: []domain.Transaction{
			tx("salario", domain.KindEntrada, "bank", "2026-10-01", 50000),
			tx("aluguel", domain.KindSaida, "bank", "2026-09-10", 20000),
			tx("futuro", domain.KindSaida, "bank", "2026-11-01", 7000), // ainda não aconteceu
			tx("sem-conta", domain.KindSaida, "", "2026-10-01", 3000),
			tx("mercado", domain.KindSaida, "card", "2026-10-02", 10000),  // fatura de outubro (fecha dia 3)
			tx("farmacia", domain.KindSaida, "card", "2026-10-04", 4000),  // fatura de novembro
			tx("estorno", domain.KindEntrada, "card", "2026-10-04", 1000), // crédito na de novembro
			tx("setembro", domain.KindSaida, "card", "2026-09-01", 2500),  // fatura de setembro, já vencida
			tx("parcela", domain.KindSaida, "card", "2026-11-04", 3000),   // fatura de dezembro (futura)
			gone,
		},
	}
}

func TestAccountsBalancesAndInvoices(t *testing.T) {
	l := accountsLedger()
	v := Accounts(l, day("2026-10-05"))
	if len(v.Accounts) != 2 || v.Accounts[0].ID != "bank" || v.Accounts[1].ID != "card" {
		t.Fatalf("contas = %+v", v.Accounts)
	}
	bank, card := v.Accounts[0], v.Accounts[1]
	// 1.000 inicial + 500 de entrada − 200 de saída (a de novembro ainda não conta).
	if bank.BalanceCents != 130000 || bank.Current != nil {
		t.Errorf("banco = %+v", bank)
	}
	// Cartão até hoje: 100 + 40 − 10 + 25 = 155 em aberto.
	if card.BalanceCents != -15500 || v.CardsOpenCents != 15500 || v.AccountsCents != 130000 {
		t.Errorf("cartão = %d, totais %+v", card.BalanceCents, v)
	}
	// Dia 5: a fatura de outubro fechou dia 3 e vence dia 10: ainda é a atual.
	cur, next := card.Current, card.Next
	if cur.Month != "2026-10" || !cur.Closed || cur.TotalCents != 10000 || cur.DueDate != "2026-10-10" || cur.ClosingDate != "2026-10-03" || len(cur.Items) != 1 {
		t.Errorf("fatura atual = %+v", cur)
	}
	if next.Month != "2026-11" || next.Closed || next.TotalCents != 3000 || next.DueDate != "2026-11-10" || len(next.Items) != 2 {
		t.Errorf("próxima fatura = %+v", next)
	}
	// Depois do vencimento, a atual passa a ser a de novembro.
	v = Accounts(l, time.Date(2026, 10, 11, 15, 0, 0, 0, time.UTC))
	if c := v.Accounts[1]; c.Current.Month != "2026-11" || c.Current.Closed || c.Next.Month != "2026-12" || c.Next.TotalCents != 3000 {
		t.Errorf("depois do vencimento: atual %+v, próxima %+v", c.Current, c.Next)
	}
}

func TestInstallments(t *testing.T) {
	base := domain.Transaction{Meta: domain.Meta{ID: "abc", UpdatedAt: 9}, Kind: domain.KindSaida, Description: "Geladeira",
		AmountCents: 100000, CategoryID: "c", Date: "2027-01-31", AccountID: "card"}
	got, err := Installments(base, 3)
	if err != nil || len(got) != 3 {
		t.Fatalf("parcelas = %+v, %v", got, err)
	}
	var sum int64
	var desc, dates, ids []string
	for _, p := range got {
		sum += p.AmountCents
		desc, dates, ids = append(desc, p.Description), append(dates, p.Date), append(ids, p.ID)
		if p.UpdatedAt != 0 || p.AccountID != "card" || p.CategoryID != "c" {
			t.Errorf("parcela = %+v", p)
		}
	}
	if sum != 100000 || got[0].AmountCents != 33334 || got[1].AmountCents != 33333 {
		t.Errorf("valores = %d %d %d (soma %d)", got[0].AmountCents, got[1].AmountCents, got[2].AmountCents, sum)
	}
	if strings.Join(desc, "|") != "Geladeira (1/3)|Geladeira (2/3)|Geladeira (3/3)" ||
		strings.Join(dates, "|") != "2027-01-31|2027-02-28|2027-03-31" ||
		strings.Join(ids, "|") != "parc-abc-1|parc-abc-2|parc-abc-3" {
		t.Errorf("parcelas: %v %v %v", desc, dates, ids)
	}
	// Somam exato para vários totais e números de parcelas.
	for _, total := range []int64{10, 99, 100001, 123456789} {
		for n := 2; n <= 12; n++ {
			b := base
			b.AmountCents = total
			ps, err := Installments(b, n)
			if total < int64(n) {
				if err == nil {
					t.Errorf("%d em %dx deveria dar erro", total, n)
				}
				continue
			}
			var s int64
			for _, p := range ps {
				s += p.AmountCents
			}
			if err != nil || s != total || ps[0].AmountCents < ps[n-1].AmountCents {
				t.Errorf("%d em %dx: soma %d, %v", total, n, s, err)
			}
		}
	}
	// No cartão (fecha dia 25), cada parcela cai na fatura do mês dela.
	card := domain.Account{Kind: domain.AccountCartao, ClosingDay: 25, DueDay: 5}
	b := base
	b.Date = "2026-10-28"
	ps, _ := Installments(b, 3)
	var months []string
	for _, p := range ps {
		months = append(months, InvoiceMonth(card, p.Day()).Key())
	}
	if strings.Join(months, ",") != "2026-11,2026-12,2027-01" {
		t.Errorf("faturas das parcelas = %v", months)
	}
	for name, n := range map[string]int{"1 parcela": 1, "muitas": MaxInstallments + 1} {
		if _, err := Installments(base, n); err == nil {
			t.Errorf("%s: deveria dar erro", name)
		}
	}
	b.Date = "ontem"
	if _, err := Installments(b, 2); err == nil {
		t.Error("data inválida deveria dar erro")
	}
}

func TestNetWorth(t *testing.T) {
	l := accountsLedger()
	l.Debts = []domain.Debt{{Meta: meta("d"), Name: "Empréstimo", BalanceCents: 50000}}
	got := NetWorth(l, day("2026-10-05"))
	if len(got) != 12 || got[0].Month != "2025-11" || got[11].Month != "2026-10" || got[11].Label != "out/26" {
		t.Fatalf("meses = %+v", got)
	}
	// Agosto: só o saldo inicial do banco, e a dívida.
	if p := got[9]; p.Month != "2026-08" || p.AccountsCents != 100000 || p.CardsCents != 0 || p.NetCents != 50000 {
		t.Errorf("agosto = %+v", p)
	}
	// Setembro: banco 1.000 − 200; cartão 25 em aberto; dívida 500.
	if p := got[10]; p.AccountsCents != 80000 || p.CardsCents != 2500 || p.DebtsCents != 50000 || p.NetCents != 80000-2500-50000 {
		t.Errorf("setembro = %+v", p)
	}
	// Outubro, até hoje (a compra de novembro ainda não conta): banco 1.300, cartão 155.
	if p := got[11]; p.AccountsCents != 130000 || p.CardsCents != 15500 || p.NetCents != 130000-15500-50000 {
		t.Errorf("outubro = %+v", p)
	}
}
