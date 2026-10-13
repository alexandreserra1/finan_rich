package importer

import (
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

var today = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)

func pdfRows(t *testing.T, text string) []string {
	t.Helper()
	st, err := ParseText(text, today)
	if err != nil {
		t.Fatal(err)
	}
	if st.Format != "pdf" || !st.Unsigned || !st.Card {
		t.Errorf("fatura em PDF: %+v", st)
	}
	return rows(st)
}

func TestParseTextNubank(t *testing.T) {
	// Como o pdf.js devolve a fatura do Nubank: mês abreviado, sem ano.
	text := `Olá, Maria
Esta é a sua fatura de outubro, no valor de R$ 1.215,67
Data de vencimento: 10/11/2026
Período vigente: 03 OUT a 03 NOV
TRANSAÇÕES DE 03 OUT A 03 NOV
05 OUT Ifood *Ifood 52,30
06 OUT Uber *Trip 23,90
07 OUT Amazon Marketplace - Parcela 2/10 129,00
08 OUT Pagamento em 08 OUT −950,00
12 OUT IOF de compra internacional 1,47
Total a pagar R$ 1.215,67
Pagamento mínimo R$ 182,35`
	got := pdfRows(t, text)
	want := []string{
		"2026-10-05|Ifood *Ifood|5230",
		"2026-10-06|Uber *Trip|2390",
		"2026-10-07|Amazon Marketplace - Parcela 2/10|12900",
		"2026-10-08|Pagamento em 08 OUT|-95000",
		"2026-10-12|IOF de compra internacional|147",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestParseTextTwoColumnsAndYearTurn(t *testing.T) {
	// Fatura que vence em janeiro, com compras de dezembro, em duas colunas.
	text := `Vencimento 15/01/2027   Total desta fatura R$ 3.000,00
DATA ESTABELECIMENTO VALOR EM R$         DATA ESTABELECIMENTO VALOR EM R$
18/12 SUPERMERCADO EXTRA 312,45          02/01 POSTO SHELL 150,00
20/12 NETFLIX.COM 55,90                  05/01 ESTORNO DROGASIL 2,00 C
24/12 LOJA 99 PRESENTES 1.234,56
Lançamentos internacionais
26/12/26 SPOTIFY USD 10,00 58,90
Total dos lançamentos atuais 1.661,81`
	got := pdfRows(t, text)
	want := []string{
		"2026-12-18|SUPERMERCADO EXTRA|31245",
		"2027-01-02|POSTO SHELL|15000",
		"2026-12-20|NETFLIX.COM|5590",
		"2027-01-05|ESTORNO DROGASIL|-200",
		"2026-12-24|LOJA 99 PRESENTES|123456",
		"2026-12-26|SPOTIFY USD 10,00|5890",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestParseTextNoTransactions(t *testing.T) {
	if _, err := ParseText("Contrato de prestação de serviços\nCláusula 1", today); err == nil {
		t.Error("PDF sem compras deveria dar erro")
	}
}

func TestPDFPreviewIsCardBill(t *testing.T) {
	st, _ := ParseText("Vencimento: 10/11/2026\n05/10 IFOOD *RESTAURANTE 52,30\n08/10 PAGAMENTO RECEBIDO 950,00 -\n", today)
	p := Build(domain.Ledger{}, st, Options{})
	if !p.Inverted || !p.CardBill || len(p.Items) != 2 {
		t.Fatalf("prévia: %+v", p)
	}
	byDesc := map[string]Item{}
	for _, it := range p.Items {
		byDesc[it.Description] = it
	}
	if it := byDesc["IFOOD *RESTAURANTE"]; it.Kind != "saida" || it.AmountCents != 5230 {
		t.Errorf("compra: %+v", it)
	}
	if it := byDesc["PAGAMENTO RECEBIDO"]; it.Kind != "entrada" || it.Status != StatusBillPay || it.Selected {
		t.Errorf("pagamento: %+v", it)
	}
}
