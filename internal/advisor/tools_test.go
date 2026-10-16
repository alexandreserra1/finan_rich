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

func TestProposeTool(t *testing.T) {
	var got []Proposal
	tools := NewTools(toolsLedger(), time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), func(p Proposal) { got = append(got, p) })

	out := tools.Run(call("propor_lancamento", `{"descricao":"Almoço","valor":"45,90","categoria":"restaurantes e delivery"}`))
	if len(got) != 1 || !strings.Contains(out, "NÃO foi lançada") {
		t.Fatalf("proposta: %v %q", got, out)
	}
	p := got[0]
	if p.Kind != domain.KindSaida || p.AmountCents != 4590 || p.CategoryID != "res" || p.Date != "2026-10-16" || !strings.HasPrefix(p.ID, "ia-") || p.CategoryName != "Restaurantes e delivery" {
		t.Errorf("proposta = %+v", p)
	}
	// Valor como número JSON também vale; entrada só em categoria de renda.
	tools.Run(call("propor_lancamento", `{"descricao":"Freela","valor":1500.5,"tipo":"entrada","categoria":"Salário","data":"2026-10-01"}`))
	if len(got) != 2 || got[1].AmountCents != 150050 || got[1].Kind != domain.KindEntrada {
		t.Errorf("entrada = %+v", got[1])
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
	if len(got) != 2 {
		t.Errorf("pedidos inválidos não podem virar proposta: %d", len(got))
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
		len(second.Tools) != 2 || !strings.Contains(second.System, "propor_lancamento") {
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
}
