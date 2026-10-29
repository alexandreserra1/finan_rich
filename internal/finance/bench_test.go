package finance

import (
	"fmt"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// perfLedger: 5 mil lançamentos em dois anos, para medir as contas do Painel.
// go test ./internal/finance -run XXX -bench Painel  (o WebAssembly é umas 20 a 30 vezes mais lento)
func perfLedger() domain.Ledger {
	l := domain.Ledger{Categories: domain.DefaultCategories(1)}
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5000; i++ {
		d := now.AddDate(0, 0, -(i % 730))
		kind, cat := domain.KindSaida, l.Categories[1+i%10].ID
		if i%25 == 0 {
			kind, cat = domain.KindEntrada, "padrao-salario"
		}
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: domain.Meta{ID: fmt.Sprint(i), UpdatedAt: 1}, Kind: kind, Description: fmt.Sprintf("Compra %d", i%300), AmountCents: int64(1000 + i*37%50000), CategoryID: cat, Date: d.Format("2006-01-02")})
	}
	return l
}

// BenchmarkPainel: tudo o que o Painel calcula numa abertura. Em 29/10/2026: 53 ms → 13 ms ao
// tirar a cópia ordenada dos lançamentos das somas (no navegador, de 1,6 s para 0,47 s).
func BenchmarkPainel(b *testing.B) {
	l := perfLedger()
	today := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	m := domain.NewMonth(2026, 10)
	for b.Loop() {
		Briefings(l, today)
		Alerts(l, today)
		Dashboard(l, m)
	}
}
