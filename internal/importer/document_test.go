package importer

import (
	"reflect"
	"strings"
	"testing"
)

func TestDocumentLegacyInvoice(t *testing.T) {
	text := "Fatura do cartão\nVencimento: 10/11/2026\n05/10 IFOOD 52,30\n08/10 PAGAMENTO RECEBIDO 950,00 -"
	want, err := ParseText(text, today)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"auto", "fatura"} {
		got, err := ParseDocumentKind(text, today, kind)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %+v, %v; esperado %+v", kind, got, err, want)
		}
	}
}

func TestDocumentStatement(t *testing.T) {
	text := "Extrato da conta corrente\n05/10/2026 Mercado -52,30\n06/10/2026 Salário +1.000,00\n07/10/2026 Farmácia 25,00 D\n08/10/2026 Reembolso 12,50C\n09/10/2026 Saldo disponível +900,20\n10/10/2026 Limite +100,00\n11/10/2026 Total de entradas +1.012,50\n12/10/2026 Valor total +1.000,00"
	got, err := ParseDocument(text, today)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-10-05|Mercado|-5230", "2026-10-06|Salário|100000", "2026-10-07|Farmácia|-2500", "2026-10-08|Reembolso|1250"}
	if got.Card || got.Unsigned || got.Format != "pdf" || !reflect.DeepEqual(rows(got), want) {
		t.Fatalf("extrato: %+v; linhas %v", got, rows(got))
	}
	got, err = ParseDocumentKind("05/10/2026 Mercado R$ 52,30 -", today, "extrato")
	if err != nil || len(got.Rows) != 1 || got.Rows[0].AmountCents != -5230 {
		t.Fatalf("tipo manual: %+v, %v", got, err)
	}
	got, err = ParseDocument("Extrato da conta corrente\n05/10/2026 Pagamento fatura -50,00", today)
	if err != nil || got.Card || got.Unsigned || len(got.Rows) != 1 || got.Rows[0].AmountCents != -5000 {
		t.Fatalf("extrato não é fatura: %+v, %v", got, err)
	}
}

func TestDocumentReceipt(t *testing.T) {
	for _, test := range []struct {
		name, text, description string
		amount                  int64
	}{
		{"pix enviado", "Comprovante Pix enviado\nData: 05/10/2026 às 10:30\nValor: R$ 1.234,56\nDestinatário: Ana Silva", "Ana Silva", -123456},
		{"transferência recebida", "Transferência recebida\nData da transação: 2026-10-05\nValor total: 25,00\nRemetente: Bruno", "Bruno", 2500},
		{"pagamento", "Recibo de pagamento realizado\nData: 05/10/2026\nTotal pago: R$ 50,00\nEstabelecimento: Padaria", "Padaria", -5000},
		{"rótulos separados", "Comprovante Pix enviado\nData\n05/10/2026\nValor\nR$ 20,00\nPara\nAna Silva", "Ana Silva", -2000},
		{"sem dois pontos", "Comprovante de transferência enviada\nData 05/10/2026\nValor R$ 20,00\nPara Ana Silva", "Ana Silva", -2000},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseDocument(test.text, today)
			if err != nil || len(got.Rows) != 1 {
				t.Fatalf("%+v, %v", got, err)
			}
			want := Row{Date: "2026-10-05", Description: test.description, AmountCents: test.amount}
			if got.Card || got.Unsigned || got.Rows[0] != want {
				t.Fatalf("obtido %+v, esperado %+v", got, want)
			}
		})
	}
}

