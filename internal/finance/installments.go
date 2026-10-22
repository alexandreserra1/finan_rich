package finance

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// MaxInstallments é o maior número de parcelas aceito.
const MaxInstallments = 72

// Installments divide uma compra em n lançamentos, um por mês a partir da data
// da compra (dia 31 vira o último dia em meses curtos). O total é dividido em
// centavos sem perder nenhum: a sobra vai na primeira parcela. O ID de cada
// parcela vem do ID da compra ("parc-<id>-<n>"), então salvar de novo, em
// qualquer aparelho, não duplica. No cartão, cada parcela cai na fatura do
// mês dela. Os lançamentos voltam sem UpdatedAt: quem grava preenche.
func Installments(t domain.Transaction, n int) ([]domain.Transaction, error) {
	if n < 2 || n > MaxInstallments {
		return nil, fmt.Errorf("parcelas devem ser de 2 a %d", MaxInstallments)
	}
	if t.AmountCents < int64(n) {
		return nil, errors.New("valor menor que o número de parcelas")
	}
	if t.ID == "" || len(t.ID) > 40 {
		return nil, errors.New("id da compra inválido")
	}
	day, err := time.Parse(domain.DateLayout, t.Date)
	if err != nil {
		return nil, errors.New("data inválida")
	}
	first := domain.NewMonth(day.Year(), day.Month())
	each := t.AmountCents / int64(n)
	out := make([]domain.Transaction, n)
	for i := range n {
		p := t
		p.Meta = domain.Meta{ID: "parc-" + t.ID + "-" + strconv.Itoa(i+1)}
		p.Description = fmt.Sprintf("%s (%d/%d)", t.Description, i+1, n)
		p.AmountCents = each
		if i == 0 {
			p.AmountCents += t.AmountCents % int64(n)
		}
		p.Date = domain.Month{Time: first.AddDate(0, i, 0)}.Day(day.Day()).Format(domain.DateLayout)
		out[i] = p
	}
	return out, nil
}
