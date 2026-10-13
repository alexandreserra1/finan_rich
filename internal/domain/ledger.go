package domain

import (
	"encoding/json"
	"sort"
)

// Ledger reúne todos os dados do usuário. É o que o app guarda em cada aparelho
// e o que as regras de cálculo (pacote finance) recebem.
type Ledger struct {
	Categories   []Category    `json:"categories"`
	Transactions []Transaction `json:"transactions"`
	Debts        []Debt        `json:"debts"`
	Prefs        []Pref        `json:"prefs"`
	Notes        []Note        `json:"notes"`
}

// CategoryIndex dá acesso às categorias por ID, inclusive apagadas: um
// lançamento antigo continua mostrando o nome da categoria.
func (l Ledger) CategoryIndex() map[string]Category {
	idx := make(map[string]Category, len(l.Categories))
	for _, c := range l.Categories {
		idx[c.ID] = c
	}
	return idx
}

// ActiveTransactions devolve os lançamentos não apagados, do mais recente ao mais antigo.
func (l Ledger) ActiveTransactions() []Transaction {
	out := make([]Transaction, 0, len(l.Transactions))
	for _, t := range l.Transactions {
		if !t.Deleted {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

// ActiveDebts devolve as dívidas não apagadas, da mais cara (maior juros) para a mais barata.
func (l Ledger) ActiveDebts() []Debt {
	out := make([]Debt, 0, len(l.Debts))
	for _, d := range l.Debts {
		if !d.Deleted {
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].MonthlyRateBP != out[j].MonthlyRateBP {
			return out[i].MonthlyRateBP > out[j].MonthlyRateBP
		}
		return out[i].BalanceCents < out[j].BalanceCents
	})
	return out
}

// ActiveNotes devolve as notas não apagadas: fixadas primeiro, depois da
// alterada mais recentemente para a mais antiga.
func (l Ledger) ActiveNotes() []Note {
	out := make([]Note, 0, len(l.Notes))
	for _, n := range l.Notes {
		if !n.Deleted {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

// Split devolve a divisão do salário salva (ou a 50/30/20).
func (l Ledger) Split() Split {
	for _, p := range l.Prefs {
		if p.ID == PrefSplit && !p.Deleted {
			var s Split
			if json.Unmarshal(p.Value, &s) == nil && s.Validate() == nil {
				return s
			}
		}
	}
	return DefaultSplit()
}
