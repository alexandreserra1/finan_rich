package advisor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// criar_conta_fixa: renda e contas do mês viram cartões de conta fixa (recorrência).
func TestNewRecurringTool(t *testing.T) {
	var got []Proposal
	tools := NewTools(toolsLedger(), teamDay, func(p Proposal) { got = append(got, p) })
	out := tools.Run(call("criar_conta_fixa", `{"descricao":"Aluguel","valor":"1.500","tipo":"saida","dia":"10","categoria":"Transporte"}`))
	if !strings.Contains(out, "Ainda NÃO foi feito") || len(got) != 1 {
		t.Fatalf("saída %q, %d propostas", out, len(got))
	}
	var r domain.Recurring
	json.Unmarshal(got[0].Record, &r)
	if got[0].Type != domain.TypeRecurring || r.Kind != domain.KindSaida || r.AmountCents != 150000 || r.Day != 10 || r.Start != "2026-10" || r.CategoryID != "tra" {
		t.Errorf("proposta = %+v, registro %+v", got[0], r)
	}
	if want := "Nova conta fixa: Aluguel · R$ 1.500,00 todo dia 10"; got[0].Label != want {
		t.Errorf("rótulo = %q, quer %q", got[0].Label, want)
	}
	tools.Run(call("criar_conta_fixa", `{"descricao":"Salário","valor":"4500","tipo":"entrada","dia":"5","categoria":"Salário"}`))
	if len(got) != 2 || !strings.HasPrefix(got[1].Label, "Nova renda fixa: Salário · R$ 4.500,00 todo dia 5") {
		t.Errorf("renda: %+v", got)
	}
	for _, args := range []string{`{"descricao":"Luz","valor":"200","dia":"40","categoria":"Transporte"}`, `{"descricao":"Luz","valor":"0","dia":"5","categoria":"Transporte"}`} {
		if out := tools.Run(call("criar_conta_fixa", args)); !strings.Contains(out, "não deu") {
			t.Errorf("%s: %q", args, out)
		}
	}
	// A conta fixa confirmada aparece como aplicada no cartão.
	l := toolsLedger()
	var rec domain.Recurring
	json.Unmarshal(got[0].Record, &rec)
	l.Recurring = append(l.Recurring, rec)
	if !Applied(l, got[0]) || Applied(toolsLedger(), got[0]) {
		t.Error("Applied não reconhece a conta fixa")
	}
}

// O começo do zero: o Otávio pergunta uma coisa por vez e só usa as ferramentas de criar.
func TestOnboardingMode(t *testing.T) {
	ask := func(history []llm.Message, text string) llm.Request {
		fake := &scriptedLLM{replies: []llm.Reply{{Text: "Quanto você recebe por mês?"}}}
		Answer(context.Background(), fake, Question{History: history, Text: text, Ledger: domain.Ledger{Categories: domain.DefaultCategories(1)}, Today: teamDay, Anonymize: true}, func(string) {}, func(Proposal) {})
		return fake.asked[0]
	}
	names := func(req llm.Request) string {
		var n []string
		for _, d := range req.Tools {
			n = append(n, d.Name)
		}
		return strings.Join(n, ",")
	}
	start := ask(nil, finance.StartQuestion)
	if !strings.Contains(start.System, "Começo do zero") || names(start) != "propor_lancamento,criar_meta,criar_divida,criar_conta_fixa" {
		t.Errorf("início: ferramentas %s\n%s", names(start), start.System)
	}
	next := ask([]llm.Message{{Role: llm.RoleUser, Content: finance.StartQuestion}, {Role: llm.RoleAssistant, Content: "Quanto você recebe?"}}, "4.500 no dia 5")
	if !strings.Contains(next.System, "Começo do zero") || !strings.Contains(names(next), "criar_conta_fixa") {
		t.Error("a resposta seguinte continua no começo do zero")
	}
	// Depois das 4 respostas (renda, contas, dívidas, meta), a conversa volta ao normal.
	var h []llm.Message
	for _, m := range []string{finance.StartQuestion, "4.500 no dia 5", "aluguel 1.500 dia 10", "não tenho", "viagem 6 mil"} {
		h = append(h, llm.Message{Role: llm.RoleUser, Content: m}, llm.Message{Role: llm.RoleAssistant, Content: "ok"})
	}
	if !onboarding("", h[:8], "viagem 6 mil") || onboarding("", h, "e agora?") || onboarding("lia", nil, finance.StartQuestion) {
		t.Error("o começo do zero vale só para o Otávio e só até a 4ª resposta")
	}
	if normal := ask(nil, "devo trocar de banco?"); strings.Contains(normal.System, "Começo do zero") || strings.Contains(names(normal), "criar_conta_fixa") {
		t.Error("conversa normal não entra no começo do zero")
	}
	// Sem IA: o caminho para começar à mão.
	if got := Offline(Question{Text: finance.StartQuestion, Ledger: domain.Ledger{}, Today: teamDay}); !strings.Contains(got, "Repetir todo mês") || !strings.Contains(got, "Dívidas") {
		t.Errorf("sem IA:\n%s", got)
	}
}

