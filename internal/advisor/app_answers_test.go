package advisor

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// Perguntas de número: o app responde sozinho, com a conta certa e para qualquer
// pessoa do time.
func TestAppAnswers(t *testing.T) {
	in := finance.Insights(teamLedger(), teamDay)
	if in.FreeCents < 30000 || in.FreeCents >= 500000 {
		t.Fatalf("o teste conta com R$ 300 cabendo no mês e R$ 5.000 não: cabe %d", in.FreeCents)
	}
	for _, c := range []struct {
		text string
		want []string
	}{
		{"quanto gastei com uber?", []string{"Gastos com Uber: R$ 20,00 em outubro de 2026"}},
		{"Quanto gastei com iFood em setembro?", []string{"R$ 30,00 em setembro de 2026"}},
		{"quanto gastei esse mês?", []string{"**Saídas de outubro de 2026: R$ 420,00.**", "Restaurantes e delivery"}},
		{"quanto ainda posso gastar?", []string{"Quanto ainda cabe no mês"}},
		{"Quanto sobra no mês?", []string{"Quanto ainda cabe no mês"}},
		{"posso gastar 300 reais no fim de semana?", []string{"**Cabe no mês.**", "depois dos R$ 300,00", "por dia", "Lembrete: a dívida Cartão"}},
		{"consigo comprar uma TV de 5 mil?", []string{"**Não cabe no mês sem passar da renda.**", "para os R$ 5.000,00"}},
		{"quando fico livre do cartão?", []string{"**Plano das dívidas**", "Cartão → Empréstimo", "livre em 7 meses (maio de 2027)"}},
		{"e se eu pagar 200 a mais?", []string{"Com R$ 200,00 a mais: livre em 4 meses (fevereiro de 2027)"}},
		{"Quanto adianta pagar R$ 200 a mais por mês?", []string{"Com R$ 200,00 a mais"}},
		{"qual dívida pago primeiro?", []string{"Ordem (maior juros primeiro): Cartão → Empréstimo"}},
		{"quanto devo no total?", []string{"**Você deve R$ 2.000,00** em 2 dívidas, com R$ 120,00 de juros por mês."}},
		{"qual o juros do meu cartão?", []string{"Cartão: R$ 1.000,00, juros de 10,00% ao mês (R$ 100,00 por mês), mínimo R$ 200,00, vence dia 20."}},
		{"o que vence essa semana?", []string{"Próximos 7 dias", "Cartão: parcela mínima R$ 200,00 em 20/10"}},
		{"o que tenho pra pagar esse mês?", []string{"Próximos 31 dias", "Aluguel"}},
		{"quanto preciso guardar por mês para a viagem?", []string{"Para chegar até março de 2027: guarde R$ 1.000,00 por mês durante 6 meses"}},
		{"se eu guardar 400 por mês, quando chego na viagem?", []string{"Guardando R$ 400,00 por mês, chega em 15 meses (dezembro de 2027)"}},
		{"minha meta está atrasada?", []string{"Viagem"}},
		{"quanto guardar por mês para a reserva?", []string{"**Reserva de emergência**", "Mínima:"}},
		{"tenho assinatura esquecida?", []string{"**Cobranças todo mês**", "Aluguel (conta fixa): R$ 1.450,00 por mês, R$ 17.400,00 por ano."}},
		{"quando fico livre das dívidas?", []string{"livre em 7 meses"}}, // o mínimo cobre os juros: sem o aviso abaixo
		{"gastei mais que no mês passado?", []string{"a mais que no mês passado.**", "no mesmo período de setembro de 2026"}},
		{"o que mudou do mês passado para este?", []string{"Restaurantes e delivery"}},
		{"em que categoria gasto mais?", []string{"**Restaurantes e delivery: R$ 400,00 em outubro de 2026**"}},
		{"qual meu saldo do mês?", []string{"Resumo de outubro de 2026"}},
		// saudação, nome do time e "me diz" no começo não mudam a pergunta
		{"Oi Lia, quanto ainda posso gastar?", []string{"Quanto ainda cabe no mês"}},
		{"Otávio, quando fico livre do cartão?", []string{"livre em 7 meses"}},
		{"me diz quanto gastei com uber", []string{"Gastos com Uber: R$ 20,00"}},
		{"quero saber se posso gastar 300 reais", []string{"**Cabe no mês.**"}},
		{"gastos com uber esse mês?", []string{"Gastos com Uber: R$ 20,00"}},
	} {
		for _, agent := range []string{"otavio", "lia", "tito"} {
			q := Question{Ledger: teamLedger(), Today: teamDay, Agent: agent, Text: c.text}
			got := appAnswer(q)
			wantAll(t, agent+" "+c.text, got, c.want...)
			if !Prepared(q) || Offline(q) != got || Numbers(q) != "" {
				t.Errorf("%s %q: a resposta do app vale como pronta (sem IA e sem números à parte)", agent, c.text)
			}
		}
	}
}

