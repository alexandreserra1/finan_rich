package finance

import (
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func pendingRecurring(l domain.Ledger, month domain.Month) []domain.Recurring {
	exists := map[string]bool{}
	for _, transaction := range l.Transactions {
		exists[transaction.ID] = true
	}
	var pending []domain.Recurring
	for _, recurring := range l.ActiveRecurring() {
		if recurring.Paused || recurring.Start > month.Key() || recurring.End != "" && recurring.End < month.Key() ||
			exists[domain.RecurringTxID(recurring.ID, month)] {
			continue
		}
		pending = append(pending, recurring)
	}
	return pending
}

func PendingRecurringExpenses(l domain.Ledger, month domain.Month) int64 {
	var total int64
	for _, recurring := range pendingRecurring(l, month) {
		if recurring.Kind == domain.KindSaida {
			total += recurring.AmountCents
		}
	}
	return total
}

// RecurringDue lista os lançamentos que as recorrências já deveriam ter
// gerado até hoje e ainda não existem. O ID é fixo por recorrência e mês,
// então dois aparelhos geram o mesmo lançamento (sem duplicar) e um
// lançamento gerado e depois apagado não volta.
// Os lançamentos voltam sem UpdatedAt: quem grava preenche.
func RecurringDue(l domain.Ledger, today time.Time) []domain.Transaction {
	exists := map[string]bool{}
	for _, t := range l.Transactions {
		exists[t.ID] = true
	}
	todayKey := today.Format(domain.DateLayout)
	current := domain.NewMonth(today.Year(), today.Month())
	var out []domain.Transaction
	for _, r := range l.ActiveRecurring() {
		if r.Paused {
			continue
		}
		last := current
		if r.End != "" && r.End < current.Key() {
			last = domain.ParseMonth(r.End)
		}
		for m := domain.ParseMonth(r.Start); !m.After(last.Time); m = m.Next() {
			date := m.Day(r.Day).Format(domain.DateLayout)
			id := domain.RecurringTxID(r.ID, m)
			if date > todayKey || exists[id] {
				continue
			}
			out = append(out, domain.Transaction{
				Meta: domain.Meta{ID: id}, Kind: r.Kind, Description: r.Description,
				AmountCents: r.AmountCents, CategoryID: r.CategoryID, Date: date,
			})
		}
	}
	return out
}
