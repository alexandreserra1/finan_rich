package advisor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func toolsLedger() domain.Ledger {
	cat := func(id, name, bucket string) domain.Category {
		return domain.Category{Meta: domain.Meta{ID: id, UpdatedAt: 1}, Name: name, Bucket: bucket}
	}
	tx := func(id, kind, desc, cat, date string, cents int64) domain.Transaction {
		return domain.Transaction{Meta: domain.Meta{ID: id, UpdatedAt: 1}, Kind: kind, Description: desc, AmountCents: cents, CategoryID: cat, Date: date}
	}
	return domain.Ledger{
		Categories: []domain.Category{cat("sal", "Salário", domain.BucketRenda), cat("res", "Restaurantes e delivery", domain.BucketDesejos), cat("tra", "Transporte", domain.BucketNecessidades)},
		Transactions: []domain.Transaction{
			tx("1", domain.KindSaida, "iFood *Pizza", "res", "2026-03-10", 5230),
			tx("2", domain.KindSaida, "IFOOD lanche", "res", "2026-09-02", 3000),
			tx("3", domain.KindSaida, "Uber", "tra", "2026-09-05", 2500),
			tx("4", domain.KindEntrada, "Salário", "sal", "2026-09-05", 500000),
			{Meta: domain.Meta{ID: "5", UpdatedAt: 2, Deleted: true}, Kind: domain.KindSaida, Description: "iFood apagado", AmountCents: 99999, CategoryID: "res", Date: "2026-09-03"},
		},
	}
}

func call(name, args string) llm.ToolCall {
	return llm.ToolCall{ID: "c", Name: name, Args: json.RawMessage(args)}
}

func TestSearchTool(t *testing.T) {
	tools := NewTools(toolsLedger(), time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), nil)
	cases := []struct{ args, want string }{
		{`{"texto":"ifood"}`, "2 lançamentos (de 2026-03-10 a 2026-09-02). Saídas: R$ 82,30"}, // acento/caixa não importam; apagado não conta
		{`{"texto":"ifood","de":"2026-09"}`, "1 lançamentos (de 2026-09-02 a 2026-09-02). Saídas: R$ 30,00"},
		{`{"categoria":"transporte","ate":"2026-09"}`, "Saídas: R$ 25,00"},
		{`{"tipo":"entrada"}`, "Entradas: R$ 5.000,00"},
		{`{"texto":"netflix"}`, "Nenhum lançamento"},
	}
	for _, c := range cases {
		if got := tools.Run(call("buscar_lancamentos", c.args)); !strings.Contains(got, c.want) {
			t.Errorf("buscar %s:\n%s\nquer %q", c.args, got, c.want)
		}
	}
}

func proposeAll(t *testing.T) (*Tools, *[]Proposal) {
	var got []Proposal
	return NewTools(toolsLedger(), time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), func(p Proposal) { got = append(got, p) }), &got
}