// Na dúvida, a pergunta vai para a IA: conselho, ação, duas perguntas, período que a
// conta não cobre, lugar que o app não conhece, conversa.
func TestAppAnswersLeaveTheRestToAI(t *testing.T) {
	for _, text := range []string{
		"o que eu faço com meu dinheiro?", "onde posso economizar?", "recebi o 13º de R$ 4.500, o que faço com ele?",
		"me conta uma piada", "quero financiar um carro de R$ 60 mil, o que acha?", "como saio do vermelho?", "Como reduzir meus gastos?", "Como chego na minha meta mais rápido?",
		"por que gastei mais com delivery este mês?", "devo cancelar a netflix?", "Me ajuda a começar do zero?",
		// ação: a IA prepara o cartão
		"gastei 50 no mercado", "paguei 200 no cartão", "eu já guardei 200 na viagem", "lança 30 de cinema", "cria uma meta de viagem",
		// duas perguntas, ou uma longa
		"quanto gastei com uber e o que faço para gastar menos?", "Qual dívida devo pagar primeiro? E a moto?",
		"quando fico livre das dívidas e quanto pago de juros?",
		"quanto gastei com uber esse mês, considerando também as corridas que dividi com meus amigos da faculdade e do trabalho?",
		// período ou valor que a conta daqui não cobre
		"quanto gastei com iFood este ano?", "quanto gasto em média com uber?", "quanto gastei com uber semana passada?", "quanto gastei com uber e 99?",
		"posso comprar um celular em 10x de 200?", "e se eu pagar mais?", "quanto devo pagar por mês para quitar o cartão?",
		// o app não sabe do que se trata
		"quanto gastei com academia?", "e a dívida da moto?", "e no mês passado?", "quanto tenho de reserva?", "tenho conta atrasada?",
		"quanto falta para quitar o cartão?", "qual a taxa selic hoje?", "oi", "", "Oi Lia, tudo bem?", "me diz uma coisa: vale a pena o CDB?",
		// a conta daqui responderia outra coisa: outro mês, outro tipo de dívida, escolha, tendência
		"o que mudou de agosto para outubro?", "qual o saldo do mês passado?", "qual a taxa de juros do cheque especial?",
		"posso pagar a fatura de 2 mil?", "qual conta pago primeiro, luz ou água?", "qual gasto mais cresceu?",
		"o que vence no mês que vem?", "em que categoria gasto mais por ano?",
	} {
		q := Question{Ledger: teamLedger(), Today: teamDay, Agent: "otavio", Text: text}
		if got := appAnswer(q); got != "" {
			t.Errorf("%q vai para a IA, mas o app respondeu:\n%s", text, got)
		}
	}
}

// "Perguntar à IA": a mesma pergunta vai para a IA, sem os números à parte (a resposta
// do app está logo acima); se a IA falhar, volta a resposta do app com o aviso.
func TestAskAISkipsAppAnswer(t *testing.T) {
	q := offlineQ("lia", "quanto gastei com uber?")
	q.AskAI = true
	if Prepared(q) || Numbers(q) != "" {
		t.Fatal("com AskAI, a pergunta vai para a IA")
	}
	p := &countingProvider{text: "da IA"}
	if r, err := Answer(context.Background(), p, q, func(string) {}, func(Proposal) {}); err != nil || p.calls != 1 || r.App || r.Text != "da IA" {
		t.Errorf("calls=%d reply=%+v err=%v", p.calls, r, err)
	}
	r, err := Answer(context.Background(), &countingProvider{err: context.DeadlineExceeded}, q, func(string) {}, func(Proposal) {})
	if err != nil || !r.App || r.AIError == nil || !strings.HasPrefix(r.Text, OfflineNotice) || !strings.Contains(r.Text, "Gastos com Uber: R$ 20,00") {
		t.Errorf("IA falhou: reply=%+v err=%v", r, err)
	}
}

// No começo do zero, as respostas da pessoa ("não tenho dívidas") são conversa com a IA.
func TestAppAnswersWaitDuringOnboarding(t *testing.T) {
	q := offlineQ("otavio", "quanto devo no total?")
	q.History = []llm.Message{{Role: llm.RoleUser, Content: finance.StartQuestion}, {Role: llm.RoleAssistant, Content: "Quanto você recebe por mês?"}}
	if appAnswer(q) != "" {
		t.Error("no começo do zero, tudo vai para a IA")
	}
}

