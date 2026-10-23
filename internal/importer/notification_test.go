package importer

import "testing"

func TestParseNotification(t *testing.T) {
	cases := []struct {
		text  string
		ok    bool
		cents int64
		merch string
	}{
		{"Compra de R$ 45,90 APROVADA em IFOOD *RESTAURANTE para o cartão com final 1234.", true, -4590, "IFOOD *RESTAURANTE"},
		{"Compra aprovada no cartão final 1234 de R$ 1.234,56 em MAGAZINE LUIZA em 23/10 às 12:34.", true, -123456, "MAGAZINE LUIZA"},
		{"Compra aprovada: R$ 23,50 em UBER *TRIP", true, -2350, "UBER *TRIP"},
		{"Você enviou um Pix de R$ 50,00 para JOAO DA SILVA.", true, -5000, "Pix para JOAO DA SILVA"},
		{"Você recebeu um Pix de R$ 150,00 de MARIA SOUZA", true, 15000, "Pix de MARIA SOUZA"},
		{"Transferência recebida: R$ 2.000,00 de EMPRESA LTDA", true, 200000, "Transferência de EMPRESA LTDA"},
		{"Pagamento de R$ 89,90 realizado: CONTA DE LUZ", true, -8990, "CONTA DE LUZ"},
		// Não é lançamento: propaganda, saldo, código, fatura fechada.
		{"Aproveite! Cashback de até R$ 50,00 no seu próximo pedido.", false, 0, ""},
		{"Seu saldo é R$ 1.000,00", false, 0, ""},
		{"Seu código de verificação é 123456", false, 0, ""},
		{"Sua fatura de R$ 800,00 fechou. Vencimento dia 20.", false, 0, ""},
		{"Compra negada de R$ 99,00 em LOJA X: saldo insuficiente", false, 0, ""},
	}
	for _, c := range cases {
		r, ok := ParseNotification(c.text)
		if ok != c.ok || (ok && (r.AmountCents != c.cents || r.Description != c.merch)) {
			t.Errorf("%q = %+v, %v; want %d %q %v", c.text, r, ok, c.cents, c.merch, c.ok)
		}
	}
}