func TestDocumentRejectUncertain(t *testing.T) {
	receipt := "Comprovante Pix enviado\nData: 05/10/2026\nValor: R$ 50,00\nDestinatário: Ana"
	for _, test := range []struct{ name, text, kind string }{
		{"vazio", "", "auto"},
		{"tipo inválido", receipt, "outro"},
		{"genérico", "Lista de compras\n05/10 Mercado 50,00", "auto"},
		{"extrato sem sinal", "Extrato\n05/10/2026 Mercado 50,00", "auto"},
		{"extrato sem ano", "Extrato\n05/10 Mercado -50,00", "auto"},
		{"extrato OCR", "Extrato\n05/10/2026 Mercado -5O,00", "auto"},
		{"extrato sem contexto", "05/10/2026 Mercado -50,00", "auto"},
		{"extrato parcial", "Extrato\n05/10/2026 Mercado -50,00\n06/10/2026 Loja 30,00", "auto"},
		{"extrato sinal conflitante", "Extrato\n05/10/2026 Mercado -50,00 C", "auto"},
		{"sem data", strings.ReplaceAll(receipt, "Data: 05/10/2026\n", ""), "auto"},
		{"sem ano", strings.ReplaceAll(receipt, "05/10/2026", "05/10"), "auto"},
		{"data inválida", strings.ReplaceAll(receipt, "05/10/2026", "31/02/2026"), "auto"},
		{"sem direção", strings.ReplaceAll(receipt, " enviado", ""), "auto"},
		{"sem destinatário", strings.ReplaceAll(receipt, "Destinatário: Ana", ""), "auto"},
		{"valor OCR", strings.ReplaceAll(receipt, "50,00", "5O,00"), "auto"},
		{"valor truncado", strings.ReplaceAll(receipt, "50,00", "50,001"), "auto"},
		{"valor estourado", strings.ReplaceAll(receipt, "50,00", "92.233.720.368.547.758,08"), "auto"},
		{"valor duplicado", receipt + "\nValor total: 60,00", "auto"},
		{"datas conflitantes", receipt + "\nData: 06/10/2026", "auto"},
		{"direções conflitantes", receipt + "\nPix recebido", "auto"},
		{"manual também valida", strings.ReplaceAll(receipt, " enviado", ""), "comprovante"},
		{"destinatário ilegível", strings.ReplaceAll(receipt, "Destinatário: Ana", "Destinatário:\nStatus: concluído"), "auto"},
		{"destinatário numérico", strings.ReplaceAll(receipt, "Destinatário: Ana", "Destinatário: 123456"), "auto"},
		{"nome não é direção", "Comprovante Pix\nData: 05/10/2026\nValor: R$ 50,00\nDestinatário: Enviado Ltda", "auto"},
		{"nome separado não é direção", "Comprovante Pix\nData: 05/10/2026\nValor: R$ 50,00\nDestinatário\nEnviado Ltda", "auto"},
		{"duas datas no campo", strings.ReplaceAll(receipt, "05/10/2026", "05/10/2026 06/10/2026"), "auto"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseDocumentKind(test.text, today, test.kind)
			if err == nil || len(got.Rows) != 0 {
				t.Fatalf("deveria recusar sem resultado parcial: %+v, %v", got, err)
			}
		})
	}
	for _, status := range []string{"agendado", "pendente", "cancelado", "não efetivado", "não realizado", "em processamento", "não enviado", "não autorizado", "não efetuado"} {
		for _, kind := range []string{"auto", "comprovante"} {
			if got, err := ParseDocumentKind(receipt+"\nStatus: "+status, today, kind); err == nil || len(got.Rows) > 0 {
				t.Errorf("status %s, tipo %s: %+v, %v", status, kind, got, err)
			}
		}
	}
}

func TestDocumentLimits(t *testing.T) {
	for _, test := range []struct{ text, kind string }{
		{strings.Repeat("x", (1<<20)+1), "auto"},
		{"Extrato\n" + strings.Repeat("05/10/2026 Mercado -1,00\n", 10001), "extrato"},
		{"Fatura\n" + strings.Repeat("05/10 Mercado 1,00\n", 10001), "fatura"},
	} {
		if got, err := ParseDocumentKind(test.text, today, test.kind); err == nil || len(got.Rows) != 0 {
			t.Fatalf("limite deveria recusar: %d linhas, %v", len(got.Rows), err)
		}
	}
}

func TestDocumentLongUnrecognizedLine(t *testing.T) {
	text := "Comprovante Pix enviado\n" + strings.Repeat("palavra ", 50000)
	if got, err := ParseDocument(text, today); err == nil || len(got.Rows) != 0 {
		t.Fatalf("texto sem campos deveria falhar: %+v, %v", got, err)
	}
}

