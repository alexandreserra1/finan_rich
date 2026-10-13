package report

import (
	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Period é o intervalo de meses de um relatório (inclusive nas pontas).
type Period struct {
	From, To domain.Month
	Label    string // "outubro de 2026", "2026", "todo o histórico"
	Slug     string // para o nome do arquivo: "2026-10", "2026", "completo"
}

// Escopos aceitos em Options.Scope.
const (
	ScopeMonth = "month"
	ScopeYear  = "year"
	ScopeAll   = "all"
)

// NewPeriod monta o período a partir do escopo e do mês de referência.
// "all" vai do primeiro ao último mês com lançamentos.
func NewPeriod(scope string, ref domain.Month, l domain.Ledger) Period {
	switch scope {
	case ScopeYear:
		y := ref.Year()
		return Period{From: domain.NewMonth(y, 1), To: domain.NewMonth(y, 12), Label: ref.Format("2006"), Slug: ref.Format("2006")}
	case ScopeAll:
		first, last := ref, ref
		for i, t := range l.ActiveTransactions() {
			m := domain.ParseMonth(t.Date[:min(7, len(t.Date))])
			if i == 0 || m.Before(first.Time) {
				first = m
			}
			if i == 0 || m.After(last.Time) {
				last = m
			}
		}
		return Period{From: first, To: last, Label: "todo o histórico", Slug: "completo"}
	}
	return Period{From: ref, To: ref, Label: ref.Label(), Slug: ref.Key()}
}

func (p Period) Contains(date string) bool {
	return len(date) >= 7 && date[:7] >= p.From.Key() && date[:7] <= p.To.Key()
}

// Months lista os meses do período, do mais antigo ao mais novo.
func (p Period) Months() []domain.Month {
	var out []domain.Month
	for m := p.From; !m.After(p.To.Time); m = m.Next() {
		out = append(out, m)
		if len(out) > 1200 { // proteção contra datas absurdas
			break
		}
	}
	return out
}

func (p Period) single() bool { return p.From.Key() == p.To.Key() }
