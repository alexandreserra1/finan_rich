package finance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestHomeEstimate(t *testing.T) {
	ledger := insightsLedger()
	if !Home(ledger, day("2026-10-15")).EstimateAvailable {
		t.Fatal("renda registrada sem dívidas ou cartão deve permitir a estimativa de orçamento")
	}
	ledger.Debts = []domain.Debt{{Meta: meta("debt"), BalanceCents: 10000}}
	if got := Home(ledger, day("2026-10-15")); got.EstimateAvailable || !strings.Contains(got.EstimateReason, "dívida") {
		t.Fatalf("dívida pendente: %+v", got)
	}
	ledger.Debts[0].Deleted = true
	if !Home(ledger, day("2026-10-15")).EstimateAvailable {
		t.Fatal("dívida apagada não deve bloquear")
	}
	ledger.Accounts = []domain.Account{{Meta: meta("card"), Name: "Cartão", Kind: domain.AccountCartao, ClosingDay: 3, DueDay: 10}}
	ledger.Transactions = append(ledger.Transactions, domain.Transaction{Meta: meta("purchase"), Kind: domain.KindSaida, CategoryID: "res", AccountID: "card", Date: "2026-10-14", AmountCents: 1000})
	ledger.Accounts = append(ledger.Accounts, domain.Account{Meta: meta("credit"), Name: "Crédito", Kind: domain.AccountCartao, ClosingDay: 3, DueDay: 10, InitialCents: 5000})
	if got := Home(ledger, day("2026-10-15")); got.EstimateAvailable || !strings.Contains(got.EstimateReason, "cartão") {
		t.Fatalf("cartão pendente: %+v", got)
	}
	ledger = insightsLedger()
	for index := range ledger.Transactions {
		if ledger.Transactions[index].Kind == domain.KindEntrada && strings.HasPrefix(ledger.Transactions[index].Date, "2026-10") {
			ledger.Transactions[index].Deleted = true
		}
	}
	if got := Home(ledger, day("2026-10-15")); got.EstimateAvailable || !strings.Contains(got.EstimateReason, "renda") {
		t.Fatalf("média histórica não substitui renda registrada: %+v", got)
	}
	ledger = insightsLedger()
	for index := range ledger.Transactions {
		if ledger.Transactions[index].Kind == domain.KindEntrada && strings.HasPrefix(ledger.Transactions[index].Date, "2026-10") {
			ledger.Transactions[index].Date = "2026-10-30"
		}
	}
	if Home(ledger, day("2026-10-15")).EstimateAvailable {
		t.Fatal("renda futura não deve liberar a estimativa de hoje")
	}
}

func TestHomeWeek(t *testing.T) {
	ledger := domain.Ledger{
		Transactions: []domain.Transaction{
			{Meta: meta("in"), Kind: domain.KindEntrada, AmountCents: 50000, Date: "2026-10-28"},
			{Meta: meta("out"), Kind: domain.KindSaida, AmountCents: 300, Date: "2026-11-03"},
			{Meta: meta("old"), Kind: domain.KindSaida, AmountCents: 999, Date: "2026-10-27"},
			{Meta: meta("future"), Kind: domain.KindSaida, AmountCents: 999, Date: "2026-11-04"},
			{Meta: domain.Meta{ID: "deleted", Deleted: true}, Kind: domain.KindSaida, AmountCents: 999, Date: "2026-11-01"},
		},
		Recurring: []domain.Recurring{
			{Meta: meta("rent"), Kind: domain.KindSaida, Description: "Aluguel", AmountCents: 10000, Day: 5, Start: "2026-10"},
			{Meta: meta("ended"), Kind: domain.KindSaida, AmountCents: 999, Day: 5, Start: "2026-01", End: "2026-10"},
			{Meta: meta("later"), Kind: domain.KindSaida, AmountCents: 999, Day: 5, Start: "2026-12"},
			{Meta: meta("paused"), Kind: domain.KindSaida, AmountCents: 999, Day: 5, Start: "2026-01", Paused: true},
		},
		Debts: []domain.Debt{{Meta: meta("loan"), Name: "Parcela", BalanceCents: 50000, MinPaymentCents: 1000, DueDay: 4}},
	}
	before, _ := json.Marshal(ledger)
	got := Home(ledger, day("2026-11-03"))
	after, _ := json.Marshal(ledger)
	if string(before) != string(after) {
		t.Fatal("consultar o Início não pode alterar os dados")
	}
	if got.WeekInCents != 50000 || got.WeekOutCents != 300 || got.WeekFrom != "2026-10-28" || got.DueThrough != "2026-11-09" {
		t.Fatalf("janela de sete dias: %+v", got)
	}
	if len(got.Due) != 2 || got.Due[0].Date != "2026-11-04" || got.Due[1].Name != "Aluguel" {
		t.Fatalf("compromissos ativos, próximos e ordenados: %+v", got.Due)
	}
}

