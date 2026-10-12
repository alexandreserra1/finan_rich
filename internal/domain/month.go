package domain

import "time"

// Month identifica um mês de referência (dia 1, UTC).
type Month struct{ time.Time }

func NewMonth(year int, m time.Month) Month {
	return Month{time.Date(year, m, 1, 0, 0, 0, 0, time.UTC)}
}

func CurrentMonth() Month {
	now := time.Now()
	return NewMonth(now.Year(), now.Month())
}

// ParseMonth aceita "2026-10"; se inválido, devolve o mês atual.
func ParseMonth(s string) Month {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return CurrentMonth()
	}
	return NewMonth(t.Year(), t.Month())
}

func (m Month) Key() string    { return m.Format("2006-01") }
func (m Month) Next() Month    { return Month{m.AddDate(0, 1, 0)} }
func (m Month) Prev() Month    { return Month{m.AddDate(0, -1, 0)} }
func (m Month) End() time.Time { return m.AddDate(0, 1, 0) }

// Contains diz se a data (AAAA-MM-DD) cai neste mês.
func (m Month) Contains(date string) bool { return len(date) >= 7 && date[:7] == m.Key() }

// Day devolve o dia informado dentro do mês, limitado ao último dia (31 → 30 em abril).
func (m Month) Day(day int) time.Time {
	last := m.End().AddDate(0, 0, -1).Day()
	return m.AddDate(0, 0, min(day, last)-1)
}

var mesesPT = [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho",
	"julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

// Label devolve "outubro de 2026".
func (m Month) Label() string { return mesesPT[m.Month()-1] + " de " + m.Format("2006") }

// Abbr devolve "out".
func (m Month) Abbr() string { return mesesPT[m.Month()-1][:3] }

// Short devolve "out/26".
func (m Month) Short() string { return m.Abbr() + "/" + m.Format("06") }
