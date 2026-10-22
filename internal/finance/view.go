package finance

import "github.com/alexandreserra1/finan_rich/internal/domain"

// As views são o que cada tela mostra, já calculado e pronto para virar JSON.
// O navegador recebe estas mesmas structs pelo WebAssembly (cmd/finanwasm),
// então a regra de cálculo existe num lugar só.

type BucketView struct {
	Bucket         string `json:"bucket"`
	Pct            int    `json:"pct"`
	PlannedCents   int64  `json:"plannedCents"`
	SpentCents     int64  `json:"spentCents"`
	RemainingCents int64  `json:"remainingCents"`
	UsedPct        int    `json:"usedPct"`
}

type CategoryView struct {
	Name       string `json:"name"`
	Bucket     string `json:"bucket"`
	SpentCents int64  `json:"spentCents"`
	SharePct   int    `json:"sharePct"`
}

// DashboardView é o painel de um mês.
type DashboardView struct {
	Month        string         `json:"month"` // "2026-10"
	Label        string         `json:"label"` // "outubro de 2026"
	InCents      int64          `json:"inCents"`
	OutCents     int64          `json:"outCents"`
	BalanceCents int64          `json:"balanceCents"`
	SavingsPct   int            `json:"savingsPct"`
	Buckets      []BucketView   `json:"buckets"`
	Categories   []CategoryView `json:"categories"`
	Year         []Totals       `json:"year"`
	Budgets      []BudgetView   `json:"budgets"` // orçamentos por categoria
}

// Dashboard monta o painel do mês.
func Dashboard(l domain.Ledger, m domain.Month) DashboardView {
	s := Summarize(m, l)
	v := DashboardView{
		Month: m.Key(), Label: m.Label(),
		InCents: s.InCents, OutCents: s.OutCents, BalanceCents: s.BalanceCents(), SavingsPct: s.SavingsRatePct(),
		Year:       YearTotals(l, m.Year()),
		Buckets:    []BucketView{},
		Categories: []CategoryView{},
		Budgets:    Budgets(l, m),
	}
	for _, b := range s.Buckets {
		v.Buckets = append(v.Buckets, BucketView{b.Bucket, b.Pct, b.PlannedCents, b.SpentCents, b.RemainingCents(), b.UsedPct()})
	}
	for _, c := range s.Categories {
		v.Categories = append(v.Categories, CategoryView{c.Name, c.Bucket, c.SpentCents, c.SharePct})
	}
	return v
}

type DebtRow struct {
	domain.Debt
	MonthlyInterestCents int64 `json:"monthlyInterestCents"`
	Months               int   `json:"months"`
	InterestCents        int64 `json:"interestCents"`
	Payable              bool  `json:"payable"`
}

// DebtsView é a tela de dívidas.
type DebtsView struct {
	Debts                []DebtRow `json:"debts"`
	TotalCents           int64     `json:"totalCents"`
	TotalMinCents        int64     `json:"totalMinCents"`
	MonthlyInterestCents int64     `json:"monthlyInterestCents"`
	Avalanche            []string  `json:"avalanche"` // nomes, maior juros primeiro
	Snowball             []string  `json:"snowball"`  // nomes, menor saldo primeiro
}

// Debts monta a tela de dívidas.
func Debts(l domain.Ledger) DebtsView {
	p := PlanDebts(l.ActiveDebts())
	v := DebtsView{
		Debts: []DebtRow{}, TotalCents: p.TotalCents, TotalMinCents: p.TotalMinCents, MonthlyInterestCents: p.MonthlyInterest,
		Avalanche: names(p.Avalanche), Snowball: names(p.Snowball),
	}
	for _, d := range p.Debts {
		v.Debts = append(v.Debts, DebtRow{d.Debt, d.MonthlyInterestCents, d.Months, d.InterestCents, d.Payable})
	}
	return v
}

func names(ds []domain.Debt) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Name
	}
	return out
}
