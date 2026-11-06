package finance

import (
	"sort"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

type HomeDue struct {
	Name  string `json:"name"`
	Date  string `json:"date"`
	Cents int64  `json:"cents"`
	Link  string `json:"link"`
}

type HomeView struct {
	HasAccounts       bool       `json:"hasAccounts"`
	AccountsCents     int64      `json:"accountsCents"`
	EstimateAvailable bool       `json:"estimateAvailable"`
	EstimateReason    string     `json:"estimateReason"`
	FreeCents         int64      `json:"freeCents"`
	PerDayCents       int64      `json:"perDayCents"`
	IncomeEstimated   bool       `json:"incomeEstimated"`
	WeekFrom          string     `json:"weekFrom"`
	WeekThrough       string     `json:"weekThrough"`
	DueThrough        string     `json:"dueThrough"`
	WeekInCents       int64      `json:"weekInCents"`
	WeekOutCents      int64      `json:"weekOutCents"`
	Due               []HomeDue  `json:"due"`
	Insights          []Briefing `json:"insights"`
}

func Home(ledger domain.Ledger, today time.Time) HomeView {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	view := HomeView{
		Due: []HomeDue{}, Insights: []Briefing{},
		WeekFrom:    today.AddDate(0, 0, -6).Format(domain.DateLayout),
		WeekThrough: today.Format(domain.DateLayout),
		DueThrough:  today.AddDate(0, 0, 6).Format(domain.DateLayout),
	}
	estimate := Insights(ledger, today)
	view.FreeCents, view.PerDayCents, view.IncomeEstimated = estimate.FreeCents, estimate.PerDayCents, estimate.IncomeEstimated
	view.EstimateAvailable = estimate.IncomeCents > 0 && !estimate.IncomeEstimated
	hasIncome := false
	hasFutureIncome := false
	for _, transaction := range ledger.ActiveTransactions() {
		if transaction.Kind == domain.KindEntrada && transaction.Date >= view.WeekThrough[:7]+"-01" && transaction.Date <= view.WeekThrough {
			hasIncome = true
		}
		if transaction.Kind == domain.KindEntrada && transaction.Date[:7] == view.WeekThrough[:7] && transaction.Date > view.WeekThrough {
			hasFutureIncome = true
		}
	}
	view.EstimateAvailable = view.EstimateAvailable && hasIncome
	if !view.EstimateAvailable {
		view.EstimateReason = "Falta renda registrada neste mês. Confira as entradas antes de usar uma estimativa."
	}
	if hasFutureIncome {
		view.EstimateAvailable = false
		view.EstimateReason = "Há entradas futuras registradas neste mês. Confirme o recebimento antes de usar uma estimativa."
	}
	accounts := Accounts(ledger, today)
	view.AccountsCents = accounts.AccountsCents
	for _, account := range accounts.Accounts {
		if account.Kind != domain.AccountCartao {
			view.HasAccounts = true
		} else if account.BalanceCents < 0 {
			view.EstimateAvailable = false
			view.EstimateReason = "Há valor em aberto no cartão. Confira as faturas antes de decidir quanto gastar."
		}
	}
	if view.HasAccounts && view.AccountsCents < 0 {
		view.EstimateAvailable = false
		view.EstimateReason = "O saldo das contas cadastradas está negativo. Confira os movimentos e compromissos."
	}
	addDue := func(name string, date time.Time, cents int64, link string) {
		key := date.Format(domain.DateLayout)
		if key >= view.WeekThrough && key <= view.DueThrough && cents > 0 {
			view.Due = append(view.Due, HomeDue{name, key, cents, link})
		}
	}
	for _, debt := range ledger.ActiveDebts() {
		if debt.BalanceCents <= 0 {
			continue
		}
		view.EstimateAvailable = false
		view.EstimateReason = "Há dívida pendente. Confira o plano e as parcelas antes de decidir quanto gastar."
		if debt.DueDay > 0 {
			month := domain.NewMonth(today.Year(), today.Month())
			date := month.Day(debt.DueDay)
			if date.Before(today) {
				date = month.Next().Day(debt.DueDay)
			}
			addDue(debt.Name, date, debt.MinPaymentCents, "/dividas")
		}
	}
	for _, recurring := range ledger.ActiveRecurring() {
		if recurring.Paused || recurring.Kind != domain.KindSaida {
			continue
		}
		month := domain.NewMonth(today.Year(), today.Month())
		for index := 0; index < 2; index++ {
			if month.Key() >= recurring.Start && (recurring.End == "" || month.Key() <= recurring.End) {
				addDue(recurring.Description, month.Day(recurring.Day), recurring.AmountCents, "/lancamentos?aba=recorrentes")
			}
			month = month.Next()
		}
	}
	for _, account := range accounts.Accounts {
		if account.Kind != domain.AccountCartao {
			continue
		}
		for _, invoice := range []*InvoiceView{account.Current, account.Next} {
			if invoice == nil {
				continue
			}
			if invoice.TotalCents > invoice.PaidCents {
				view.EstimateAvailable = false
				view.EstimateReason = "Há valor registrado ou previsto no cartão. Confira as faturas antes de decidir quanto gastar."
			}
			date, err := time.Parse(domain.DateLayout, invoice.DueDate)
			if err == nil {
				addDue("Fatura "+account.Name, date, max(0, invoice.TotalCents-invoice.PaidCents), "/contas")
			}
		}
	}
	sort.SliceStable(view.Due, func(first, second int) bool { return view.Due[first].Date < view.Due[second].Date })
	for _, transaction := range ledger.ActiveTransactions() {
		if transaction.Date < view.WeekFrom || transaction.Date > view.WeekThrough {
			continue
		}
		if transaction.Kind == domain.KindEntrada {
			view.WeekInCents += transaction.AmountCents
		} else if transaction.Kind == domain.KindSaida {
			view.WeekOutCents += transaction.AmountCents
		}
	}
	seen := map[string]bool{}
	addInsight := func(brief Briefing) {
		if len(view.Insights) < 3 {
			view.Insights = append(view.Insights, brief)
			seen[brief.Agent.ID] = true
		}
	}
	alerts := PendingAlerts(ledger, today)
	for _, alert := range alerts {
		if alert.Level != "ok" {
			addInsight(Briefing{Agent: alert.Agent, Title: alert.Title, Lines: []string{alert.Text}, Link: alert.Link, Ask: alert.Ask})
		}
	}
	for _, brief := range Briefings(ledger, today) {
		if !seen[brief.Agent.ID] {
			addInsight(brief)
		}
	}
	return view
}
