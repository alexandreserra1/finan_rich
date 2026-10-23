package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Options muda como o extrato é interpretado.
type Options struct {
	// Invert troca entradas e saídas. nil = automático: inverte as faturas de
	// cartão em CSV, onde o valor positivo é gasto.
	Invert *bool `json:"invert"`
}

// Status de cada linha na revisão.
const (
	StatusNew       = "novo"
	StatusImported  = "importado" // já foi importado antes (mesmo arquivo de novo)
	StatusDuplicate = "duplicado" // parece um lançamento digitado à mão
	StatusBillPay   = "fatura"    // pagamento de fatura: os gastos vêm do cartão
)

// De onde veio a categoria sugerida.
const (
	ReasonFile    = "arquivo"
	ReasonHistory = "histórico"
	ReasonKeyword = "palavra-chave"
)

// Item é uma linha pronta para o usuário revisar e importar.
type Item struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	Description string `json:"description"`
	AmountCents int64  `json:"amountCents"` // sempre positivo; o sentido está em Kind
	Kind        string `json:"kind"`        // entrada ou saida
	CategoryID  string `json:"categoryId"`  // sugestão ("" se nenhuma)
	Reason      string `json:"reason"`      // de onde veio a sugestão
	Status      string `json:"status"`
	Selected    bool   `json:"selected"` // vem marcado para importar?
}

type Preview struct {
	Format   string `json:"format"`
	Inverted bool   `json:"inverted"` // entradas e saídas trocadas
	CardBill bool   `json:"cardBill"` // o arquivo parece fatura de cartão
	Account  string `json:"account"`  // número da conta no arquivo (OFX), se houver
	From     string `json:"from"`
	To       string `json:"to"`
	Items    []Item `json:"items"`
}

// Build prepara a revisão: sentido de cada valor, categoria sugerida,
// duplicados e pagamentos de fatura.
func Build(l domain.Ledger, st Statement, o Options) Preview {
	p := Preview{Format: st.Format, CardBill: st.Card || st.Unsigned, Inverted: st.Unsigned, Account: st.Account}
	if o.Invert != nil {
		p.Inverted = *o.Invert
	}
	cats := newCategoryMatcher(l)
	existing := map[string]bool{}
	for _, t := range l.Transactions {
		existing[t.ID] = true
	}
	manual := manualIndex(l)
	seen := map[string]int{}

	for _, r := range st.Rows {
		amount := r.AmountCents
		if p.Inverted {
			amount = -amount
		}
		it := Item{Date: r.Date, Description: r.Description, Kind: domain.KindSaida, AmountCents: abs(amount), Status: StatusNew, Selected: true}
		if amount > 0 {
			it.Kind = domain.KindEntrada
		}
		it.ID = importID(st, r, seen)

		switch {
		case existing[it.ID]:
			it.Status, it.Selected = StatusImported, false
		case isBillPayment(r.Description):
			it.Status, it.Selected = StatusBillPay, false
		default:
			if manual.take(it) {
				it.Status, it.Selected = StatusDuplicate, false
			}
		}
		it.CategoryID, it.Reason = cats.suggest(r, it.Kind)
		p.Items = append(p.Items, it)
		if p.From == "" || it.Date < p.From {
			p.From = it.Date
		}
		if it.Date > p.To {
			p.To = it.Date
		}
	}
	sort.SliceStable(p.Items, func(i, j int) bool { return p.Items[i].Date > p.Items[j].Date })
	return p
}

// importID é estável: importar o mesmo arquivo de novo gera os mesmos IDs, e
// o app reconhece o que já entrou. Usa o identificador do banco (OFX) quando
// existe; senão, data + valor + descrição (+ a ordem, para compras iguais no
// mesmo dia).
func importID(st Statement, r Row, seen map[string]int) string {
	key := st.Account + "|" + r.FITID
	if r.FITID == "" {
		key = r.Date + "|" + strconv.FormatInt(r.AmountCents, 10) + "|" + normalize(r.Description)
	}
	seen[key]++
	if n := seen[key]; n > 1 {
		key += "|" + strconv.Itoa(n)
	}
	sum := sha256.Sum256([]byte(key))
	return "imp-" + hex.EncodeToString(sum[:12])
}

// isBillPayment reconhece o pagamento da fatura do cartão. No extrato da
// conta ele é uma saída, mas os gastos já aparecem um a um na fatura:
// importar os dois contaria o mesmo dinheiro duas vezes.
func isBillPayment(desc string) bool {
	d := normalize(desc)
	return strings.Contains(d, "fatura") && (strings.Contains(d, "pag") || strings.Contains(d, "cartao")) ||
		strings.Contains(d, "pagamento recebido") || strings.Contains(d, "pagamento de cartao")
}

// ---- duplicados digitados à mão ----

type manualTx struct {
	date, kind string
	amount     int64
}

type manualIdx map[manualTx]int

// manualIndex conta os lançamentos digitados (não importados) por data, tipo e
// valor: o banco descreve "SUPERMERCADO XYZ" e a pessoa digitou "Mercado",
// então a descrição não ajuda a comparar.
func manualIndex(l domain.Ledger) manualIdx {
	idx := manualIdx{}
	for _, t := range l.ActiveTransactions() {
		if !strings.HasPrefix(t.ID, "imp-") {
			idx[manualTx{t.Date, t.Kind, t.AmountCents}]++
		}
	}
	return idx
}

// take marca um lançamento digitado como "já usado" por esta linha.
func (m manualIdx) take(it Item) bool {
	k := manualTx{it.Date, it.Kind, it.AmountCents}
	if m[k] > 0 {
		m[k]--
		return true
	}
	return false
}
