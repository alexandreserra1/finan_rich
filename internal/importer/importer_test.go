package importer

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func parseFile(t *testing.T, name string) Statement {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	st, err := Parse(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return st
}

// rows resume as linhas como "data|descrição|valor".
func rows(st Statement) []string {
	var out []string
	for _, r := range st.Rows {
		out = append(out, r.Date+"|"+r.Description+"|"+strconv.FormatInt(r.AmountCents, 10))
	}
	return out
}

func TestParseFiles(t *testing.T) {
	cases := map[string]struct {
		format   string
		unsigned bool
		want     []string
	}{
		"itau.ofx": {"ofx", false, []string{
			"2026-10-05|SALARIO EMPRESA X|500000",
			"2026-10-06|ALUGUEL - TED JOAO IMOVEIS|-140000",
			"2026-10-07|COMPRA CARTAO - IFOOD *RESTAURANTE|-8750",
			"2026-10-08|PÃO DE AÇÚCAR 123|-31240", // Windows-1252 convertido
			"2026-10-10|PAGAMENTO FATURA CARTAO|-95000",
		}},
		"nubank_cartao.ofx": {"ofx", false, []string{"2026-10-02|Spotify|-5990", "2026-10-05|Pagamento recebido|95000"}},
		"nubank_conta.csv": {"csv", false, []string{
			"2026-10-01|Transferência recebida pelo Pix - EMPRESA X LTDA - CNPJ 12.345.678/0001-90|500000",
			"2026-10-03|Compra no débito - UBER *TRIP|-4590",
			"2026-10-04|Compra no débito - NETFLIX.COM|-8990",
			"2026-10-05|Pagamento de fatura|-120000",
		}},
		"nubank_cartao.csv": {"csv", true, []string{
			"2026-10-02|Ifood *Ifood|5230", "2026-10-03|Supermercado Boa Compra|23147", "2026-10-04|Drogasil 123|3890",
			"2026-10-05|Amazon Marketplace|12900", "2026-10-06|Pagamento recebido|-95000",
		}},
		"inter.csv":        {"csv", false, []string{"2026-10-02|Pix enviado - Maria Silva|-15000", "2026-10-03|Compra no débito - POSTO SHELL|-20000", "2026-10-04|Pix recebido - João|123456"}},
		"bb.csv":           {"csv", false, []string{"2026-10-05|Pagamento de Boleto - ENEL SP|-31210", "2026-10-06|Pix - Recebido - Cliente ABC|80000"}},
		"finan_export.csv": {"csv", false, []string{"2026-10-05|Salário|500000", "2026-10-07|Cinema \"estreia\"|-8750"}},
	}
	for name, c := range cases {
		st := parseFile(t, name)
		if st.Format != c.format || st.Unsigned != c.unsigned {
			t.Errorf("%s: formato %q, sem sinal %v", name, st.Format, st.Unsigned)
		}
		if got := rows(st); strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%s:\n got %q\nwant %q", name, got, c.want)
		}
	}
	if st := parseFile(t, "itau.ofx"); st.Account != "12345-6" || st.Card {
		t.Errorf("conta OFX = %q cartão=%v", st.Account, st.Card)
	}
	if st := parseFile(t, "nubank_cartao.ofx"); !st.Card {
		t.Error("OFX de cartão não reconhecido")
	}
	if st := parseFile(t, "finan_export.csv"); st.Rows[1].Category != "Lazer" {
		t.Errorf("categoria do CSV exportado = %q", st.Rows[1].Category)
	}
	if _, err := Parse([]byte("isto não é um extrato")); err == nil {
		t.Error("texto qualquer deveria dar erro")
	}
	if _, err := Parse([]byte("a;b;c\n1;2;3\n")); err == nil {
		t.Error("CSV sem colunas reconhecíveis deveria dar erro")
	}
}

