// Package report gera os arquivos que o usuário leva para fora do app:
// planilha do Excel (.xlsx), CSV, PDF e o backup (.json).
//
// É Go puro e sem dependências (só a biblioteca padrão), então roda também no
// aparelho, compilado para WebAssembly: exportar funciona sem conexão.
//
// Os dados viram primeiro tabelas genéricas (Table), e cada formato só sabe
// desenhar tabelas. Para exportar algo novo, monte uma Table.
package report

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Kind diz como uma coluna é escrita em cada formato.
type Kind int

const (
	Text     Kind = iota
	Money         // centavos (int64): número no Excel, "1234,56" no CSV, "R$ 1.234,56" no PDF
	Date          // "2026-10-05": data de verdade no Excel
	Number        // inteiro
	Percent       // inteiro de 0 a 100
	LongText      // texto com quebra de linha (notas)
)

type Column struct {
	Title string
	Kind  Kind
	Width int // largura aproximada em caracteres
}

// Cell guarda o valor bruto: string, int64 ou int, conforme a coluna.
type Cell any

type Table struct {
	Name    string // nome da aba no Excel e do arquivo CSV
	Title   string // título no PDF
	Columns []Column
	Rows    [][]Cell
	Total   []Cell // linha de total (opcional)
}

// ---- Tabelas do finan_rich ----

func transactionsTable(l domain.Ledger, p Period) Table {
	cats := l.CategoryIndex()
	t := Table{Name: "Lançamentos", Title: "Lançamentos", Columns: []Column{
		{"Data", Date, 11}, {"Tipo", Text, 8}, {"Descrição", Text, 34}, {"Categoria", Text, 22},
		{"Grupo", Text, 14}, {"Valor (R$)", Money, 14},
	}}
	txs := l.ActiveTransactions()
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Date < txs[j].Date }) // planilha: do mais antigo ao mais novo
	var total int64
	for _, tx := range txs {
		if !p.Contains(tx.Date) {
			continue
		}
		cat := cats[tx.CategoryID]
		amount, kind := tx.AmountCents, "Entrada"
		if tx.Kind == domain.KindSaida {
			amount, kind = -amount, "Saída"
		}
		total += amount
		t.Rows = append(t.Rows, []Cell{tx.Date, kind, tx.Description, cat.Name, bucketLabel(cat.Bucket), amount})
	}
	t.Total = []Cell{"", "", "Saldo do período", "", "", total}
	return t
}

func monthsTable(l domain.Ledger, p Period) Table {
	t := Table{Name: "Resumo por mês", Title: "Resumo por mês", Columns: []Column{
		{"Mês", Text, 12}, {"Entradas", Money, 14}, {"Saídas", Money, 14}, {"Saldo", Money, 14},
		{"Necessidades", Money, 14}, {"Desejos", Money, 14}, {"Investimentos", Money, 14},
	}}
	var in, out, nec, des, inv int64
	for _, m := range p.Months() {
		s := finance.Summarize(m, l)
		spent := bucketSpent(s)
		t.Rows = append(t.Rows, []Cell{m.Short(), s.InCents, s.OutCents, s.BalanceCents(),
			spent[domain.BucketNecessidades], spent[domain.BucketDesejos], spent[domain.BucketInvestimentos]})
		in, out = in+s.InCents, out+s.OutCents
		nec, des, inv = nec+spent[domain.BucketNecessidades], des+spent[domain.BucketDesejos], inv+spent[domain.BucketInvestimentos]
	}
	t.Total = []Cell{"Total", in, out, in - out, nec, des, inv}
	return t
}

func categoriesTable(s finance.Summary) Table {
	t := Table{Name: "Por categoria", Title: "Gastos por categoria", Columns: []Column{
		{"Categoria", Text, 26}, {"Grupo", Text, 14}, {"Gasto (R$)", Money, 14}, {"% das saídas", Percent, 12},
	}}
	for _, c := range s.Categories {
		t.Rows = append(t.Rows, []Cell{c.Name, bucketLabel(c.Bucket), c.SpentCents, c.SharePct})
	}
	t.Total = []Cell{"Total", "", s.OutCents, ""}
	return t
}

