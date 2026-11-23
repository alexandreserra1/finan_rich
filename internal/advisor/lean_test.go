package advisor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// Pedido menor para a IA: conselho e conversa vão sem ferramentas (e sem as linhas da voz
// sobre elas); busca leva só buscar, alterar e apagar; ação leva as do personagem.
func TestAskToolsOnlyWhenNeeded(t *testing.T) {
	ask := func(agent, q string) llm.Request {
		t.Helper()
		fake := &scriptedLLM{replies: []llm.Reply{{Text: "Ok."}}}
		if _, err := Ask(context.Background(), fake, agent, nil, q, "dados", NewTools(toolsLedger(), teamDay, nil), func(string) {}); err != nil {
			t.Fatal(err)
		}
		return fake.asked[0]
	}
	names := func(r llm.Request) string {
		var out []string
		for _, tl := range r.Tools {
			out = append(out, tl.Name)
		}
		return strings.Join(out, ",")
	}
	for _, c := range []struct{ agent, q string }{
		{"otavio", "o que eu faço com meu dinheiro?"}, {"lia", "onde posso economizar?"}, {"tito", "como chego na minha meta mais rápido?"},
		{"otavio", "me conta uma piada"}, {"otavio", "quero financiar um carro de R$ 60 mil, o que acha?"},
	} {
		r := ask(c.agent, c.q)
		if r.Tools != nil || strings.Contains(r.System, "Ferramentas:") || strings.Contains(r.System, "simular_") || strings.Contains(r.System, "comparar_meses") || !strings.Contains(r.System, "Você é ") {
			t.Errorf("%s %q: conselho vai sem ferramentas e sem as linhas sobre elas (%s):\n%s", c.agent, c.q, names(r), r.System)
		}
	}
	// A voz continua inteira no que não é ferramenta.
	if s := ask("otavio", "o que eu faço?").System; !strings.Contains(s, "Dívidas: ordene pela taxa de juros") || !strings.Contains(s, "Rotativo e cheque especial") {
		t.Errorf("voz do Otávio sem as regras dele:\n%s", s)
	}
	for _, q := range []string{"por que gastei tanto com delivery?", "Quanto gastei com Uber? O valor certo era 30", "o mercado está na categoria errada"} {
		if r := ask("lia", q); names(r) != "buscar_lancamentos,alterar_lancamento,apagar_lancamento" || !strings.Contains(r.System, "buscar_lancamentos:") || strings.Contains(r.System, "propor_lancamento") {
			t.Errorf("%q: busca leva só buscar, alterar e apagar (%s):\n%s", q, names(r), r.System)
		}
	}
	for _, c := range []struct{ agent, q, want string }{
		{"otavio", "lança 30 de cinema", "propor_lancamento"}, {"otavio", "paguei 200 no cartão", "registrar_pagamento"},
		{"tito", "quero juntar 3 mil até julho", "criar_meta"}, {"tito", "guardei 200 na viagem", "registrar_aporte"},
		{"lia", "apaga o uber repetido", "apagar_lancamento"},
	} {
		r := ask(c.agent, c.q)
		if len(r.Tools) != len(persona(c.agent).Tools) || !strings.Contains(names(r), c.want) || !strings.Contains(r.System, "Ferramentas:") {
			t.Errorf("%s %q: ação leva as ferramentas do personagem (%s)", c.agent, c.q, names(r))
		}
	}
	// O pedido de conselho fica bem menor (o das ferramentas tem uns 5 mil caracteres a mais).
	size := func(r llm.Request) int {
		b, _ := json.Marshal(r.Tools)
		return len(r.System) + len(b)
	}
	if lean, full := size(ask("otavio", "o que eu faço?")), size(ask("otavio", "lança 30 de cinema")); full-lean < 4500 {
		t.Errorf("sem ferramentas: %d caracteres; com: %d", lean, full)
	}
}

// Sem ferramentas, a IA disse que deixou um cartão pronto: era um pedido de ação que o app
// não reconheceu. Uma nova tentativa, com as ferramentas do personagem e o aviso.
func TestAskArmsToolsAfterFalseClaim(t *testing.T) {
	call := llm.ToolCall{ID: "c1", Name: "propor_lancamento", Args: json.RawMessage(`{"descricao":"Cinema","valor":"30","categoria":"Transporte"}`)}
	fake := &scriptedLLM{replies: []llm.Reply{
		{Text: "Deixei pronto o lançamento para você confirmar."},
		{ToolCalls: []llm.ToolCall{call}},
		{Text: "Pronto: confira o cartão do cinema."},
	}}
	var props []Proposal
	got, err := Ask(context.Background(), fake, "otavio", nil, "aquele cinema de ontem foi 30", "dados", NewTools(toolsLedger(), teamDay, func(p Proposal) { props = append(props, p) }), func(string) {})
	if err != nil || got != "Pronto: confira o cartão do cinema." || len(props) != 1 || len(fake.asked) != 3 {
		t.Fatalf("resposta %q, %d cartões, %d pedidos, %v", got, len(props), len(fake.asked), err)
	}
	if fake.asked[0].Tools != nil || len(fake.asked[1].Tools) != len(persona("otavio").Tools) || !strings.Contains(fake.asked[1].System, "Ferramentas:") {
		t.Errorf("1º pedido com %d ferramentas, 2º com %d", len(fake.asked[0].Tools), len(fake.asked[1].Tools))
	}
	if nudge := fake.asked[1].Messages[len(fake.asked[1].Messages)-1].Content; !strings.Contains(nudge, "nenhum cartão foi criado") {
		t.Errorf("o segundo pedido não avisou a IA: %q", nudge)
	}
	// Errou de novo, já com as ferramentas: a resposta não engana a pessoa (2 pedidos só).
	fake = &scriptedLLM{replies: []llm.Reply{{Text: "Deixei pronto o cartão."}, {Text: "Deixei pronto, é só confirmar."}}}
	got, _ = Ask(context.Background(), fake, "otavio", nil, "aquele cinema de ontem foi 30", "dados", NewTools(toolsLedger(), teamDay, func(Proposal) {}), func(string) {})
	if !strings.Contains(got, "Não consegui preparar o cartão") || len(fake.asked) != 2 {
		t.Errorf("segunda afirmação falsa: %q (%d pedidos)", got, len(fake.asked))
	}
}