func TestParseAmountAndDate(t *testing.T) {
	amounts := map[string]int64{
		"1.234,56": 123456, "-1.234,56": -123456, "1234.56": 123456, "R$ 1.234,56": 123456, "(87,50)": -8750,
		"87,50 D": -8750, "87,50 C": 8750, "3.500": 350000, "1.234.567": 123456700, "-12.3456": -1234, "45": 4500,
		"1,234.56": 123456, "100-": -10000,
	}
	for in, want := range amounts {
		if got, ok := parseAmount(in); !ok || got != want {
			t.Errorf("parseAmount(%q) = %d, %v; quer %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "abc", "R$", "12,3x"} {
		if _, ok := parseAmount(bad); ok {
			t.Errorf("parseAmount(%q) deveria falhar", bad)
		}
	}
	dates := map[string]string{
		"05/10/2026": "2026-10-05", "5/10/2026": "2026-10-05", "05/10/26": "2026-10-05", "2026-10-05": "2026-10-05",
		"20261005": "2026-10-05", "20261005120000[-3:BRT]": "2026-10-05", "05/10/2026 14:32": "2026-10-05", "2026-10-05T14:32:00": "2026-10-05",
	}
	for in, want := range dates {
		if got, ok := parseDate(in); !ok || got != want {
			t.Errorf("parseDate(%q) = %q, %v", in, got, ok)
		}
	}
	if _, ok := parseDate("Saldo"); ok {
		t.Error("parseDate de texto deveria falhar")
	}
}

// ledger com as categorias padrão e um histórico.
func testLedger() domain.Ledger {
	names := []struct{ id, name, bucket string }{
		{"sal", "Salário", "renda"}, {"ext", "Renda extra", "renda"}, {"mor", "Moradia", "necessidades"},
		{"mer", "Mercado", "necessidades"}, {"tra", "Transporte", "necessidades"}, {"sau", "Saúde", "necessidades"},
		{"res", "Restaurantes e delivery", "desejos"}, {"ass", "Assinaturas", "desejos"}, {"com", "Compras", "desejos"},
		{"laz", "Lazer", "desejos"}, {"pet", "Pet", "desejos"},
	}
	var l domain.Ledger
	for _, n := range names {
		l.Categories = append(l.Categories, domain.Category{Meta: domain.Meta{ID: n.id, UpdatedAt: 1}, Name: n.name, Bucket: n.bucket})
	}
	tx := func(id, desc, cat, date string, cents int64) domain.Transaction {
		return domain.Transaction{Meta: domain.Meta{ID: id, UpdatedAt: 1}, Kind: domain.KindSaida, Description: desc, AmountCents: cents, CategoryID: cat, Date: date}
	}
	l.Transactions = []domain.Transaction{
		// A pessoa já classificou o supermercado do bairro como Pet (é onde compra a ração).
		tx("h1", "Compra no débito - SUPERMERCADO BOA COMPRA 01/02", "pet", "2026-09-03", 10000),
		// Digitado à mão: mesmo dia e valor do Uber do extrato.
		tx("m1", "Uber para o trabalho", "tra", "2026-10-03", 4590),
	}
	return l
}

func TestBuildPreview(t *testing.T) {
	l := testLedger()
	byDesc := func(p Preview) map[string]Item {
		out := map[string]Item{}
		for _, it := range p.Items {
			out[it.Description] = it
		}
		return out
	}

	conta := byDesc(Build(l, parseFile(t, "nubank_conta.csv"), Options{}))
	check := func(items map[string]Item, desc, kind, cat, reason, status string, selected bool) {
		t.Helper()
		it, ok := items[desc]
		if !ok {
			t.Fatalf("sem a linha %q", desc)
		}
		if it.Kind != kind || it.CategoryID != cat || it.Reason != reason || it.Status != status || it.Selected != selected {
			t.Errorf("%q = %+v; quer kind=%s cat=%s reason=%s status=%s selected=%v", desc, it, kind, cat, reason, status, selected)
		}
	}
	check(conta, "Compra no débito - NETFLIX.COM", "saida", "ass", ReasonKeyword, StatusNew, true)
	check(conta, "Compra no débito - UBER *TRIP", "saida", "tra", ReasonKeyword, StatusDuplicate, false) // já digitado à mão
	check(conta, "Pagamento de fatura", "saida", "", "", StatusBillPay, false)
	check(conta, "Transferência recebida pelo Pix - EMPRESA X LTDA - CNPJ 12.345.678/0001-90", "entrada", "", "", StatusNew, true)

	cartao := Build(l, parseFile(t, "nubank_cartao.csv"), Options{})
	if !cartao.Inverted || !cartao.CardBill {
		t.Error("fatura em CSV deveria ser invertida (positivo = gasto)")
	}
	items := byDesc(cartao)
	check(items, "Ifood *Ifood", "saida", "res", ReasonKeyword, StatusNew, true)
	check(items, "Supermercado Boa Compra", "saida", "pet", ReasonHistory, StatusNew, true) // o histórico vence a palavra-chave
	check(items, "Drogasil 123", "saida", "sau", ReasonKeyword, StatusNew, true)
	check(items, "Pagamento recebido", "entrada", "", "", StatusBillPay, false)
	if cartao.From != "2026-10-02" || cartao.To != "2026-10-06" || cartao.Items[0].Date != "2026-10-06" {
		t.Errorf("período %s a %s, primeiro %s", cartao.From, cartao.To, cartao.Items[0].Date)
	}

	// Desfazer a inversão automática.
	no := false
	if it := byDesc(Build(l, parseFile(t, "nubank_cartao.csv"), Options{Invert: &no}))["Ifood *Ifood"]; it.Kind != "entrada" {
		t.Error("Invert=false deveria manter o sinal do arquivo")
	}

	// Importar o mesmo arquivo de novo: tudo já importado.
	first := Build(l, parseFile(t, "itau.ofx"), Options{})
	for _, it := range first.Items {
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: domain.Meta{ID: it.ID, UpdatedAt: 2}, Kind: it.Kind,
			Description: it.Description, AmountCents: it.AmountCents, CategoryID: "mer", Date: it.Date})
	}
	for _, it := range Build(l, parseFile(t, "itau.ofx"), Options{}).Items {
		if it.Status != StatusImported || it.Selected {
			t.Errorf("reimportar %q: %s", it.Description, it.Status)
		}
	}

	// Categoria vinda do CSV exportado pelo próprio finan_rich.
	if it := byDesc(Build(testLedger(), parseFile(t, "finan_export.csv"), Options{}))[`Cinema "estreia"`]; it.CategoryID != "laz" || it.Reason != ReasonFile {
		t.Errorf("categoria do arquivo: %+v", it)
	}
}

