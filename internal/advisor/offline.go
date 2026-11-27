package advisor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Resposta sem IA: quando a IA não responde (cota, rede, provedor fora) ou
// quando a pergunta é uma das sugestões que só precisam de contas, o
// personagem responde com o que o Go calculou, num texto fixo do jeito dele.
// Nunca há número inventado aqui: tudo vem das mesmas contas das telas.
// Usa o livro-caixa original (nomes reais), porque este texto não sai do aparelho.

// OfflineNotice abre a resposta sem IA quando a IA falhou.
const OfflineNotice = "**A IA não respondeu agora.** Estes são os números do app:"

// Reply é a resposta do consultor. App: o texto é do app, sem IA (pergunta
// pronta ou a IA falhou); AIError diz por que a IA falhou (nil se não falhou).
type Reply struct {
	Text      string
	Proposals []Proposal
	App       bool
	AIError   error
}

// prepared: as perguntas que o Go responde sozinho, por personagem (as
// sugestões e as dos avisos e do time no Painel). As outras vão para a IA.
var prepared = map[string][]string{
	"otavio": {"Como foi meu mês?", "Qual é o meu maior problema agora?", "Qual dívida devo pagar primeiro?", "Quando eu fico livre das dívidas?",
		"Quanto adianta pagar R$ 200 a mais por mês?", "O que vence esta semana?"},
	"lia":  {"Onde estou gastando demais este mês?", "Quanto ainda posso gastar?", "Tenho assinatura esquecida?"},
	"tito": {"Quanto guardar por mês para a reserva?", "Minhas metas estão no ritmo?"},
}

// rateQuestionRe: "qual a selic hoje?", "quanto está o CDI?" (já com fold). Pedido de
// conselho ("vale a pena...") não entra: vai para a IA, com as taxas nos dados.
var rateQuestionRe = regexp.MustCompile(`^(qual|quanto) (e |esta |ta )?(a |o )?(taxa )?(da |do )?(selic|cdi|ipca|inflacao)( (de )?(hoje|agora|atual|do dia))? ?\??$`)

// rateAnswer: a pergunta é só a taxa do dia e o app tem as taxas.
func rateAnswer(q Question) bool {
	return q.Rates.Date != "" && rateQuestionRe.MatchString(fold(q.Text))
}

// Prepared diz se a pergunta tem resposta pronta do app (sem IA): pergunta de número
// (app_answers.go), as taxas do dia ou uma sugestão deste personagem. Com AskAI, não tem.
func Prepared(q Question) bool {
	if q.AskAI {
		return false
	}
	if rateAnswer(q) || appAnswer(q) != "" {
		return true
	}
	text := fold(q.Text)
	for _, p := range prepared[persona(q.Agent).ID] {
		if fold(p) == text {
			return true
		}
	}
	return false
}

// OverLimit diz se a IA recusou por limite de uso: 429, ou 413 do Groq (o
// pedido passou da cota por minuto do modelo).
func OverLimit(err error) bool {
	var apiErr *llm.APIError
	return errors.As(err, &apiErr) && (apiErr.Status == 429 || apiErr.Status == 413)
}

// Fallback é a resposta quando a IA falhou com err: o aviso e as contas do
// personagem (ou a resposta pronta, se a pergunta tiver uma).
func Fallback(q Question, err error) Reply {
	if Prepared(q) {
		return Reply{Text: Offline(q), App: true}
	}
	return Reply{Text: OfflineNotice + "\n\n" + Offline(q), App: true, AIError: err}
}

// answerOrFallback trata o fim de Answer: cancelado é erro; o resto vira resposta sem IA.
func answerOrFallback(ctx context.Context, q Question, err error) (Reply, error) {
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return Reply{}, err
	}
	return Fallback(q, err), nil
}

