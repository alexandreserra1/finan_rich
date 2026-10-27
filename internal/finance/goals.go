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
		v.MonthsLeft, v.PerMonthCents, v.Late = GoalPlan(v.RemainingCents, g.Deadline, today)
		out = append(out, v)
	}
	return out
}

// GoalPlan diz quantos meses faltam até o prazo (contando o atual) e quanto
// guardar por mês, arredondado para cima. Prazo passado com dinheiro faltando
// é late; sem prazo ou sem nada a juntar, tudo zero.
func GoalPlan(remainingCents int64, deadline string, today time.Time) (months int, perMonthCents int64, late bool) {
	if deadline == "" || remainingCents <= 0 {
		return 0, 0, false
	}
	d := domain.ParseMonth(deadline)
	months = (d.Year()-today.Year())*12 + int(d.Month()-today.Month()) + 1
	if months <= 0 {
		return 0, 0, true
	}
	return months, (remainingCents + int64(months) - 1) / int64(months), false
}

// Contribution é guardar dinheiro numa meta (botão "Guardar" e o consultor).
// Com categoria, vira uma saída lançada nela (sem id: quem grava cria); sem
// categoria, soma no que já está guardado.
func Contribution(g domain.Goal, cents int64, date string) (string, any) {
	if g.CategoryID != "" {
		return domain.TypeTransaction, domain.Transaction{Kind: domain.KindSaida, Description: "Aporte: " + g.Name, AmountCents: cents, CategoryID: g.CategoryID, Date: date}
	}
	g.SavedCents += cents
	return domain.TypeGoal, g
}

// GoalsOverview é a tela Metas: as metas e as somas da faixa do topo.
type GoalsOverview struct {
	Goals              []GoalView `json:"goals"`
	SavedCents         int64      `json:"savedCents"`         // guardado em todas
	RemainingCents     int64      `json:"remainingCents"`     // falta juntar
	PerMonthCents      int64      `json:"perMonthCents"`      // guardar por mês, somando as metas com prazo
	InvestPlannedCents int64      `json:"investPlannedCents"` // o que a divisão do salário reserva para Investimentos (0 = sem renda no mês)
	ReserveCents       int64      `json:"reserveCents"`       // sugestão da primeira meta: 3 meses de gastos
}

func GoalsPage(l domain.Ledger, today time.Time) GoalsOverview {
	m := domain.NewMonth(today.Year(), today.Month())
	p := GoalsOverview{Goals: Goals(l, today)}
	for _, g := range p.Goals {
		p.SavedCents += g.ProgressCents
		p.RemainingCents += g.RemainingCents
		p.PerMonthCents += g.PerMonthCents
	}
	for _, b := range Summarize(m, l).Buckets {
		if b.Bucket == domain.BucketInvestimentos {
			p.InvestPlannedCents = b.PlannedCents
		}
	}
	p.ReserveCents = PlanEmergencyFund(RecentTotals(l, m, 6), 0).MinCents
	return p
}
