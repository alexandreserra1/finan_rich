package finance

import (
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestBriefings(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	tx := func(id, cat string, cents int64, date string) domain.Transaction {
		return domain.Transaction{Meta: meta(id), Kind: domain.KindSaida, Description: id, AmountCents: cents, CategoryID: cat, Date: date}
	}
	l := domain.Ledger{
		Categories: []domain.Category{
			{Meta: meta("sal"), Name: "Salário", Bucket: domain.BucketRenda},
			{Meta: meta("res"), Name: "Restaurantes", Bucket: domain.BucketDesejos},
		},
		Transactions: []domain.Transaction{
			{Meta: meta("s"), Kind: domain.KindEntrada, Description: "Salário", AmountCents: 500000, CategoryID: "sal", Date: "2026-10-05"},
			tx("r10", "res", 120000, "2026-10-06"),
			tx("r09", "res", 40000, "2026-09-06"),
			tx("r08", "res", 50000, "2026-08-06"),
			tx("r07", "res", 60000, "2026-07-06"),
		},
		Debts: []domain.Debt{
			{Meta: meta("d1"), Name: "Cartão", BalanceCents: 100000, MinPaymentCents: 20000, MonthlyRateBP: 1000},
			{Meta: meta("d2"), Name: "Empréstimo", BalanceCents: 100000, MinPaymentCents: 20000, MonthlyRateBP: 200},
		},
		Goals: []domain.Goal{
			{Meta: meta("g1"), Name: "Viagem", TargetCents: 600000, SavedCents: 0, Deadline: "2027-03", StartDate: "2026-10-01"},
			{Meta: meta("g2"), Name: "Notebook", TargetCents: 300000, SavedCents: 100000, Deadline: "2026-08", StartDate: "2026-01-01"},
		},
	}
	got := map[string]string{}
	var order []string
	for _, b := range Briefings(l, today) {
		got[b.Agent.ID] = b.Title + "\n" + strings.Join(b.Lines, "\n")
		order = append(order, b.Agent.ID)
		if b.Link == "" || len(b.Lines) == 0 || len(b.Lines) > 3 {
			t.Errorf("%s: link %q, %d linhas", b.Agent.Name, b.Link, len(b.Lines))
		}
	}
	if strings.Join(order, ",") != "bento,tito,otavio" {
		t.Errorf("ordem = %v", order)
	}
	for agent, want := range map[string][]string{
		"bento":  {"Cartão primeiro: 10,00% ao mês (R$ 100,00 de juros por mês)", "Só no mínimo, fica livre de todas em 7 meses (maio de 2027)"},
		"tito":   {"Viagem: guarde R$ 1.000,00 por mês até março de 2027", "Notebook: o prazo passou e faltam R$ 2.000,00"},
		"otavio": {"Restaurantes: R$ 1.200,00 este mês, 140% acima da média"},
	} {
		for _, w := range want {
			if !strings.Contains(got[agent], w) {
				t.Errorf("%s sem %q:\n%s", agent, w, got[agent])
			}
		}
	}

	// Sem dívidas, metas nem histórico: ninguém tem o que contar.
	if b := Briefings(domain.Ledger{}, today); b == nil || len(b) != 0 {
		t.Errorf("vazio = %+v", b)
	}
}

func TestBriefingSubscriptionsAreOnlyWants(t *testing.T) {
	l := domain.Ledger{Categories: []domain.Category{
		{Meta: meta("cas"), Name: "Moradia", Bucket: domain.BucketNecessidades},
		{Meta: meta("ass"), Name: "Assinaturas", Bucket: domain.BucketDesejos},
	}}
	for _, mo := range []string{"07", "08", "09", "10"} {
		l.Transactions = append(l.Transactions,
			domain.Transaction{Meta: meta("al" + mo), Kind: domain.KindSaida, Description: "Aluguel", AmountCents: 145000, CategoryID: "cas", Date: "2026-" + mo + "-02"},
			domain.Transaction{Meta: meta("nf" + mo), Kind: domain.KindSaida, Description: "Netflix", AmountCents: 5590, CategoryID: "ass", Date: "2026-" + mo + "-11"})
	}
	var text string
	for _, b := range Briefings(l, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)) {
		text += strings.Join(b.Lines, "\n")
	}
	if !strings.Contains(text, "1 assinatura: R$ 670,80 por ano.") {
		t.Errorf("o aluguel não é assinatura:\n%s", text)
	}
}

func TestBriefingOneDebt(t *testing.T) {
	l := domain.Ledger{Debts: []domain.Debt{{Meta: meta("d1"), Name: "Cartão", BalanceCents: 100000, MinPaymentCents: 20000, MonthlyRateBP: 1000}}}
	b := Briefings(l, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if len(b) != 1 || b[0].Lines[0] != "Cartão: 10,00% ao mês (R$ 100,00 de juros por mês)." ||
		b[0].Lines[1] != "Só no mínimo, fica livre de todas em 8 meses (junho de 2027), com R$ 456,39 de juros." {
		t.Errorf("uma dívida = %+v", b)
	}
}