// Numbers são as contas do personagem para mostrar na hora, enquanto a IA
// escreve (a IA leva de segundos a quase um minuto). Vazio para pergunta
// pronta (nela o app já responde e a IA nem é chamada) e no "Perguntar à IA"
// (a resposta do app já está logo acima, na conversa).
func Numbers(q Question) string {
	if q.AskAI || Prepared(q) {
		return ""
	}
	return Offline(q)
}

// Offline é a resposta do personagem só com as contas do app.
func Offline(q Question) string {
	if rateAnswer(q) {
		return q.Rates.Text()
	}
	if s := appAnswer(q); s != "" {
		return s
	}
	if fold(q.Text) == fold(finance.StartQuestion) {
		return "Para começar sem IA, cadastre à mão:\n- a renda e as contas do mês em \"Novo lançamento\", com \"Repetir todo mês\" marcado;\n- as dívidas na tela Dívidas;\n- as metas na tela Metas."
	}
	t := NewTools(q.Ledger, q.Today, nil)
	switch id := persona(q.Agent).ID; id {
	case "otavio":
		switch otavioTopic(q.Text) {
		case "dividas":
			sim := prefetch(id, t, q.Text)
			if sim == "" { // cadastro ou pagamento já feito: o plano sem ler o valor como "a mais por mês"
				sim = t.simulateDebts("", "")
			}
			plan := "**Plano das dívidas**\n\n" + bullets(sim)
			if strings.Contains(fold(q.Text), "paguei") { // o pagamento já aconteceu: falta registrar
				return "Para baixar o saldo, use \"Registrar pagamento\" no cartão da dívida, na tela Dívidas.\n\n" + plan
			}
			return plan
		case "vencimentos":
			return bullets(prefetch(id, t, q.Text))
		}
		if s := t.placeSpending(fold(q.Text)); s != "" {
			return s
		}
		return overviewOffline(q)
	case "tito":
		return titoOffline(t, q)
	case "lia":
		if s := t.placeSpending(fold(q.Text)); s != "" {
			return s
		}
		if f := fold(q.Text); strings.Contains(f, "posso gastar") || strings.Contains(f, "ainda cabe") {
			return liaAllowance(q)
		}
		return liaOffline(q)
	}
	return overviewOffline(q)
}

// payRe: pagar, pagarei, paguei... como palavra; "apagar" não é pagamento.
var payRe = regexp.MustCompile(`(^|[^a-z])pag(ar|uei)`)

// otavioTopic: do que a pergunta ao Otávio trata ("" é o mês como um todo).
func otavioTopic(text string) string {
	f := fold(text)
	for _, w := range []string{"vence", "vencimento", "fatura", "boleto"} {
		if strings.Contains(f, w) {
			return "vencimentos"
		}
	}
	if finance.Route(text) == "otavio" || payRe.MatchString(f) || strings.Contains(f, "a mais") {
		return "dividas"
	}
	return ""
}

// bullets põe cada linha da conta pronta como item de lista (cabeçalhos com ":" ficam como estão).
func bullets(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "- ") || strings.HasSuffix(line, ":"):
			out = append(out, line)
		default:
			out = append(out, "- "+line)
		}
	}
	return strings.Join(out, "\n")
}

func titoOffline(t *Tools, q Question) string {
	goals := prefetch("tito", t, q.Text)
	var parts []string
	if goals != "" {
		parts = append(parts, "**Suas metas**\n\n"+bullets(goals))
	}
	if r := reserveText(q); r != "" && (goals == "" || strings.Contains(fold(q.Text), "reserva")) {
		parts = append(parts, r)
	}
	if len(parts) == 0 {
		return "Nenhuma meta em andamento. Crie uma na tela Metas e eu digo quanto guardar por mês."
	}
	if savedRe.MatchString(fold(q.Text)) && questionMoney(q.Text) != "" { // "guardei 200": falta registrar
		parts = append([]string{"Para somar esse dinheiro na meta, use \"Registrar aporte\" no cartão dela, na tela Metas."}, parts...)
	}
	return strings.Join(parts, "\n\n")
}

