package finance

import (
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// InvoiceMonth é o mês da fatura (o mês em que ela fecha) de uma compra no
// cartão: até o dia de fechamento entra na fatura do mês; depois, na do mês
// seguinte. Fechamento 31 vira o último dia em meses curtos.
func InvoiceMonth(card domain.Account, date time.Time) domain.Month {
	m := domain.NewMonth(date.Year(), date.Month())
	if date.Day() > m.Day(card.ClosingDay).Day() {
		return m.Next()
	}
	return m
}

// invoiceDue é o vencimento da fatura que fecha no mês m: no mesmo mês se o
// dia de vencimento vem depois do fechamento; senão, no mês seguinte.
func invoiceDue(card domain.Account, m domain.Month) time.Time {
	if card.DueDay > card.ClosingDay {
		return m.Day(card.DueDay)
	}
	return m.Next().Day(card.DueDay)
}

// InvoiceView é uma fatura do cartão.
type InvoiceView struct {
	Month       string               `json:"month"` // mês em que fecha: "2026-10"
	Label       string               `json:"label"` // "outubro de 2026"
	ClosingDate string               `json:"closingDate"`
	DueDate     string               `json:"dueDate"`
	TotalCents  int64                `json:"totalCents"` // compras − estornos e créditos
	Closed      bool                 `json:"closed"`     // já fechou (só falta pagar)
	Items       []domain.Transaction `json:"items"`      // do mais recente ao mais antigo
}

// AccountView é uma conta com o saldo e, no cartão, as faturas.
type AccountView struct {
	domain.Account
	BalanceCents int64        `json:"balanceCents"` // até hoje; no cartão, negativo = em aberto
	Current      *InvoiceView `json:"current,omitempty"`
	Next         *InvoiceView `json:"next,omitempty"`
}

// AccountsView é a tela de contas.
type AccountsView struct {
	Accounts       []AccountView `json:"accounts"`
	AccountsCents  int64         `json:"accountsCents"`  // soma das contas (sem cartões)
	CardsOpenCents int64         `json:"cardsOpenCents"` // em aberto nos cartões
}

// Accounts calcula o saldo de cada conta até hoje e, nos cartões, a fatura
// atual (a primeira que ainda não venceu, aberta ou fechada) e a próxima.
func Accounts(l domain.Ledger, today time.Time) AccountsView {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	v := AccountsView{Accounts: []AccountView{}}
	txs := l.ActiveTransactions()
	for _, a := range l.ActiveAccounts() {
		av := AccountView{Account: a, BalanceCents: balanceAt(a, txs, today.Format(domain.DateLayout))}
		if a.Kind == domain.AccountCartao {
			open := InvoiceMonth(a, today)
			cur := open.Prev()
			if invoiceDue(a, cur).Before(today) {
				cur = open
			}
			av.Current, av.Next = invoice(a, txs, cur, today), invoice(a, txs, cur.Next(), today)
			v.CardsOpenCents -= av.BalanceCents
		} else {
			v.AccountsCents += av.BalanceCents
		}
		v.Accounts = append(v.Accounts, av)
	}
	return v
}

// signed é o efeito do lançamento no saldo da conta.
func signed(t domain.Transaction) int64 {
	if t.Kind == domain.KindEntrada {
		return t.AmountCents
	}
	return -t.AmountCents
}

// balanceAt é o saldo inicial mais os lançamentos da conta até a data (inclusive).
func balanceAt(a domain.Account, txs []domain.Transaction, date string) int64 {
	total := a.InitialCents
	for _, t := range txs {
		if t.AccountID == a.ID && t.Date <= date {
			total += signed(t)
		}
	}
	return total
}

func invoice(card domain.Account, txs []domain.Transaction, m domain.Month, today time.Time) *InvoiceView {
	closing := m.Day(card.ClosingDay)
	v := &InvoiceView{
		Month: m.Key(), Label: m.Label(), Items: []domain.Transaction{},
		ClosingDate: closing.Format(domain.DateLayout), DueDate: invoiceDue(card, m).Format(domain.DateLayout),
		Closed: today.After(closing),
	}
	for _, t := range txs {
		if t.AccountID == card.ID && InvoiceMonth(card, t.Day()) == m {
			v.TotalCents -= signed(t)
			v.Items = append(v.Items, t)
		}
	}
	return v
}

// NetWorthPoint é o patrimônio no fim de um mês.
type NetWorthPoint struct {
	Month         string `json:"month"` // "2026-10"
	Label         string `json:"label"` // "out/26"
	AccountsCents int64  `json:"accountsCents"`
	CardsCents    int64  `json:"cardsCents"` // em aberto nos cartões
	DebtsCents    int64  `json:"debtsCents"`
	NetCents      int64  `json:"netCents"` // contas − cartões − dívidas
}

// NetWorth é o patrimônio mês a mês nos últimos 12 meses (o atual até hoje):
// saldos das contas, menos o que está em aberto nos cartões, menos as dívidas.
func NetWorth(l domain.Ledger, today time.Time) []NetWorthPoint {
	txs := l.ActiveTransactions()
	var debts int64
	// shortcut: a dívida só guarda o saldo atual, então ele vale para os 12 meses;
	// guardar o histórico do saldo se a pessoa pedir a curva das dívidas.
	for _, d := range l.ActiveDebts() {
		debts += d.BalanceCents
	}
	accounts := l.ActiveAccounts()
	current := domain.NewMonth(today.Year(), today.Month())
	out := make([]NetWorthPoint, 0, 12)
	for i := 11; i >= 0; i-- {
		m := domain.Month{Time: current.AddDate(0, -i, 0)}
		end := m.End().AddDate(0, 0, -1)
		if i == 0 {
			end = today
		}
		p := NetWorthPoint{Month: m.Key(), Label: m.Short(), DebtsCents: debts}
		for _, a := range accounts {
			b := balanceAt(a, txs, end.Format(domain.DateLayout))
			if a.Kind == domain.AccountCartao {
				p.CardsCents -= b
			} else {
				p.AccountsCents += b
			}
		}
		p.NetCents = p.AccountsCents - p.CardsCents - p.DebtsCents
		out = append(out, p)
	}
	return out
}
