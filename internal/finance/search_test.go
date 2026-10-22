package finance

import (
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestSearchTransactions(t *testing.T) {
	tx := func(id, kind, desc string, cents int64, cat, date string) domain.Transaction {
		return domain.Transaction{Meta: meta(id), Kind: kind, Description: desc, AmountCents: cents, CategoryID: cat, Date: date}
	}
	l := domain.Ledger{Transactions: []domain.Transaction{
		tx("a", domain.KindSaida, "Pão de Açúcar", 12000, "mer", "2026-10-05"),
		tx("b", domain.KindSaida, "PAO DE ACUCAR delivery", 3000, "res", "2026-10-07"),
		tx("c", domain.KindSaida, "Pão de Açúcar", 25000, "mer", "2026-09-10"),
		tx("d", domain.KindEntrada, "Salário", 500000, "sal", "2026-10-01"),
		tx("e", domain.KindEntrada, "Reembolso Pão de Açúcar", 1000, "sal", "2026-10-08"),
		{Meta: domain.Meta{ID: "x", UpdatedAt: 1, Deleted: true}, Kind: domain.KindSaida, Description: "Pão de Açúcar", AmountCents: 999, CategoryID: "mer", Date: "2026-10-09"},
	}}
	ids := func(s TxSearch) string {
		var out []string
		for _, t := range s.Transactions {
			out = append(out, t.ID)
		}
		return strings.Join(out, ",")
	}
	cases := []struct {
		name    string
		f       TxFilter
		ids     string
		in, out int64
	}{
		{"sem filtro, no mês", TxFilter{Month: "2026-10"}, "e,b,a,d", 501000, 15000},
		{"texto sem acento nem maiúscula", TxFilter{Text: "acúcar", Month: "2026-10"}, "e,b,a", 1000, 15000},
		{"todos os meses", TxFilter{Text: "pao de acucar"}, "e,b,a,c", 1000, 40000},
		{"categoria", TxFilter{CategoryID: "mer"}, "a,c", 0, 37000},
		{"tipo", TxFilter{Kind: domain.KindEntrada}, "e,d", 501000, 0},
		{"faixa de valor", TxFilter{MinCents: 3000, MaxCents: 12000}, "b,a", 0, 15000},
		{"nada achado", TxFilter{Text: "uber"}, "", 0, 0},
	}
	for _, c := range cases {
		got := SearchTransactions(l, c.f)
		if ids(got) != c.ids || got.Count != len(got.Transactions) || got.InCents != c.in || got.OutCents != c.out || got.BalanceCents != c.in-c.out {
			t.Errorf("%s: %s %+v", c.name, ids(got), got)
		}
	}
}