// reserveText: a reserva de emergência pelas contas do app ("" sem gastos de meses
// fechados para medir).
func reserveText(q Question) string {
	lines := reserveLines(q)
	if len(lines) == 0 {
		return ""
	}
	return "**Reserva de emergência**\n\n- " + strings.Join(lines, "\n- ")
}

// reserveLines: o tamanho da reserva e quando chega nela guardando a sobra (nil sem meses fechados).
func reserveLines(q Question) []string {
	e := BuildFacts(q.Ledger, q.Today).Emergency
	if e.AvgMonthlyOutCents == 0 {
		return nil
	}
	lines := []string{fmt.Sprintf("Mínima: %s (3 meses de gastos); ideal: %s (6 meses).", money.Format(e.MinCents), money.Format(e.IdealCents))}
	if e.MonthsToMin > 0 {
		return append(lines, fmt.Sprintf("Guardando a sobra livre (%s por mês), chega na mínima em %s e na ideal em %s.", money.Format(e.MonthlySavingCents), finance.Meses(e.MonthsToMin), finance.Meses(e.MonthsToIdeal)))
	}
	return append(lines, "Hoje não sobra dinheiro no mês: sem cortar gastos, não dá para formar a reserva.")
}

// subscriptionsText: o que é cobrado todo mês, com o custo no ano; as assinaturas
// (desejos) primeiro, depois as contas fixas, como na ferramenta "achados".
func subscriptionsText(q Question) string {
	var subs, fixed []string
	for _, s := range finance.Insights(q.Ledger, q.Today).Subscriptions {
		cost := money.Format(s.MonthlyCents) + " por mês, " + money.Format(s.YearlyCents) + " por ano."
		if s.Bucket == domain.BucketDesejos {
			subs = append(subs, s.Name+" (assinatura): "+cost)
		} else {
			fixed = append(fixed, s.Name+" (conta fixa): "+cost)
		}
	}
	lines := append(subs, fixed...)
	if len(lines) == 0 {
		return "Não achei nada cobrado todo mês nos seus lançamentos."
	}
	return "**Cobranças todo mês**\n\n" + bullets(strings.Join(lines, "\n"))
}

// maxFindings: a Lia traz até 3 achados.
const maxFindings = 3

func liaOffline(q Question) string {
	if strings.Contains(fold(q.Text), "assinatura") {
		return subscriptionsText(q)
	}
	in := finance.Insights(q.Ledger, q.Today)
	var lines []string
	if in.Day > 0 {
		lines = append(lines, fmt.Sprintf("Até o dia %d: %s em gastos variáveis; com as contas fixas, a projeção do mês é %s (mês passado: %s).",
			in.Day, money.Format(in.VariableCents), money.Format(in.ProjectedCents), money.Format(in.LastMonthCents)))
	}
	for _, b := range finance.Briefings(q.Ledger, q.Today) {
		if b.Agent.ID == "lia" {
			lines = append(lines, b.Lines...)
		}
	}
	if len(lines) > maxFindings {
		lines = lines[:maxFindings]
	}
	if len(lines) == 0 {
		return "Ainda não há gastos neste mês para comparar. Lance os gastos e eu mostro o que mudou."
	}
	return "**Achados do mês**\n\n" + bullets(strings.Join(lines, "\n"))
}

