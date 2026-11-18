package advisor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/rates"
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
	// Otávio: dívidas, vencimentos ou o mês todo, conforme a pergunta.
	wantAll(t, "otávio (dívidas)", Offline(offlineQ("otavio", "Quanto adianta pagar R$ 200 a mais por mês?")),
		"Cartão → Empréstimo", "livre em 7 meses (maio de 2027)", "livre em 4 meses (fevereiro de 2027)")
	due := Offline(offlineQ("otavio", "O que vence esta semana?"))
	wantAll(t, "otávio (vencimentos)", due, "Próximos 7 dias", "Cartão: parcela mínima R$ 200,00 em 20/10")
	if strings.Contains(due, "Aluguel") {
		t.Errorf("aluguel (dia 25) fica fora da semana:\n%s", due)
	}
	// "Quanto gastei com uber?" sem IA: o total do lugar, não o resumo do mês.
	wantAll(t, "otávio (lugar)", Offline(offlineQ("otavio", "quanto gastei com uber?")), "Gastos com Uber: R$ 20,00 em outubro de 2026")
	month := Offline(offlineQ("otavio", "Como foi meu mês?"))
	wantAll(t, "otávio (mês)", month, "outubro de 2026", "Restaurantes e delivery", "Lia")
	q := offlineQ("otavio", "Qual é o meu maior problema agora?")
	q.Ledger.Debts[0].MinPaymentCents = 5000 // menos que os juros: a dívida cresce
	if got := Offline(q); !strings.Contains(got, "Cartão cresce todo mês") || strings.Contains(got, "pode aprofundar") {
		t.Errorf("dívida crescendo é com o próprio Otávio (sem mandar para outro):\n%s", got)
	}

	wantAll(t, "tito", Offline(offlineQ("tito", "Minhas metas estão no ritmo?")), "Viagem", "por mês")
	if got := Offline(offlineQ("tito", "Minhas metas estão no ritmo?")); strings.Contains(got, "Notebook") {
		t.Errorf("meta atingida não entra:\n%s", got)
	}

	lia := Offline(offlineQ("lia", "Onde estou gastando demais este mês?"))
	if n := strings.Count(lia, "\n- "); n < 1 || n > 3 {
		t.Errorf("lia: quer de 1 a 3 achados, veio %d:\n%s", n, lia)
	}
	wantAll(t, "lia (quanto cabe)", Offline(offlineQ("lia", "Quanto ainda posso gastar?")), "Quanto ainda cabe no mês", "Ainda cabe no mês", "por dia")
}