func TestDocumentMoneyBounds(t *testing.T) {
	for _, value := range []string{"92233720368547758,07", "92.233.720.368.547.758,07"} {
		got, err := ParseDocumentKind("05/10/2026 Mercado -"+value, today, "extrato")
		if err != nil || got.Rows[0].AmountCents != -9223372036854775807 {
			t.Fatalf("limite int64: %+v, %v", got, err)
		}
	}
	text := "Extrato\n" + strings.Repeat("05/10/2026 Mercado -1,00\n", 10000)
	got, err := ParseDocument(text, today)
	if err != nil || len(got.Rows) != 10000 {
		t.Fatalf("limite de linhas: %d, %v", len(got.Rows), err)
	}
}

func TestDocumentStatementRejectMultipleAmounts(t *testing.T) {
	for _, amounts := range []string{
		"-50,00 +950,00",
		"-50,00 -950,00",
		"50,00 D 950,00 C",
		"50,00 D 950,00 D",
		"-50,00 950,00",
		"-50,00 0,00 C",
	} {
		for _, kind := range []string{"auto", "extrato"} {
			t.Run(kind+"/"+amounts, func(t *testing.T) {
				text := "Extrato\n04/10/2026 Padaria -5,00\n05/10/2026 Mercado " + amounts
				got, err := ParseDocumentKind(text, today, kind)
				if err == nil || len(got.Rows) != 0 {
					t.Fatalf("linha com movimento e saldo deveria falhar sem resultado parcial: %+v, %v", got, err)
				}
				if !strings.Contains(err.Error(), "OFX ou CSV") {
					t.Fatalf("erro precisa orientar a exportação: %v", err)
				}
			})
		}
	}
}

func TestDocumentDueDateIsNotInvoiceEvidence(t *testing.T) {
	text := "Extrato da conta corrente\nVencimento: 10/11/2026\n05/10/2026 Loja -50,00"
	for _, kind := range []string{"auto", "extrato"} {
		got, err := ParseDocumentKind(text, today, kind)
		if err != nil || got.Card || got.Unsigned || len(got.Rows) != 1 || got.Rows[0].AmountCents != -5000 {
			t.Fatalf("vencimento não deve transformar extrato em fatura (%s): %+v, %v", kind, got, err)
		}
	}
	legacy := "Vencimento: 10/11/2026\n05/10 Loja 50,00"
	got, err := ParseDocument(legacy, today)
	if err == nil || len(got.Rows) != 0 || !strings.Contains(err.Error(), "escolha o tipo") {
		t.Fatalf("vencimento sozinho exige escolha do tipo: %+v, %v", got, err)
	}
	want, err := ParseText(legacy, today)
	if err != nil {
		t.Fatal(err)
	}
	got, err = ParseDocumentKind(legacy, today, "fatura")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("fatura manual deve preservar o legado: %+v, %v; esperado %+v", got, err, want)
	}
}

func TestDocumentConflictingKindsRequireManualChoice(t *testing.T) {
	for _, header := range []string{"Fatura\nExtrato", "Extrato\nFatura", "Extrato do cartão de crédito"} {
		text := header + "\nVencimento: 10/11/2026\n05/10/2026 Loja -50,00"
		got, err := ParseDocument(text, today)
		if err == nil || len(got.Rows) != 0 || !strings.Contains(err.Error(), "escolha o tipo") {
			t.Fatalf("conflito deve exigir escolha manual: %+v, %v", got, err)
		}
		got, err = ParseDocumentKind(text, today, "extrato")
		if err != nil || got.Card || got.Unsigned || len(got.Rows) != 1 || got.Rows[0].AmountCents != -5000 {
			t.Fatalf("extrato manual deve manter a saída: %+v, %v", got, err)
		}
	}
}
func TestDocumentMerchantNamesAreNotSummaryLabels(t *testing.T) {
	statement, err := ParseDocument("Extrato\n05/10/2026 Total Express -50,00\n06/10/2026 Mercado -20,00", today)
	if err != nil || len(statement.Rows) != 2 || statement.Rows[0].Description != "Total Express" {
		t.Fatalf("estabelecimento descartado: %+v, %v", statement, err)
	}
}
