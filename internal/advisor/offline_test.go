package advisor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func offlineQ(agent, text string) Question {
	return Question{Ledger: teamLedger(), Today: teamDay, Agent: agent, Text: text}
}

func wantAll(t *testing.T, who, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("%s: sem %q:\n%s", who, w, got)
		}
	}
}

// Cada personagem responde sem IA com as contas do Go, do jeito dele.
func TestOfflineByPersona(t *testing.T) {
	wantAll(t, "bento", Offline(offlineQ("bento", "Quanto adianta pagar R$ 200 a mais por mês?")),
		"Cartão → Empréstimo", "livre em 7 meses (maio de 2027)", "livre em 4 meses (fevereiro de 2027)")
	wantAll(t, "tito", Offline(offlineQ("tito", "Minhas metas estão no ritmo?")),
		"Viagem", "por mês")
	if got := Offline(offlineQ("tito", "Minhas metas estão no ritmo?")); strings.Contains(got, "Notebook") {
		t.Errorf("meta atingida não entra:\n%s", got)
	}
	luna := Offline(offlineQ("luna", "O que vence esta semana?"))
	wantAll(t, "luna", luna, "Próximos 7 dias", "Cartão: parcela mínima R$ 200,00 em 20/10")
	if strings.Contains(luna, "Aluguel") {
		t.Errorf("aluguel (dia 25) fica fora da semana:\n%s", luna)
	}
	otavio := Offline(offlineQ("otavio", "Onde estou gastando demais este mês?"))
	if n := strings.Count(otavio, "\n- "); n < 1 || n > 3 {
		t.Errorf("otávio: quer de 1 a 3 achados, veio %d:\n%s", n, otavio)
	}
	// Jade: resumo do mês, o maior problema e o colega certo.
	jade := Offline(offlineQ("jade", "Qual é o meu maior problema agora?"))
	wantAll(t, "jade", jade, "outubro de 2026", "Restaurantes e delivery", "Otávio")
	q := offlineQ("jade", "Qual é o meu maior problema agora?")
	q.Ledger.Debts[0].MinPaymentCents = 5000 // menos que os juros: a dívida cresce
	wantAll(t, "jade com dívida crescendo", Offline(q), "Cartão cresce todo mês", "Bento (Dívidas)")
}

// O texto sem IA usa os nomes reais (não sai do aparelho) e nunca o apelido.
func TestOfflineUsesRealNames(t *testing.T) {
	q := offlineQ("bento", "Qual dívida devo pagar primeiro?")
	q.Anonymize = true
	if got := Offline(q); !strings.Contains(got, "Cartão") || strings.Contains(got, "Dívida 1") {
		t.Errorf("nomes:\n%s", got)
	}
}

func TestPrepared(t *testing.T) {
	for _, c := range []struct {
		agent, text string
		want        bool
	}{
		{"bento", "Qual dívida devo pagar primeiro?", true},
		{"bento", "  quando eu fico LIVRE das dívidas? ", true},
		{"luna", "O que vence esta semana?", true},
		{"tito", "Quanto guardar por mês para a reserva?", true},
		{"otavio", "Tenho assinatura esquecida?", true},
		{"jade", "Qual é o meu maior problema agora?", true},
		{"jade", "Qual dívida devo pagar primeiro?", false}, // pronta é a do Bento
		{"otavio", "Quanto gastei com iFood este ano?", false},
		{"bento", "Qual dívida devo pagar primeiro? E a moto?", false},
	} {
		if got := Prepared(Question{Agent: c.agent, Text: c.text}); got != c.want {
			t.Errorf("%s %q = %v", c.agent, c.text, got)
		}
	}
	for agent, list := range Suggestions {
		if len(list) == 0 {
			t.Errorf("%s sem sugestões", agent)
		}
	}
}

type countingProvider struct {
	calls int
	text  string
	err   error
}

func (c *countingProvider) Chat(_ context.Context, _ llm.Request, onText func(string)) (llm.Reply, error) {
	c.calls++
	if c.err != nil {
		return llm.Reply{}, c.err
	}
	onText(c.text)
	return llm.Reply{Text: c.text}, nil
}

func TestAnswerPreparedSkipsAI(t *testing.T) {
	p := &countingProvider{text: "da IA"}
	var streamed string
	r, err := Answer(context.Background(), p, offlineQ("bento", "Qual dívida devo pagar primeiro?"), func(s string) { streamed += s }, func(Proposal) {})
	if err != nil || p.calls != 0 || !r.App || r.AIError != nil {
		t.Fatalf("calls=%d reply=%+v err=%v", p.calls, r, err)
	}
	if strings.Contains(r.Text, OfflineNotice) || !strings.Contains(r.Text, "Cartão → Empréstimo") || streamed != "" {
		t.Errorf("texto = %q streamed = %q", r.Text, streamed)
	}
	// Pergunta livre continua indo para a IA.
	r, _ = Answer(context.Background(), p, offlineQ("bento", "e a moto?"), func(string) {}, func(Proposal) {})
	if p.calls != 1 || r.App || r.Text != "da IA" {
		t.Errorf("calls=%d reply=%+v", p.calls, r)
	}
}

func TestAnswerFallsBackWhenAIFails(t *testing.T) {
	for _, fail := range []*countingProvider{
		{err: &llm.APIError{Status: 429, Message: "limite"}},
		{err: errors.New("dial tcp: no such host")},
		{text: ""}, // modelo sem resposta
	} {
		q := offlineQ("bento", "e a moto?")
		q.Anonymize = true
		r, err := Answer(context.Background(), fail, q, func(string) {}, func(Proposal) {})
		if err != nil || !r.App || r.AIError == nil {
			t.Fatalf("%v: reply=%+v err=%v", fail.err, r, err)
		}
		if !strings.HasPrefix(r.Text, OfflineNotice) || !strings.Contains(r.Text, "Cartão → Empréstimo") {
			t.Errorf("texto:\n%s", r.Text)
		}
	}
	// A pessoa saiu da tela: nada de resposta.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Answer(ctx, &countingProvider{err: context.Canceled}, offlineQ("bento", "e a moto?"), func(string) {}, func(Proposal) {}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelado: err = %v", err)
	}
}

func TestOverLimit(t *testing.T) {
	if !OverLimit(fmt.Errorf("x: %w", &llm.APIError{Status: 429})) || OverLimit(&llm.APIError{Status: 500}) || OverLimit(errors.New("429")) {
		t.Error("OverLimit")
	}
}
