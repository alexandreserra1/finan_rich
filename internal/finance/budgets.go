package finance

import (
	"sort"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// BudgetView é o orçamento de uma categoria no mês: o teto e quanto já foi usado.
type BudgetView struct {
	CategoryID     string `json:"categoryId"`
	Name           string `json:"name"`
	Bucket         string `json:"bucket"`
	BudgetCents    int64  `json:"budgetCents"`
	SpentCents     int64  `json:"spentCents"`
	RemainingCents int64  `json:"remainingCents"` // negativo = estourou
	UsedPct        int    `json:"usedPct"`        // pode passar de 100
}

// budgetWarnPct é quando o orçamento vira aviso no painel.
const budgetWarnPct = 80

// Budgets devolve os orçamentos do mês (só categorias com teto), do mais
// usado para o menos usado. Conta todas as saídas da categoria no mês.
func Budgets(l domain.Ledger, m domain.Month) []BudgetView {
	spent := map[string]int64{}
	for _, t := range l.Transactions { // soma: a ordem não importa (ActiveTransactions copia e ordena tudo)
		if t.Deleted {
			continue
		}
		if t.Kind == domain.KindSaida && m.Contains(t.Date) {
			spent[t.CategoryID] += t.AmountCents
		}
	}
	out := []BudgetView{}
	for _, c := range l.Categories {
		if c.Deleted || c.BudgetCents <= 0 {
			continue
		}
		s := spent[c.ID]
		out = append(out, BudgetView{CategoryID: c.ID, Name: c.Name, Bucket: c.Bucket, BudgetCents: c.BudgetCents,
			SpentCents: s, RemainingCents: c.BudgetCents - s, UsedPct: int(s * 100 / c.BudgetCents)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UsedPct != out[j].UsedPct {
			return out[i].UsedPct > out[j].UsedPct
		}
		return out[i].Name < out[j].Name
	})
	return out
}
