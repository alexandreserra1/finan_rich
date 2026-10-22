package finance

import (
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/importer"
)

// TxFilter é a busca da tela de lançamentos. Campos vazios (ou zero) não filtram.
type TxFilter struct {
	Text       string `json:"text"`       // parte da descrição, sem diferenciar acento e maiúscula
	CategoryID string `json:"categoryId"` // uma categoria
	Kind       string `json:"kind"`       // entrada ou saida
	MinCents   int64  `json:"minCents"`   // valor a partir de
	MaxCents   int64  `json:"maxCents"`   // valor até
	Month      string `json:"month"`      // "2026-10"; vazio = todos os meses
}

// TxSearch é o resultado: os lançamentos (mais recente primeiro) e os totais
// de tudo que passou no filtro.
type TxSearch struct {
	Transactions []domain.Transaction `json:"transactions"`
	Count        int                  `json:"count"` // total achado (a lista pode vir cortada)
	InCents      int64                `json:"inCents"`
	OutCents     int64                `json:"outCents"`
	BalanceCents int64                `json:"balanceCents"`
}

// maxSearchList limita a lista devolvida; os totais contam tudo.
// shortcut: corta em 500 sem paginar; paginar se alguém precisar ver mais de 500 de uma vez.
const maxSearchList = 500

// SearchTransactions filtra os lançamentos e soma o que foi achado.
func SearchTransactions(l domain.Ledger, f TxFilter) TxSearch {
	text := importer.Normalize(f.Text)
	out := TxSearch{Transactions: []domain.Transaction{}}
	for _, t := range l.ActiveTransactions() {
		if f.Month != "" && !strings.HasPrefix(t.Date, f.Month) ||
			f.CategoryID != "" && t.CategoryID != f.CategoryID ||
			f.Kind != "" && t.Kind != f.Kind ||
			f.MinCents > 0 && t.AmountCents < f.MinCents ||
			f.MaxCents > 0 && t.AmountCents > f.MaxCents ||
			text != "" && !strings.Contains(importer.Normalize(t.Description), text) {
			continue
		}
		out.Count++
		if t.Kind == domain.KindEntrada {
			out.InCents += t.AmountCents
		} else {
			out.OutCents += t.AmountCents
		}
		if len(out.Transactions) < maxSearchList {
			out.Transactions = append(out.Transactions, t)
		}
	}
	out.BalanceCents = out.InCents - out.OutCents
	return out
}