func splitTable(s finance.Summary) Table {
	t := Table{Name: "Divisão do salário", Title: "Divisão do salário (planejado x gasto)", Columns: []Column{
		{"Grupo", Text, 16}, {"Meta (%)", Percent, 10}, {"Planejado (R$)", Money, 15}, {"Gasto (R$)", Money, 15},
		{"Sobra (R$)", Money, 15}, {"Usado (%)", Percent, 10},
	}}
	for _, b := range s.Buckets {
		t.Rows = append(t.Rows, []Cell{bucketLabel(b.Bucket), b.Pct, b.PlannedCents, b.SpentCents, b.RemainingCents(), b.UsedPct()})
	}
	return t
}

func debtsTable(l domain.Ledger) Table {
	plan := finance.PlanDebts(l.ActiveDebts())
	t := Table{Name: "Dívidas", Title: "Dívidas", Columns: []Column{
		{"Dívida", Text, 24}, {"Credor", Text, 18}, {"Saldo (R$)", Money, 14}, {"Juros a.m.", Text, 10},
		{"Juros/mês (R$)", Money, 14}, {"Parcela mínima (R$)", Money, 16}, {"Vence dia", Number, 9},
		{"Só pagando o mínimo", Text, 30},
	}}
	for _, d := range plan.Debts {
		payoff := "informe a parcela"
		switch {
		case d.Payable:
			payoff = strconv.Itoa(d.Months) + " meses, " + money.Format(d.InterestCents) + " de juros"
		case d.MinPaymentCents > 0:
			payoff = "não cobre os juros: a dívida cresce"
		}
		due := Cell("")
		if d.DueDay > 0 {
			due = d.DueDay
		}
		t.Rows = append(t.Rows, []Cell{d.Name, d.Creditor, d.BalanceCents, ratePct(d.MonthlyRateBP),
			d.MonthlyInterestCents, d.MinPaymentCents, due, payoff})
	}
	t.Total = []Cell{"Total", "", plan.TotalCents, "", plan.MonthlyInterest, plan.TotalMinCents, "", ""}
	return t
}

func notesTable(l domain.Ledger) Table {
	t := Table{Name: "Notas", Title: "Notas", Columns: []Column{
		{"Título", Text, 30}, {"Texto", LongText, 70}, {"Fixada", Text, 7}, {"Atualizada em", Text, 17},
	}}
	for _, n := range l.ActiveNotes() {
		pinned := ""
		if n.Pinned {
			pinned = "sim"
		}
		t.Rows = append(t.Rows, []Cell{n.Title, n.Body, pinned, msTime(n.UpdatedAt)})
	}
	return t
}

func categoryListTable(l domain.Ledger) Table {
	t := Table{Name: "Categorias", Title: "Categorias", Columns: []Column{{"Categoria", Text, 28}, {"Grupo", Text, 16}}}
	cats := append([]domain.Category(nil), l.Categories...)
	sort.SliceStable(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })
	for _, c := range cats {
		if !c.Deleted {
			t.Rows = append(t.Rows, []Cell{c.Name, bucketLabel(c.Bucket)})
		}
	}
	return t
}

// ---- utilidades ----

func bucketSpent(s finance.Summary) map[string]int64 {
	out := map[string]int64{}
	for _, b := range s.Buckets {
		out[b.Bucket] = b.SpentCents
	}
	return out
}

func bucketLabel(b string) string {
	switch b {
	case domain.BucketNecessidades:
		return "Necessidades"
	case domain.BucketDesejos:
		return "Desejos"
	case domain.BucketInvestimentos:
		return "Investimentos"
	case domain.BucketRenda:
		return "Renda"
	}
	return "Sem grupo"
}

// msTime mostra uma hora em milissegundos no fuso de Brasília.
func msTime(ms int64) string {
	return time.UnixMilli(ms).In(brazil).Format("02/01/2006 15:04")
}

// brazil é o fuso usado nos relatórios. FixedZone em vez de LoadLocation: no
// navegador (WebAssembly) não há banco de fusos horários.
var brazil = time.FixedZone("BRT", -3*60*60)

// ratePct mostra juros em pontos-base como "12,50%".
func ratePct(bp int) string {
	return fmt.Sprintf("%d,%02d%%", bp/100, bp%100)
}