// "2 mil", "1,5 mil" e "2k" são milhares nas contas prontas; "1 milhão" também conta.
func TestQuestionMoneyThousands(t *testing.T) {
	for q, want := range map[string]string{
		"consigo comprar um celular de 2 mil?": "2000,00", "posso gastar 1,5 mil?": "1500,00", "e um de 2k?": "2000,00",
		"uma casa de 1 milhão": "1000000,00", "R$ 300 no fim de semana": "300", "em 10x de 200": "200",
	} {
		if got := questionMoney(q); got != want {
			t.Errorf("questionMoney(%q) = %q, quer %q", q, got, want)
		}
	}
}

// As perguntas da avaliação com IA que o app agora responde sozinho passam nos mesmos
// critérios, sem IA (o TestEval manda direto para a IA, com AskAI).
func TestAppAnswersMeetEvalCriteria(t *testing.T) {
	today := time.Date(2026, 10, 22, 12, 0, 0, 0, time.Local)
	type agentCase struct {
		agent string
		evalCase
	}
	var all []agentCase
	for _, c := range evalCases {
		all = append(all, agentCase{"", c})
	}
	for _, c := range teamEvalCases {
		all = append(all, agentCase{c.agent, c.evalCase})
	}
	answered := 0
	for _, c := range all {
		got := appAnswer(Question{Text: c.q, Ledger: evalLedger(), Today: today, Agent: c.agent})
		if got == "" {
			continue
		}
		answered++
		for _, re := range c.must {
			if !regexp.MustCompile(`(?i)` + re).MatchString(got) {
				t.Errorf("%q: faltou /%s/:\n%s", c.q, re, got)
			}
		}
		for _, re := range c.never {
			if regexp.MustCompile(`(?i)` + re).MatchString(got) {
				t.Errorf("%q: não podia /%s/:\n%s", c.q, re, got)
			}
		}
		if n := len(strings.Fields(got)); direct.MatchString(c.q) && n > 90 {
			t.Errorf("%q: pergunta direta com %d palavras", c.q, n)
		}
	}
	// uber, R$ 500 a mais, R$ 400 para a viagem, o que mudou, próximos 20 dias; e as explicações
	// (concepts.go): ações, reserva e pagar só o mínimo.
	if answered != 8 {
		t.Errorf("o app respondeu %d perguntas da avaliação, quer 8", answered)
	}
}

// Só no mínimo, a dívida cresce: o plano diz quanto a mais faz ter fim e como perguntar.
func TestAppAnswerDebtThatNeverEnds(t *testing.T) {
	q := offlineQ("otavio", "quando fico livre do cartão?")
	q.Ledger.Debts = q.Ledger.Debts[:1]
	q.Ledger.Debts[0].MinPaymentCents = 5000 // juros de R$ 100 por mês
	wantAll(t, "dívida sem fim", appAnswer(q), "não acaba", "os juros (R$ 100,00 por mês) passam dos mínimos (R$ 50,00)", `"e se eu pagar 100 a mais?"`)
	if got := appAnswer(offlineQ("otavio", "quando fico livre do cartão?")); strings.Contains(got, "Só com o mínimo, não acaba") {
		t.Errorf("o mínimo cobre os juros: sem o aviso\n%s", got)
	}
}

// "Posso gastar 300 com delivery?": além do mês, o orçamento da categoria.
func TestAppAnswerAffordWithBudget(t *testing.T) {
	q := offlineQ("lia", "posso gastar 300 com delivery?")
	for i := range q.Ledger.Categories {
		if q.Ledger.Categories[i].ID == "res" {
			q.Ledger.Categories[i].BudgetCents = 50000 // R$ 400 já gastos em outubro
		}
	}
	wantAll(t, "orçamento", appAnswer(q), "**Cabe no mês.**", "Mas no orçamento de Restaurantes e delivery restam só R$ 100,00 de R$ 500,00.")
	q.Text = "posso gastar 50 com delivery?"
	wantAll(t, "cabe no orçamento", appAnswer(q), "No orçamento de Restaurantes e delivery, restam R$ 100,00 de R$ 500,00; depois, ficam R$ 50,00.")
}

// A resposta do app vale para o time todo, mas quem responde não manda escolher a si mesmo.
func TestAppAnswerNeverPointsToItself(t *testing.T) {
	if got := appAnswer(offlineQ("otavio", "como foi meu mês?")); !strings.Contains(got, "escolha Lia no time") {
		t.Fatalf("o teste precisa do Otávio apontando a Lia:\n%s", got)
	}
	if got := appAnswer(offlineQ("lia", "como foi meu mês?")); got == "" || strings.Contains(got, "escolha Lia") {
		t.Errorf("a Lia responde sem mandar escolher a Lia:\n%s", got)
	}
}