// O texto sem IA usa os nomes reais (não sai do aparelho) e nunca o apelido.
func TestOfflineUsesRealNames(t *testing.T) {
	q := offlineQ("otavio", "Qual dívida devo pagar primeiro?")
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
		{"otavio", "Qual dívida devo pagar primeiro?", true},
		{"otavio", "  quando eu fico LIVRE das dívidas? ", true},
		{"otavio", "O que vence esta semana?", true},
		{"otavio", "Como foi meu mês?", true},
		{"tito", "Quanto guardar por mês para a reserva?", true},
		{"lia", "Tenho assinatura esquecida?", true},
		{"lia", "Quanto ainda posso gastar?", true},
		{"lia", "Qual dívida devo pagar primeiro?", false}, // pronta é a do Otávio
		{"lia", "Quanto gastei com iFood este ano?", false},
		{"otavio", "Qual dívida devo pagar primeiro? E a moto?", false},
		{"jade", "Como foi meu mês?", true}, // quem saiu do time vira o Otávio
	} {
		if got := Prepared(Question{Agent: c.agent, Text: c.text}); got != c.want {
			t.Errorf("%s %q = %v", c.agent, c.text, got)
		}
	}
	// As sugestões de sempre do Otávio (finance.Suggestions) têm resposta pronta, sem IA.
	for _, q := range append(finance.DefaultSuggestions["otavio"], "Qual é o meu maior problema agora?") {
		if !Prepared(Question{Agent: "otavio", Text: q}) {
			t.Errorf("%q sem resposta pronta", q)
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

// chunkedLLM manda a resposta em pedaços, como o streaming da IA na nuvem.
type chunkedLLM struct{ chunks []string }

func (c chunkedLLM) Chat(_ context.Context, _ llm.Request, onText func(string)) (llm.Reply, error) {
	for _, s := range c.chunks {
		onText(s)
	}
	return llm.Reply{Text: strings.Join(c.chunks, "")}, nil
}

// Com apelidos (IA na nuvem), a resposta também chega aos poucos, já com os nomes.
func TestAnswerStreamsAnonymized(t *testing.T) {
	q := offlineQ("otavio", "e agora, o que faço?")
	q.Anonymize = true
	p := chunkedLLM{[]string{"**Pague a Dív", "ida 1 primeiro.**", " Ela tem os juros", " mais altos."}}
	var pieces []string
	r, err := Answer(context.Background(), p, q, func(s string) { pieces = append(pieces, s) }, func(Proposal) {})
	if err != nil || r.App {
		t.Fatalf("r=%+v err=%v", r, err)
	}
	if len(pieces) < 2 {
		t.Errorf("chegou tudo de uma vez: %q", pieces)
	}
	if got := strings.TrimSpace(strings.Join(pieces, "")); got != r.Text || !strings.Contains(got, "Cartão") || strings.Contains(got, "Dívida 1") {
		t.Errorf("na tela %q, resposta %q", got, r.Text)
	}
}

func TestAnswerPreparedSkipsAI(t *testing.T) {
	p := &countingProvider{text: "da IA"}
	var streamed string
	r, err := Answer(context.Background(), p, offlineQ("otavio", "Qual dívida devo pagar primeiro?"), func(s string) { streamed += s }, func(Proposal) {})
	if err != nil || p.calls != 0 || !r.App || r.AIError != nil {
		t.Fatalf("calls=%d reply=%+v err=%v", p.calls, r, err)
	}
	if strings.Contains(r.Text, OfflineNotice) || !strings.Contains(r.Text, "Cartão → Empréstimo") || streamed != "" {
		t.Errorf("texto = %q streamed = %q", r.Text, streamed)
	}
	// Pergunta livre continua indo para a IA.
	r, _ = Answer(context.Background(), p, offlineQ("otavio", "e a dívida da moto?"), func(string) {}, func(Proposal) {})
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
		q := offlineQ("otavio", "e a dívida da moto?")
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
	if _, err := Answer(ctx, &countingProvider{err: context.Canceled}, offlineQ("otavio", "e a dívida da moto?"), func(string) {}, func(Proposal) {}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelado: err = %v", err)
	}
}

func TestOverLimit(t *testing.T) {
	if !OverLimit(fmt.Errorf("x: %w", &llm.APIError{Status: 429})) || !OverLimit(&llm.APIError{Status: 413}) || OverLimit(&llm.APIError{Status: 500}) || OverLimit(errors.New("429")) {
		t.Error("OverLimit")
	}
}

// Os números do app, mostrados na hora enquanto a IA escreve: as mesmas contas
// da resposta sem IA, sem o aviso de falha; pergunta pronta não tem (o app já responde).
func TestNumbers(t *testing.T) {
	q := offlineQ("otavio", "O que eu faço com o meu dinheiro este mês?")
	got := Numbers(q)
	if got != Offline(q) || strings.Contains(got, OfflineNotice) || !strings.Contains(got, "Resumo de") {
		t.Errorf("números:\n%s", got)
	}
	if got := Numbers(offlineQ("otavio", "Qual dívida devo pagar primeiro?")); got != "" {
		t.Errorf("pergunta pronta não leva números à parte:\n%s", got)
	}
}

// O "Perguntar para ..." dos avisos e do time: as perguntas de vencimento, dívida,
// meta atrasada e gasto fora do normal têm resposta pronta, sem gastar IA.
func TestTeamAsksArePrepared(t *testing.T) {
	l := teamLedger()
	l.Debts[0].DueDay = 17 // vence amanhã
	l.Debts = append(l.Debts, domain.Debt{Meta: domain.Meta{ID: "d3", UpdatedAt: 1}, Name: "Cheque especial", BalanceCents: 500000, MinPaymentCents: 10000, MonthlyRateBP: 800})
	l.Goals = append(l.Goals, domain.Goal{Meta: domain.Meta{ID: "g3", UpdatedAt: 1}, Name: "Sofá", TargetCents: 300000, Deadline: "2026-08", StartDate: "2026-01-01"})
	free := map[string]bool{"debt-due": true, "debt-growing": true, "goal-late": true, "unusual": true}
	seen := map[string]bool{}
	for _, a := range finance.Alerts(l, teamDay) {
		if !free[a.Kind] {
			continue
		}
		seen[a.Kind] = true
		if !Prepared(Question{Agent: a.Agent.ID, Text: a.Ask}) {
			t.Errorf("aviso %s (%s): %q não tem resposta pronta", a.Kind, a.Agent.ID, a.Ask)
		}
	}
	if !seen["debt-due"] || !seen["debt-growing"] || !seen["goal-late"] {
		t.Errorf("o teste precisa dos avisos de vencimento, dívida e meta: %v", seen)
	}
	briefs := finance.Briefings(l, teamDay)
	for _, b := range briefs {
		if !Prepared(Question{Agent: b.Agent.ID, Text: b.Ask}) {
			t.Errorf("resumo de %s: %q não tem resposta pronta", b.Agent.ID, b.Ask)
		}
	}
	if len(briefs) < 2 {
		t.Errorf("o teste precisa dos resumos do time: %+v", briefs)
	}
}

// "Quero juntar X até <mês>" é uma meta nova (não "guardar X por mês"), e
// "guardei X" é um aporte que já aconteceu: o Tito sem IA não confunde.
func TestTitoReadsNewGoalAndSavedMoney(t *testing.T) {
	nov := Question{Ledger: teamLedger(), Today: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC), Agent: "tito"}
	nov.Text = "Quero juntar 3.000 para trocar de carro até julho de 2027"
	got := Offline(nov)
	wantAll(t, "meta nova", got, "R$ 3.000,00 até julho de 2027", "R$ 333,34 por mês", "9 meses")
	if strings.Contains(got, "Guardando R$ 3.000,00 por mês") {
		t.Errorf("leu R$ 3.000 como valor por mês:\n%s", got)
	}
	nov.Text = "Guardei 200 na viagem"
	got = Offline(nov)
	wantAll(t, "aporte", got, "Registrar aporte")
	if strings.Contains(got, "Guardando R$ 200,00 por mês") {
		t.Errorf("leu o aporte como valor por mês:\n%s", got)
	}
}

// "Paguei X na dívida" sem IA: o Otávio aponta onde registrar e mostra o plano.
func TestOtavioReadsPayment(t *testing.T) {
	got := Offline(offlineQ("otavio", "Paguei 200 reais no Cartão hoje"))
	wantAll(t, "pagamento", got, "Registrar pagamento", "Cartão → Empréstimo")
}

// Sem renda em nenhum mês, a Lia pede a entrada antes de dizer quanto cabe
// (no navegador esse caso só aparece até o dia 4, antes do salário do guia).
func TestLiaAllowanceWithoutIncome(t *testing.T) {
	q := Question{Ledger: domain.Ledger{Categories: []domain.Category{{Meta: domain.Meta{ID: "m", UpdatedAt: 1}, Name: "Mercado", Bucket: domain.BucketNecessidades}},
		Transactions: []domain.Transaction{{Meta: domain.Meta{ID: "t", UpdatedAt: 1}, Kind: domain.KindSaida, Description: "Feira", AmountCents: 5000, CategoryID: "m", Date: "2026-10-03"}}},
		Today: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), Agent: "lia", Text: "Quanto ainda posso gastar?"}
	wantAll(t, "lia sem renda", Offline(q), "Ainda não há renda neste mês", "Lance a entrada")
}