// liaAllowance: quanto ainda cabe no mês (sem estourar a renda) e nos orçamentos. Com dívida,
// cartão em aberto ou sem renda registrada no mês, a sobra não vira limite de gasto: vale o
// motivo do Início (finance.Home), e só os orçamentos aparecem.
func liaAllowance(q Question) string {
	in := finance.Insights(q.Ledger, q.Today)
	if in.IncomeCents == 0 {
		return "Ainda não há renda neste mês nem nos anteriores. Lance a entrada do mês e eu digo quanto ainda cabe."
	}
	head := "**Quanto ainda cabe no mês**"
	var lines []string
	switch home := finance.Home(q.Ledger, q.Today); {
	case !home.EstimateAvailable:
		head = "**Sem estimativa por enquanto.** " + home.EstimateReason
	case in.FreeCents >= 0:
		lines = append(lines, fmt.Sprintf("Ainda cabe no mês: %s (renda de %s), pela estimativa com os registros do app, não o saldo da conta.", money.Format(in.FreeCents), money.Format(in.IncomeCents)))
		if in.PerDayCents > 0 {
			lines = append(lines, fmt.Sprintf("Dá uns %s por dia até o fim do mês.", money.Format(in.PerDayCents)))
		}
	default:
		lines = append(lines, fmt.Sprintf("O mês já passou da renda (%s) em %s, contando os fixos que ainda vão cair.", money.Format(in.IncomeCents), money.Format(-in.FreeCents)))
	}
	budgets := slices.Clone(in.Budgets)
	slices.SortStableFunc(budgets, func(a, b finance.BudgetView) int { return b.UsedPct - a.UsedPct })
	for i, b := range budgets {
		if i == maxFindings {
			break
		}
		if b.RemainingCents > 0 {
			lines = append(lines, fmt.Sprintf("%s: restam %s de %s (%d%% usado).", b.Name, money.Format(b.RemainingCents), money.Format(b.BudgetCents), b.UsedPct))
		} else {
			lines = append(lines, fmt.Sprintf("%s: o orçamento de %s já acabou (%d%% usado).", b.Name, money.Format(b.BudgetCents), b.UsedPct))
		}
	}
	if len(lines) == 0 {
		return head
	}
	return head + "\n\n" + bullets(strings.Join(lines, "\n"))
}

// overviewOffline: o mês como um todo e o maior problema agora (Otávio).
func overviewOffline(q Question) string {
	f := BuildFacts(q.Ledger, q.Today)
	sum := f.Month
	var b strings.Builder
	fmt.Fprintf(&b, "**Resumo de %s:** entrou %s, saiu %s, sobra %s.", sum.Month.Label(),
		money.Format(sum.InCents), money.Format(sum.OutCents), money.Format(sum.BalanceCents()))

	// O maior problema: o que o retrato já marca como atenção, depois os avisos, depois a dívida mais cara.
	problem, who := "", ""
	if points := attentionPoints(f); len(points) > 0 {
		problem = points[0]
	} else {
		for _, a := range f.Alerts {
			if a.Level == "danger" || a.Level == "warn" {
				problem, who = a.Title+": "+a.Text, a.Agent.ID
				break
			}
		}
	}
	if c := f.Diagnosis.Costliest; problem == "" && c != nil {
		problem = fmt.Sprintf("a dívida mais cara é %s, com %s de juros por mês (%s ao mês)", c.Name, money.Format(c.MonthlyInterestCents), RatePct(c.MonthlyRateBP))
	}
	if problem == "" {
		b.WriteString("\n\nNada urgente agora: nenhuma conta no vermelho e nenhuma dívida crescendo.")
		return b.String()
	}
	fmt.Fprintf(&b, "\n\n**Maior problema agora:** %s.", strings.TrimSuffix(problem, "."))

	// Quem do time aprofunda: o assunto da pergunta, senão o do problema.
	for _, id := range []string{finance.Route(q.Text), finance.Route(problem), who} {
		if id != "" && id != "otavio" && id != persona(q.Agent).ID { // quem responde não manda escolher a si mesmo
			a := member(id)
			fmt.Fprintf(&b, "\n\n%s (%s) pode aprofundar: escolha %s no time.", a.Name, a.Role, a.Name)
			break
		}
	}
	return b.String()
}

func member(id string) finance.Agent {
	for _, a := range finance.Team {
		if a.ID == id {
			return a
		}
	}
	return finance.Team[0]
}