func record[T any](t *testing.T, p Proposal) T {
	t.Helper()
	var r T
	if err := json.Unmarshal(p.Record, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestProposeTransaction(t *testing.T) {
	tools, got := proposeAll(t)
	out := tools.Run(call("propor_lancamento", `{"descricao":"Almoço","valor":"45,90","categoria":"restaurantes e delivery"}`))
	if len(*got) != 1 || !strings.Contains(out, "NÃO foi feito") {
		t.Fatalf("proposta: %v %q", *got, out)
	}
	p := (*got)[0]
	tx := record[domain.Transaction](t, p)
	if p.Action != ActionCreate || p.Type != domain.TypeTransaction || tx.Kind != domain.KindSaida || tx.AmountCents != 4590 || tx.CategoryID != "res" ||
		tx.Date != "2026-10-16" || !strings.HasPrefix(tx.ID, "ia-") || p.Label != "Novo gasto: 16/10/2026 · Almoço · Restaurantes e delivery · -R$ 45,90" {
		t.Errorf("proposta = %+v %+v", p, tx)
	}
	// Valor como número JSON também vale; entrada só em categoria de renda.
	tools.Run(call("propor_lancamento", `{"descricao":"Freela","valor":1500.5,"tipo":"entrada","categoria":"Salário","data":"2026-10-01"}`))
	if tx := record[domain.Transaction](t, (*got)[1]); tx.AmountCents != 150050 || tx.Kind != domain.KindEntrada {
		t.Errorf("entrada = %+v", tx)
	}
	for args, want := range map[string]string{
		`{"descricao":"x","valor":"10","categoria":"Viagem"}`:                   "Use uma destas: Restaurantes e delivery, Transporte",
		`{"descricao":"x","valor":"10","tipo":"entrada","categoria":"Uber"}`:    "Use uma destas: Salário",
		`{"descricao":"x","valor":"-3","categoria":"Transporte"}`:               "valor inválido",
		`{"descricao":"","valor":"3","categoria":"Transporte"}`:                 "informe uma descrição",
		`{"descricao":"x","valor":"3","categoria":"Transporte","data":"ontem"}`: "data inválida",
	} {
		if out := tools.Run(call("propor_lancamento", args)); !strings.Contains(out, want) {
			t.Errorf("%s = %q; quer %q", args, out, want)
		}
	}
	if len(*got) != 2 {
		t.Errorf("pedidos inválidos não podem virar proposta: %d", len(*got))
	}
}

func TestEditAndDeleteTransaction(t *testing.T) {
	tools, got := proposeAll(t)
	if out := tools.Run(call("buscar_lancamentos", `{"texto":"uber"}`)); !strings.Contains(out, "[3] 05/09/2026 · Uber") {
		t.Fatalf("a busca precisa dar o id: %q", out)
	}
	tools.Run(call("alterar_lancamento", `{"id":"3","valor":"30","categoria":"Restaurantes e delivery"}`))
	p := (*got)[0]
	tx := record[domain.Transaction](t, p)
	if p.Action != ActionEdit || tx.ID != "3" || tx.AmountCents != 3000 || tx.CategoryID != "res" || tx.Description != "Uber" || tx.UpdatedAt <= 1 ||
		p.Label != "Alterar Uber (05/09/2026): valor R$ 25,00 → R$ 30,00; categoria Transporte → Restaurantes e delivery" {
		t.Errorf("alterar = %+v %+v", p, tx)
	}
	tools.Run(call("apagar_lancamento", `{"id":"3"}`))
	p = (*got)[1]
	if tx := record[domain.Transaction](t, p); p.Action != ActionDelete || !tx.Deleted || tx.ID != "3" || p.Label != "Apagar: 05/09/2026 · Uber · Restaurantes e delivery · -R$ 30,00" { // parte da alteração acima
		t.Errorf("apagar = %+v %+v", p, tx)
	}
	for args, want := range map[string]string{
		`{"id":"nao-existe"}`: "não encontrado",
		`{"id":"5"}`:          "não encontrado", // apagado não conta
	} {
		if out := tools.Run(call("apagar_lancamento", args)); !strings.Contains(out, want) {
			t.Errorf("apagar %s = %q", args, out)
		}
	}
	if !strings.Contains(string((*got)[0].Before), `"amountCents":2500`) {
		t.Errorf("a alteração precisa guardar como o registro estava: %s", (*got)[0].Before)
	}
	fresh, _ := proposeAll(t)
	if out := fresh.Run(call("alterar_lancamento", `{"id":"3","valor":"25,00"}`)); !strings.Contains(out, "nada muda") {
		t.Errorf("alterar sem mudança = %q", out)
	}
}

func TestGoalAndDebt(t *testing.T) {
	tools, got := proposeAll(t)
	tools.Run(call("criar_meta", `{"nome":"Viagem","valor":"6.000","prazo":"2027-06","ja_guardado":"1500"}`))
	g := record[domain.Goal](t, (*got)[0])
	if (*got)[0].Type != domain.TypeGoal || g.TargetCents != 600000 || g.SavedCents != 150000 || g.Deadline != "2027-06" || g.StartDate != "2026-10-16" ||
		(*got)[0].Label != "Nova meta: Viagem · R$ 6.000,00 até junho de 2027 · já guardado R$ 1.500,00" {
		t.Errorf("meta = %+v %+v", (*got)[0], g)
	}
	tools.Run(call("criar_divida", `{"nome":"Cartão","saldo":"3200","juros_mes":"12,9%","parcela_minima":"480","vencimento":15}`))
	d := record[domain.Debt](t, (*got)[1])
	if (*got)[1].Type != domain.TypeDebt || d.BalanceCents != 320000 || d.MonthlyRateBP != 1290 || d.MinPaymentCents != 48000 || d.DueDay != 15 {
		t.Errorf("dívida = %+v %+v", (*got)[1], d)
	}
	if out := tools.Run(call("criar_meta", `{"nome":"X","valor":"100","prazo":"junho"}`)); !strings.Contains(out, "não deu para preparar") {
		t.Errorf("prazo inválido = %q", out)
	}
}

func TestApplied(t *testing.T) {
	l := toolsLedger()
	tools, got := proposeAll(t)
	tools.Run(call("propor_lancamento", `{"descricao":"Almoço","valor":"45,90","categoria":"Restaurantes e delivery"}`))
	tools.Run(call("alterar_lancamento", `{"id":"3","valor":"30"}`))
	tools.Run(call("apagar_lancamento", `{"id":"2"}`))
	create, edit, del := (*got)[0], (*got)[1], (*got)[2]
	if Applied(l, create) || Applied(l, edit) || Applied(l, del) {
		t.Fatal("nada foi confirmado ainda")
	}
	// A pessoa confirmou (o aparelho grava com updatedAt novo).
	newTx := record[domain.Transaction](t, create)
	l.Transactions = append(l.Transactions, newTx)
	edited := record[domain.Transaction](t, edit)
	edited.UpdatedAt += 50
	l.Transactions[2] = edited
	l.Transactions[1].Deleted = true
	if !Applied(l, create) || !Applied(l, edit) || !Applied(l, del) {
		t.Error("confirmadas deveriam aparecer como feitas")
	}
}

// scriptedLLM responde em sequência: cada item é uma rodada.
type scriptedLLM struct {
	replies []llm.Reply
	errs    []error
	asked   []llm.Request
}

func (s *scriptedLLM) Chat(_ context.Context, r llm.Request, onText func(string)) (llm.Reply, error) {
	i := len(s.asked)
	s.asked = append(s.asked, r)
	if i < len(s.errs) && s.errs[i] != nil {
		return llm.Reply{}, s.errs[i]
	}
	onText(s.replies[i].Text)
	return s.replies[i], nil
}

func TestAskWithTools(t *testing.T) {
	fake := &scriptedLLM{replies: []llm.Reply{
		{ToolCalls: []llm.ToolCall{call("buscar_lancamentos", `{"texto":"ifood"}`)}},
		{Text: "Você gastou **R$ 82,30** com iFood."},
	}}
	tools := NewTools(toolsLedger(), time.Now(), nil)
	answer, err := Ask(context.Background(), fake, nil, "quanto gastei com ifood?", "resumo", tools, func(string) {})
	if err != nil || answer != "Você gastou **R$ 82,30** com iFood." {
		t.Fatalf("answer=%q err=%v", answer, err)
	}
	second := fake.asked[1]
	last := second.Messages[len(second.Messages)-1]
	if len(fake.asked) != 2 || last.Role != llm.RoleTool || !strings.Contains(last.Content, "R$ 82,30") ||
		len(second.Tools) != 8 || !strings.Contains(second.System, "propor_lancamento") {
		t.Errorf("segunda rodada = %+v", second)
	}
}

func TestAskFallsBackWithoutTools(t *testing.T) {
	fake := &scriptedLLM{
		errs:    []error{&llm.APIError{Status: 400, Message: "model does not support tools"}},
		replies: []llm.Reply{{}, {Text: "Resposta sem ferramentas."}},
	}
	answer, err := Ask(context.Background(), fake, nil, "oi", "resumo", NewTools(toolsLedger(), time.Now(), nil), func(string) {})
	if err != nil || answer != "Resposta sem ferramentas." || fake.asked[1].Tools != nil || fake.asked[1].System != SystemPrompt {
		t.Fatalf("answer=%q err=%v", answer, err)
	}
}

func TestAskStopsCallingTools(t *testing.T) {
	loop := llm.Reply{ToolCalls: []llm.ToolCall{call("buscar_lancamentos", `{}`)}}
	fake := &scriptedLLM{replies: []llm.Reply{loop, loop, loop, loop, {Text: "Fim."}}}
	answer, err := Ask(context.Background(), fake, nil, "oi", "resumo", NewTools(toolsLedger(), time.Now(), nil), func(string) {})
	if err != nil || answer != "Fim." || len(fake.asked) != maxToolRounds+1 || fake.asked[maxToolRounds].Tools != nil {
		t.Fatalf("answer=%q err=%v rodadas=%d", answer, err, len(fake.asked))
	}
	// Sem "tools", o pedido não pode levar blocos de ferramenta (a Anthropic recusa):
	// os resultados vão como texto.
	last := fake.asked[maxToolRounds].Messages
	for _, m := range last {
		if m.Role == llm.RoleTool || len(m.ToolCalls) > 0 {
			t.Fatalf("última rodada ainda tem bloco de ferramenta: %+v", m)
		}
	}
	if !strings.Contains(last[len(last)-1].Content, "buscar_lancamentos") {
		t.Errorf("os resultados das buscas deveriam ir como texto: %+v", last[len(last)-1])
	}
}

func TestContributeAndPay(t *testing.T) {
	l := toolsLedger()
	l.Goals = []domain.Goal{
		{Meta: domain.Meta{ID: "g1", UpdatedAt: 1}, Name: "Viagem", TargetCents: 600000, SavedCents: 100000, StartDate: "2026-01-01"},
		{Meta: domain.Meta{ID: "g2", UpdatedAt: 1}, Name: "Reserva", TargetCents: 900000, CategoryID: "res", StartDate: "2026-01-01"},
	}
	l.Debts = []domain.Debt{{Meta: domain.Meta{ID: "d1", UpdatedAt: 1}, Name: "Cartão Nubank", BalanceCents: 320000, MonthlyRateBP: 1290, MinPaymentCents: 48000}}
	var got []Proposal
	tools := NewTools(l, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), func(p Proposal) { got = append(got, p) })

	// Meta sem categoria: soma no "já guardado".
	tools.Run(call("registrar_aporte", `{"meta":"viagem","valor":"500"}`))
	if g := record[domain.Goal](t, got[0]); got[0].Action != ActionEdit || g.SavedCents != 150000 || g.ID != "g1" ||
		got[0].Label != "Aporte na meta Viagem: guardado R$ 1.000,00 → R$ 1.500,00 (de R$ 6.000,00)" {
		t.Errorf("aporte = %+v %+v", got[0], g)
	}
	// Meta com categoria: o aporte é um lançamento nela.
	tools.Run(call("registrar_aporte", `{"meta":"Reserva","valor":"200"}`))
	if tx := record[domain.Transaction](t, got[1]); got[1].Action != ActionCreate || tx.CategoryID != "res" || tx.AmountCents != 20000 || tx.Description != "Aporte: Reserva" {
		t.Errorf("aporte com categoria = %+v %+v", got[1], tx)
	}
	// Sem a categoria "Pagamento de dívidas": só baixa o saldo.
	tools.Run(call("registrar_pagamento", `{"divida":"cartão nubank","valor":"500"}`))
	if d := record[domain.Debt](t, got[2]); len(got) != 3 || d.BalanceCents != 270000 ||
		got[2].Label != "Pagamento de R$ 500,00 em Cartão Nubank: saldo R$ 3.200,00 → R$ 2.700,00" {
		t.Errorf("pagamento = %+v %+v (%d cartões)", got[2], d, len(got))
	}
	// Com a categoria: baixa o saldo e lança o gasto (dois cartões); pagar mais que o saldo zera.
	l.Categories = append(l.Categories, domain.Category{Meta: domain.Meta{ID: "pd", UpdatedAt: 1}, Name: "Pagamento de dívidas", Bucket: domain.BucketNecessidades})
	got = nil
	tools = NewTools(l, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), func(p Proposal) { got = append(got, p) })
	out := tools.Run(call("registrar_pagamento", `{"divida":"Cartão Nubank","valor":"4000"}`))
	if len(got) != 2 || record[domain.Debt](t, got[0]).BalanceCents != 0 || record[domain.Transaction](t, got[1]).CategoryID != "pd" || !strings.Contains(out, "; Novo gasto") {
		t.Errorf("pagamento com lançamento = %+v %q", got, out)
	}
	for args, want := range map[string]string{
		`{"meta":"Casa","valor":"10"}`:  "Metas: Viagem, Reserva",
		`{"meta":"Viagem","valor":"0"}`: "valor do aporte inválido",
	} {
		if out := tools.Run(call("registrar_aporte", args)); !strings.Contains(out, want) {
			t.Errorf("%s = %q", args, out)
		}
	}
	if out := tools.Run(call("registrar_pagamento", `{"divida":"Carro","valor":"10"}`)); !strings.Contains(out, "Dívidas: Cartão Nubank") {
		t.Errorf("dívida inexistente = %q", out)
	}
}