// Taxas do Banco Central: vão nos dados da IA e respondem sozinhas "qual a selic hoje?".
func TestRates(t *testing.T) {
	r := rates.Rates{SelicBP: 1375, CDIBP: 1365, IPCABP: 463, IPCAMonth: "2026-10", Date: "2026-11-17"}
	q := offlineQ("otavio", "qual é a taxa Selic hoje?")
	if Prepared(q) {
		t.Error("sem taxas, a pergunta vai para a IA")
	}
	q.Rates = r
	if !Prepared(q) || Offline(q) != r.Text() {
		t.Errorf("com taxas, o app responde: %v %q", Prepared(q), Offline(q))
	}
	for _, text := range []string{"quanto está o CDI?", "qual a inflação?", "Qual o IPCA agora?"} {
		if q.Text = text; !Prepared(q) {
			t.Errorf("%q sem resposta pronta", text)
		}
	}
	for _, text := range []string{"vale a pena investir no CDI com a selic assim?", "qual a selic do meu banco?"} {
		if q.Text = text; Prepared(q) {
			t.Errorf("%q é pedido de conselho: vai para a IA", text)
		}
	}
	f := &fakeProvider{}
	q.Text, q.Anonymize = "vale a pena o tesouro selic?", true
	if _, err := Answer(context.Background(), f, q, func(string) {}, func(Proposal) {}); err != nil {
		t.Fatal(err)
	}
	if last := f.got.Messages[len(f.got.Messages)-1].Content; !strings.Contains(last, "Selic 13,75% ao ano") {
		t.Errorf("as taxas não foram nos dados:\n%s", last)
	}
	if !strings.Contains(SystemPrompt, "Taxas do Banco Central") {
		t.Error("as instruções não falam das taxas de hoje")
	}
}