func TestImportIDsAreStableAndUnique(t *testing.T) {
	st := Statement{Format: "csv", Rows: []Row{
		{Date: "2026-10-01", Description: "Café", AmountCents: -500},
		{Date: "2026-10-01", Description: "Café", AmountCents: -500}, // duas compras iguais no mesmo dia
	}}
	a := Build(domain.Ledger{}, st, Options{})
	b := Build(domain.Ledger{}, st, Options{})
	if a.Items[0].ID == a.Items[1].ID {
		t.Error("compras iguais no mesmo dia precisam de IDs diferentes")
	}
	if a.Items[0].ID != b.Items[0].ID || !strings.HasPrefix(a.Items[0].ID, "imp-") {
		t.Error("o ID precisa ser o mesmo a cada importação")
	}
}

func TestDescKey(t *testing.T) {
	for in, want := range map[string]string{
		"Compra no débito - IFOOD *RESTAURANTE 1234": "ifood restaurante",
		"SUPERMERCADO BOA COMPRA 01/02":              "supermercado boa", // "compra" é palavra genérica de extrato
		"Pix enviado - Maria Silva":                  "maria silva",
	} {
		if got := descKey(in); got != want {
			t.Errorf("descKey(%q) = %q; quer %q", in, got, want)
		}
	}
}