// Dois pagamentos na mesma resposta: o segundo parte do primeiro.
func TestPaymentsChain(t *testing.T) {
	l := toolsLedger()
	l.Debts = []domain.Debt{{Meta: domain.Meta{ID: "d1", UpdatedAt: 1}, Name: "Cartão", BalanceCents: 320000, MonthlyRateBP: 1000, MinPaymentCents: 30000}}
	var got []Proposal
	tools := NewTools(l, time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC), func(p Proposal) { got = append(got, p) })
	tools.Run(call("registrar_pagamento", `{"divida":"Cartão","valor":"500","data":"2026-01-10"}`))
	tools.Run(call("registrar_pagamento", `{"divida":"Cartão","valor":"500","data":"2026-02-10"}`))
	var debts []domain.Debt
	for _, p := range got {
		if p.Type == domain.TypeDebt {
			debts = append(debts, record[domain.Debt](t, p))
		}
	}
	if len(debts) != 2 || debts[0].BalanceCents != 270000 || debts[1].BalanceCents != 220000 {
		t.Errorf("saldos = %+v", debts)
	}
	if l.Debts[0].BalanceCents != 320000 {
		t.Error("as ferramentas não podem mexer nos dados de quem chamou")
	}
}

func TestSearchWholeYear(t *testing.T) {
	tools := NewTools(toolsLedger(), time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC), nil)
	if out := tools.Run(call("buscar_lancamentos", `{"texto":"uber","de":"2026","ate":"2026"}`)); !strings.Contains(out, "Uber") {
		t.Errorf("período só com o ano: %q", out)
	}
}