func TestHomeMonthBoundary(t *testing.T) {
	ledger := domain.Ledger{Recurring: []domain.Recurring{
		{Meta: meta("short"), Kind: domain.KindSaida, Description: "Dia 31", AmountCents: 1000, Day: 31, Start: "2026-01"},
		{Meta: meta("next"), Kind: domain.KindSaida, Description: "Mês seguinte", AmountCents: 2000, Day: 1, Start: "2026-03"},
	}}
	got := Home(ledger, day("2026-02-27"))
	if len(got.Due) != 2 || got.Due[0].Date != "2026-02-28" || got.Due[1].Date != "2026-03-01" {
		t.Fatalf("mês curto e início de recorrência: %+v", got.Due)
	}
}

func TestHomeFutureCommitments(t *testing.T) {
	ledger := insightsLedger()
	ledger.Transactions = append(ledger.Transactions, domain.Transaction{Meta: meta("future-income"), Kind: domain.KindEntrada, Date: "2026-10-30", CategoryID: "sal", AmountCents: 900000})
	if Home(ledger, day("2026-10-15")).EstimateAvailable {
		t.Fatal("renda passada não pode liberar uma estimativa inflada por renda futura")
	}
	ledger = insightsLedger()
	ledger.Accounts = []domain.Account{{Meta: meta("card"), Kind: domain.AccountCartao, ClosingDay: 25, DueDay: 5}}
	ledger.Transactions = append(ledger.Transactions, domain.Transaction{Meta: meta("future-card"), Kind: domain.KindSaida, Date: "2026-10-30", CategoryID: "res", AccountID: "card", AmountCents: 10000})
	if got := Home(ledger, day("2026-10-15")); got.EstimateAvailable || !strings.Contains(got.EstimateReason, "cartão") {
		t.Fatalf("fatura futura pendente também impede estimativa: %+v", got)
	}
}

func TestHomeInvoiceRemaining(t *testing.T) {
	ledger := domain.Ledger{
		Accounts: []domain.Account{
			{Meta: meta("bank"), Kind: domain.AccountCorrente, InitialCents: 50000},
			{Meta: meta("card"), Name: "Cartão", Kind: domain.AccountCartao, ClosingDay: 31, DueDay: 5},
		},
		Transactions: []domain.Transaction{{Meta: meta("purchase"), Kind: domain.KindSaida, AccountID: "card", AmountCents: 10000, Date: "2026-10-29"}},
		Transfers:    []domain.Transfer{{Meta: meta("paid"), FromAccountID: "bank", ToAccountID: "card", AmountCents: 4000, Date: "2026-11-02"}},
	}
	got := Home(ledger, day("2026-11-03"))
	if len(got.Due) != 1 || got.Due[0].Cents != 6000 || got.WeekInCents != 0 || got.WeekOutCents != 10000 {
		t.Fatalf("fatura parcial sem contar pagamento como gasto: %+v", got)
	}
	ledger.Transfers[0].AmountCents = 10000
	if got := Home(ledger, day("2026-11-03")); len(got.Due) != 0 {
		t.Fatalf("fatura paga não deve aparecer de novo: %+v", got.Due)
	}
}

func TestHomeInsights(t *testing.T) {
	ledger := insightsLedger()
	ledger.Debts = []domain.Debt{{Meta: meta("debt"), Name: "Dívida", BalanceCents: 100000, MonthlyRateBP: 1000, MinPaymentCents: 100}}
	got := Home(ledger, day("2026-10-15"))
	if len(got.Insights) == 0 || len(got.Insights) > 3 || !strings.Contains(got.Insights[0].Title, "crescendo") {
		t.Fatalf("aviso grave deve vir primeiro: %+v", got.Insights)
	}
	seen := map[string]bool{}
	for _, insight := range got.Insights {
		key := insight.Title + strings.Join(insight.Lines, " ")
		if seen[key] || insight.Link == "" || insight.Ask == "" {
			t.Fatalf("insight repetido ou sem ação: %+v", insight)
		}
		seen[key] = true
	}
	if empty := Home(domain.Ledger{}, day("2026-10-15")); len(empty.Insights) != 0 || empty.Due == nil || empty.Insights == nil {
		t.Fatalf("vazio sem cartões inventados: %+v", empty)
	}
	ledger.Debts = []domain.Debt{}
	for _, name := range []string{"Primeira", "Segunda", "Terceira", "Quarta"} {
		ledger.Debts = append(ledger.Debts, domain.Debt{Meta: meta(name), Name: name, BalanceCents: 100000, MonthlyRateBP: 1000, MinPaymentCents: 100})
	}
	critical := Home(ledger, day("2026-10-15")).Insights
	if len(critical) != 3 {
		t.Fatalf("três avisos críticos não devem ser escondidos por serem da mesma pessoa do time: %+v", critical)
	}
	for _, insight := range critical {
		if !strings.Contains(insight.Title, "crescendo") {
			t.Fatalf("um resumo comum tomou o lugar de um aviso crítico: %+v", critical)
		}
	}
}