// Sem categoria (ou com uma que não existe), o app escolhe pela descrição, com as mesmas
// regras da importação: a IA não precisa perguntar a categoria à pessoa.
func TestRecurringCategoryFromDescription(t *testing.T) {
	var got []Proposal
	tools := NewTools(domain.Ledger{Categories: domain.DefaultCategories(1)}, teamDay, func(p Proposal) { got = append(got, p) })
	for args, want := range map[string]string{
		`{"descricao":"Internet","valor":"100","dia":15}`:                        "padrao-contas-casa",
		`{"descricao":"Aluguel","valor":"1500","dia":10,"categoria":"Conforto"}`: "padrao-moradia",
		`{"descricao":"Salário","valor":"4500","tipo":"entrada","dia":5}`:        "padrao-salario",
	} {
		got = nil
		if out := tools.Run(call("criar_conta_fixa", args)); len(got) != 1 {
			t.Errorf("%s: %s", args, out)
			continue
		}
		var r domain.Recurring
		json.Unmarshal(got[0].Record, &r)
		if r.CategoryID != want {
			t.Errorf("%s: categoria %q, quer %q", args, r.CategoryID, want)
		}
	}
	// Sem pista nenhuma, continua pedindo uma categoria que existe (a lista vai junto).
	if out := tools.Run(call("criar_conta_fixa", `{"descricao":"Coisa rara","valor":"10","dia":1}`)); !strings.Contains(out, "Mercado") {
		t.Errorf("sem pista: %s", out)
	}
}

// No começo do zero, quando a pessoa manda valores, o primeiro passo obriga a IA a chamar
// a ferramenta (o modelo grátis dizia "conta criada" sem criar cartão nenhum). O passo
// seguinte, já com o resultado, volta ao normal; sem números na mensagem, não obriga.
func TestOnboardingRequiresToolWithValues(t *testing.T) {
	start := []llm.Message{{Role: llm.RoleUser, Content: finance.StartQuestion}, {Role: llm.RoleAssistant, Content: "Quanto você recebe?"}}
	call := llm.ToolCall{ID: "c1", Name: "criar_conta_fixa", Args: json.RawMessage(`{"descricao":"Salário","valor":"4500","tipo":"entrada","dia":5}`)}
	fake := &scriptedLLM{replies: []llm.Reply{{ToolCalls: []llm.ToolCall{call}}, {Text: "Pronto, é só confirmar."}}}
	tools := NewTools(domain.Ledger{Categories: domain.DefaultCategories(1)}, teamDay, func(Proposal) {})
	if _, err := Ask(context.Background(), fake, "", start, "Recebo 4.500 no dia 5", "", tools, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if fake.asked[0].ToolChoice != "required" || fake.asked[1].ToolChoice != "" {
		t.Errorf("tool_choice = %q, depois %q", fake.asked[0].ToolChoice, fake.asked[1].ToolChoice)
	}
	// Dois itens na mesma mensagem: o resultado lembra a IA de criar o cartão do que falta.
	if result := fake.asked[1].Messages[len(fake.asked[1].Messages)-1].Content; !strings.Contains(result, "outro item com valor") {
		t.Errorf("sem o lembrete dos outros itens: %q", result)
	}
	for _, q := range []string{"Não tenho dívidas", "e agora?"} {
		fake = &scriptedLLM{replies: []llm.Reply{{Text: "Ok."}}}
		Ask(context.Background(), fake, "", start, q, "", NewTools(domain.Ledger{}, teamDay, nil), func(string) {})
		if fake.asked[0].ToolChoice != "" {
			t.Errorf("%q sem números obrigou a ferramenta", q)
		}
	}
	// Fora do começo do zero, nunca obriga.
	fake = &scriptedLLM{replies: []llm.Reply{{Text: "Ok."}}}
	Ask(context.Background(), fake, "", nil, "gastei 50 no mercado?", "", NewTools(domain.Ledger{}, teamDay, nil), func(string) {})
	if fake.asked[0].ToolChoice != "" {
		t.Error("conversa normal obrigou a ferramenta")
	}
}
