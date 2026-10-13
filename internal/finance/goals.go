package finance

import (
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// GoalView é uma meta com o progresso calculado.
type GoalView struct {
	domain.Goal
	ContributedCents int64 `json:"contributedCents"` // saídas lançadas na categoria da meta
	ProgressCents    int64 `json:"progressCents"`    // já guardado + aportes
	RemainingCents   int64 `json:"remainingCents"`
	Pct              int   `json:"pct"`
	MonthsLeft       int   `json:"monthsLeft"`    // meses até o prazo, contando o atual (0 = sem prazo ou vencido)
	PerMonthCents    int64 `json:"perMonthCents"` // quanto guardar por mês para chegar no prazo
	Done             bool  `json:"done"`
	Late             bool  `json:"late"` // prazo passou sem atingir
}

// Goals calcula o progresso de cada meta em relação a hoje.
func Goals(l domain.Ledger, today time.Time) []GoalView {
	current := domain.NewMonth(today.Year(), today.Month())
	out := []GoalView{}
	for _, g := range l.ActiveGoals() {
		v := GoalView{Goal: g}
		if g.CategoryID != "" {
			for _, t := range l.ActiveTransactions() {
				if t.CategoryID == g.CategoryID && t.Kind == domain.KindSaida && t.Date >= g.StartDate {
					v.ContributedCents += t.AmountCents
				}
			}
		}
		v.ProgressCents = g.SavedCents + v.ContributedCents
		v.RemainingCents = max(0, g.TargetCents-v.ProgressCents)
		v.Pct = int(min(100, v.ProgressCents*100/g.TargetCents))
		v.Done = v.RemainingCents == 0
		if g.Deadline != "" && !v.Done {
			deadline := domain.ParseMonth(g.Deadline)
			months := (deadline.Year()-current.Year())*12 + int(deadline.Month()-current.Month()) + 1
			if months <= 0 {
				v.Late = true
			} else {
				v.MonthsLeft = months
				v.PerMonthCents = (v.RemainingCents + int64(months) - 1) / int64(months) // arredonda para cima
			}
		}
		out = append(out, v)
	}
	return out
}
