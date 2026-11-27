package advisor

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Perguntas de número que o app responde sozinho, na hora e sem IA: a cota da IA
// grátis é pequena e dividida por todo mundo, e a conta do app é exata. Cada assunto
// casa uma expressão na pergunta (sem acentos) e responde com as mesmas contas das
// telas e da resposta sem IA, para qualquer pessoa do time. Na dúvida, a pergunta vai
// para a IA. O "Perguntar à IA" da tela manda a mesma pergunta com Question.AskAI.

// maxAppRunes: pergunta mais longa costuma trazer contexto ou nuance; vai para a IA.
const maxAppRunes = 100

var (
	// leadRe: o começo que não muda a pergunta ("oi, Lia,", "me diz", "quero saber se").
	leadRe = regexp.MustCompile(`^((oi|ola|ei|e ai|bom dia|boa tarde|boa noite|otavio|lia|tito|por favor)[,!]? |(me (diz|diga|fala|fale|mostra|mostre|conta|conte)|(voce )?sabe( me)? dizer|quero saber|queria saber|gostaria de saber)( se)?:? )+`)
	// Conselho, decisão ou explicação: é conversa, não conta.
	adviceRe = regexp.MustCompile(`^(devo|deveria|vale)\b|\b(vale a pena|compensa|melhor|pior|por ?que|pq|o que (eu )?(faco|fazer|voce acha)|o que (e|sao|significa)\b|como funciona|como (eu )?(faco|posso|consigo|devo|saio|sair|reduz|economiz|organiz|mont|evit|cort|junt|guard|pag|invist|aplic|comec|recuper|cheg|control)|ajud|dica|conselh|acha\b|acham\b|recomend|suger|sugest|opini|estrateg|explic|invest|cancel)`)
	// Ação (gastei 50, lança, cria, apaga): a IA prepara o cartão para a pessoa confirmar.
	// "gastei mais que no mês passado?" é pergunta: no passado, só vale com um valor.
	actionRe = regexp.MustCompile(`^((ja|eu|hoje|ontem|agora|acabei de) )*(gastei|paguei|recebi|ganhei|guardei|juntei|depositei|comprei|vendi|transferi|emprestei|peguei)\b.*\d|\b(lanc[ae]r?|registr[ae]r?|anot[ae]r?|cri[ae]r?|adicion[ae]r?|cadastr[ae]r?|apag[ae]r?|exclu[ia]r?|remov[ae]r?|corrig[ei]r?|edit[ae]r?|troc[ae]r?|lembr[ae]r?)\b`)
	// Duas perguntas numa só ("quando fico livre e quanto pago de juros?"). "Se eu guardar
	// 400, quando chego?" é uma pergunta só. Escolha ("luz ou água?") também vai para a IA.
	partRe  = regexp.MustCompile(`,|;|:| e | ou `)
	askedRe = regexp.MustCompile(`^\s*(quanto|quanta|qual|quais|quando|como|onde|o que|por ?que|devo|posso|consigo|vale|da pra|da para|tenho|tem)\b`)
	// Gasto num período que estas contas não cobrem: a IA busca nos lançamentos.
	periodRe = regexp.MustCompile(`\b(ano|anos|semana|semanas|hoje|ontem|dia|dias|trimestre|semestre|media|medio|desde|ultimos|ultimas|por mes|todo mes|cada mes)\b`)
	// Parcelado, financiado ou emprestado não é "cabe no mês": a IA conversa.
	installmentRe = regexp.MustCompile(`\d+ ?x\b|\b(parcel|vezes|financ|empres|consorcio)`)
	// "quanto gastei este mês?", sem lugar nem categoria: o total do mês.
	monthTotalRe = regexp.MustCompile(`^quanto (eu )?(ja )?(gastei|gasto)( no total| ate agora)?( (esse|este|neste|nesse|no) mes| (no )?(mes passado|mes anterior|ultimo mes)| em (janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)( de \d{4})?)?( no total| ate agora)?$`)
	// Crescimento ("qual gasto mais cresceu?") é tendência, não o maior gasto: a IA olha os achados.
	growthRe      = regexp.MustCompile(`\b(cresc\w*|aument\w*|subiu|subiram|dispar\w*|diminu\w*|caiu|cairam|reduz\w*)\b`)
	reserveAskRe  = regexp.MustCompile(`^(quanto|qual|quando|em quanto tempo)\b`)
	reservePlanRe = regexp.MustCompile(`\b(guardar|juntar|poupar|separar|ter|precis\w*|ideal|minima|tamanho|complet\w*|cheg\w*)\b`)
)

// appAnswer é a resposta do app para a pergunta, ou "" se ela vai para a IA.
func appAnswer(q Question) string {
	if onboarding(q.Agent, q.History, q.Text) {
		return ""
	}
	f := normalize(q.Text)
	if f == "" || len([]rune(f)) > maxAppRunes || strings.ContainsAny(f, "?!") || strings.Contains(f, ". ") || actionRe.MatchString(f) {
		return ""
	}
	// Conceito antes do filtro de conselho: "o que é", "como funciona" e "vale a pena" são a forma dessas perguntas.
	if s := conceptAnswer(q, f); s != "" {
		return s
	}
	if strings.Contains(f, " ou ") || adviceRe.MatchString(f) || twoQuestions(f) {
		return ""
	}
	for _, in := range appIntents {
		if in.match(f) {
			return in.answer(q, f)
		}
	}
	return ""
}

// normalize: a pergunta sem acentos, sem a pontuação do fim e sem o começo que não muda
// nada ("oi, Lia,", "me diz", "e se eu pagar 500 a mais?").
func normalize(text string) string {
	return strings.TrimPrefix(leadRe.ReplaceAllString(strings.TrimRight(fold(text), "?!. "), ""), "e ")
}

func twoQuestions(f string) bool {
	n := 0
	for _, part := range partRe.Split(f, -1) {
		if askedRe.MatchString(part) {
			n++
		}
	}
	return n > 1
}

type appIntent struct {
	match  func(f string) bool
	answer func(q Question, f string) string // "" quando falta o que a conta precisa: vai para a IA
}

func matches(expr string) func(string) bool { return regexp.MustCompile(expr).MatchString }

// appIntents, na ordem: o primeiro assunto que casa decide.
var appIntents = []appIntent{
	// "gastei mais que no mês passado?", "o que mudou do mês passado para este?"
	{matches(`\bgast\w*\b.*\b(mais|menos)\b.*\b(mes passado|mes anterior|ultimo mes)\b|^o que mudou\b|\bcompar\w*\b.*\bmes(es)?\b`), compareAnswer},
	// "o que vence esta semana?", "quando vence o cartão?", "o que tenho pra pagar este mês?"
	{matches(`^(o que|quais?|que|quando|tem|tenho|algum|alguma)\b.*\bvenc(e|em|endo|er|imento|imentos)\b|^(o que|quais?|que) (eu )?(tenho|tem) (pra|para|que|de) pagar\b`), dueAnswer},
	// "quando fico livre das dívidas?", "qual dívida pago primeiro?"
	{matches(`^quando\b.*\blivre\b|^quando\b.*\b(quit\w*|termin\w*|acab\w*|zer\w*|sair|saio)\b.*\b(divida|dividas|cartao|emprestimo|financiamento|parcela|parcelas|cheque)\b|^(qual|quais|que)\b.*\b(pag|quit)\w*\b.*\bprimeiro\b|^(qual|quais) dividas?\b.*\bprimeiro\b`),
		func(q Question, f string) string { return debtPlan(q, f, "") }},
	// "e se eu pagar 500 a mais?", "quanto adianta pagar 200 a mais por mês?"
	{matches(`\bse\b.*\b(pag\w*|der|dando|coloc\w*|mand\w*|bot\w*|amortiz\w*)\b.*\b(a mais|extra)\b|^(quanto|qual|o que)\b.*\b(adianta|economiz\w*|ganho|muda)\b.*\b(a mais|extra)\b|^pagando\b.*\b(a mais|extra)\b`),
		func(q Question, f string) string {
			if v := questionMoney(q.Text); v != "" { // "e se eu pagar mais?": sem o valor, é conversa
				return debtPlan(q, f, v)
			}
			return ""
		}},
	// "quanto devo no total?", "qual o juros do meu cartão?"
	{matches(`^quanto (eu )?(ainda )?(devo|estou devendo|to devendo)( no total| ao todo| hoje| (de|em|nas) dividas?| no cartao)?$|^(qual|quanto) (e |eh )?(o )?(total|valor|tamanho) (da|das|de) (minha |minhas )?dividas?\b|^(qual|quais|quanto) (e |sao |esta |estao )?(o |os |a |as )?(juros|taxa|taxas)\b.*\b(divida|dividas|cartao|emprestimo|financiamento|cheque)\b|^quanto (eu )?pago de juros\b`), debtListAnswer},
	// "quanto ainda posso gastar?", "quanto sobra no mês?"
	{matches(`^quanto (ainda )?(eu )?(posso|consigo|da pra|da para|tenho pra|tenho para) gastar( ainda)?( (esse|este|neste|nesse|no) mes| ate o fim do mes| por dia)?$|^quanto (ainda )?(me )?(sobra|resta)( ainda)?( (esse|este|neste|nesse|no) mes| ate o fim do mes| (pra|para) gastar| livre| de dinheiro)?$`),
		func(q Question, _ string) string { return liaAllowance(q) }},
	// "posso gastar 300?", "consigo comprar um celular de 2 mil?"
	{matches(`^(sera que )?(eu )?(posso|consigo|da pra|da para|cabe)\b.*\b(gastar|comprar|torrar)\b`), affordAnswer}, // "pagar a fatura" já está nas contas do mês
	// "quanto guardar por mês para a reserva?"
	{func(f string) bool {
		return hasWord(f, "reserva") && reserveAskRe.MatchString(f) && reservePlanRe.MatchString(f)
	}, reserveAnswer},
	// "quanto preciso guardar para a viagem?", "se eu guardar 400, quando chego?", "minhas metas estão no ritmo?"
	{matches(`^quanto\b.*\b(guardar|juntar|poupar|separar|falta|faltam)\b|\bquando\b.*\b(cheg\w*|alcanc\w*|ating\w*|bat\w*|complet\w*)\b|\bmetas?\b.*\b(ritmo|atrasad\w*|em dia|no prazo)\b|^como (esta|estao|anda|andam|vai|vao) (a |as )?(minha |minhas )?metas?\b`), goalsAnswer},
	// "tenho assinatura esquecida?", "quanto pago de assinaturas?"
	{matches(`^(tenho|tem|quais|qual|quanto|como|minhas|lista)\b.*\bassinatura`), func(q Question, _ string) string { return subscriptionsText(q) }},
	// "em que categoria gasto mais?", "com o que eu mais gasto?"
	{matches(`^(em que|qual|quais|onde|com o que|com que|no que)\b.*\b(categoria|categorias|gasto|gastei|gastando)\b.*\bmais\b|^(com o que|com que|no que|onde) (eu )?mais (gasto|gastei)\b`), topCategoriesAnswer},
	// "qual meu saldo do mês?", "como está meu mês?"
	{matches(`\b(saldo|balanco|resumo) (do|deste|desse|no) mes\b|^como (foi|esta|ta|anda) (o )?(meu )?mes\b`), func(q Question, f string) string {
		if otherMonth(f) {
			return "" // o resumo daqui é o do mês de hoje
		}
		return overviewOffline(q)
	}},
	// "quanto gastei com uber?", "quanto gastei em setembro?"
	{matches(`^(quanto|qto)\b.*\bgast(ei|o|amos|ou|aram)\b|^(qual|quais) (foi |foram |e |sao )?(o |os )?(meu |meus )?gastos?\b|^(meus |os )?gastos? (com|de|no|na|em)\b`), spendingAnswer},
}

// noDebts: o texto quando não há dívida com saldo.
const noDebts = "Nenhuma dívida cadastrada com saldo. Se tiver alguma, cadastre na tela Dívidas e eu monto o plano."

func openDebts(l domain.Ledger) []domain.Debt {
	var out []domain.Debt
	for _, d := range l.ActiveDebts() {
		if d.BalanceCents > 0 {
			out = append(out, d)
		}
	}
	return out
}

// debtPlan: o plano das dívidas, com o valor a mais por mês (extra) se a pergunta trouxe.
func debtPlan(q Question, f, extra string) string {
	debts := openDebts(q.Ledger)
	if len(debts) == 0 {
		return noDebts
	}
	order := ""
	if strings.Contains(f, "menor") || strings.Contains(f, "bola") {
		order = "menor saldo"
	}
	out := "**Plano das dívidas**\n\n" + bullets(NewTools(q.Ledger, q.Today, nil).simulateDebts(extra, order))
	// Só no mínimo não acaba: quanto a mais faz ter fim (os juros menos os mínimos, em centenas).
	if p := finance.PlanDebts(debts); extra == "" && p.MonthlyInterest >= p.TotalMinCents && !finance.PayoffSchedule(debts, 0, true).OK {
		example := ((p.MonthlyInterest-p.TotalMinCents)/10000 + 1) * 100
		out += fmt.Sprintf("\n\nSó com o mínimo, não acaba: os juros (%s por mês) passam dos mínimos (%s). Pergunte \"e se eu pagar %d a mais?\", com o valor que dá para pagar, e eu calculo quando acaba.",
			money.Format(p.MonthlyInterest), money.Format(p.TotalMinCents), example)
	}
	return out
}

func debtListAnswer(q Question, f string) string {
	debts := openDebts(q.Ledger)
	// "qual a taxa do cheque especial?" sem cheque especial cadastrado: é pergunta de conceito.
	for _, w := range []string{"cartao", "emprestimo", "financiamento", "cheque"} {
		if hasWord(f, w) && !slices.ContainsFunc(debts, func(d domain.Debt) bool { return strings.Contains(fold(d.Name+" "+d.Creditor), w) }) {
			return ""
		}
	}
	if len(debts) == 0 {
		return noDebts
	}
	p := finance.PlanDebts(debts)
	brl := money.Format
	which := "1 dívida"
	if len(debts) > 1 {
		which = fmt.Sprintf("%d dívidas", len(debts))
	}
	lines := make([]string, len(p.Debts))
	for i, d := range p.Debts {
		lines[i] = fmt.Sprintf("%s: %s, juros de %s ao mês (%s por mês), mínimo %s", d.Name, brl(d.BalanceCents), RatePct(d.MonthlyRateBP), brl(d.MonthlyInterestCents), brl(d.MinPaymentCents))
		if d.DueDay > 0 {
			lines[i] += fmt.Sprintf(", vence dia %d", d.DueDay)
		}
		lines[i] += "."
	}
	return fmt.Sprintf("**Você deve %s** em %s, com %s de juros por mês.\n\n", brl(p.TotalCents), which, brl(p.MonthlyInterest)) + bullets(strings.Join(lines, "\n"))
}

// affordAnswer: "posso gastar 300?" compara o valor com o que ainda cabe no mês.
func affordAnswer(q Question, f string) string {
	v := questionMoney(q.Text)
	if v == "" || installmentRe.MatchString(f) {
		return ""
	}
	cents, err := money.Parse(v)
	if err != nil || cents <= 0 {
		return ""
	}
	in := finance.Insights(q.Ledger, q.Today)
	if in.IncomeCents == 0 {
		return liaAllowance(q) // sem renda nenhuma: pede a entrada
	}
	brl := money.Format
	income := "renda de " + brl(in.IncomeCents)
	left := in.FreeCents - cents
	var out string
	switch home := finance.Home(q.Ledger, q.Today); {
	case !home.EstimateAvailable: // a mesma regra do Início: a sobra não vira limite de gasto
		out = "**Sem estimativa por enquanto.** " + home.EstimateReason
	case left >= 0:
		out = fmt.Sprintf("**Cabe no mês, pela estimativa do app.** Ainda cabem %s (%s, já contando os fixos que vão cair); depois dos %s, ficam %s.", brl(in.FreeCents), income, brl(cents), brl(left))
		if days := int64(in.DaysInMonth - in.Day + 1); left > 0 && days > 0 {
			out += fmt.Sprintf(" Dá uns %s por dia até o fim do mês.", brl(left/days))
		}
	case in.FreeCents > 0:
		out = fmt.Sprintf("**Não cabe no mês sem passar da renda.** Pela estimativa do app, ainda cabem %s (%s, já contando os fixos que vão cair): faltam %s para os %s.", brl(in.FreeCents), income, brl(-left), brl(cents))
	default:
		out = fmt.Sprintf("**Não cabe: o mês já passou da renda** em %s, contando os fixos que ainda vão cair (%s).", brl(-in.FreeCents), income)
	}
	// "posso gastar 300 no mercado?": o orçamento da categoria também conta.
	if id, _, _ := NewTools(q.Ledger, q.Today, nil).categoryIn(f); id != "" {
		for _, b := range in.Budgets {
			if b.CategoryID != id {
				continue
			}
			if b.RemainingCents >= cents {
				out += fmt.Sprintf(" No orçamento de %s, restam %s de %s; depois, ficam %s.", b.Name, brl(b.RemainingCents), brl(b.BudgetCents), brl(b.RemainingCents-cents))
			} else {
				out += fmt.Sprintf(" Mas no orçamento de %s restam só %s de %s.", b.Name, brl(max(b.RemainingCents, 0)), brl(b.BudgetCents))
			}
		}
	}
	if c := finance.Diagnose(q.Ledger, q.Today).Costliest; c != nil && c.MonthlyInterestCents > 0 {
		out += fmt.Sprintf("\n\nLembrete: a dívida %s custa %s de juros por mês (%s ao mês).", c.Name, brl(c.MonthlyInterestCents), RatePct(c.MonthlyRateBP))
	}
	if cents > in.IncomeCents { // compra maior que a renda do mês: vira meta
		out += "\n\nPara juntar esse valor, crie uma meta na tela Metas: o app calcula quanto guardar por mês."
	}
	return out
}

// namedGoal: a pergunta cita uma meta em andamento pelo nome.
func namedGoal(q Question, f string) bool {
	for _, g := range finance.Goals(q.Ledger, q.Today) {
		if !g.Done && strings.Contains(f, fold(g.Name)) {
			return true
		}
	}
	return false
}

func reserveAnswer(q Question, f string) string {
	if namedGoal(q, f) { // a pessoa tem uma meta com esse nome: é ela
		return titoOffline(NewTools(q.Ledger, q.Today, nil), q)
	}
	return reserveText(q) // sem meses fechados para medir os gastos: a IA explica o que falta
}

func goalsAnswer(q Question, f string) string {
	named := namedGoal(q, f)
	newGoal := questionMoney(q.Text) != "" && questionMonth(f, q.Today) != "" // "juntar 3 mil até julho"
	if !named && (strings.Contains(f, "reserva") || !newGoal && !hasWord(f, "meta") && !hasWord(f, "metas")) {
		return "" // não diz de que meta fala ("quanto falta para quitar o cartão?"): a IA conversa
	}
	if !named && !newGoal && len(q.Ledger.ActiveGoals()) == 0 {
		return "Nenhuma meta em andamento. Crie uma na tela Metas e eu digo quanto guardar por mês."
	}
	return titoOffline(NewTools(q.Ledger, q.Today, nil), q)
}

// compareAnswer: este mês contra o passado, até o mesmo dia (o mês pela metade
// comparado com o mês inteiro sempre pareceria menor), e o que mudou por categoria.
func compareAnswer(q Question, f string) string {
	if monthRe.MatchString(f) || periodRe.MatchString(f) {
		return "" // outros meses ou outro período: a IA compara com a ferramenta
	}
	in := finance.Insights(q.Ledger, q.Today)
	if in.VariableCents == 0 && in.LastMonthSameDayCents == 0 {
		return "Ainda não há gastos neste mês para comparar. Lance os gastos e eu mostro o que mudou."
	}
	brl := money.Format
	last := domain.NewMonth(q.Today.Year(), q.Today.Month()).Prev().Label()
	diff := in.VariableCents - in.LastMonthSameDayCents
	head := "**O mesmo que no mês passado.**"
	if diff > 0 {
		head = fmt.Sprintf("**%s a mais que no mês passado.**", brl(diff))
	} else if diff < 0 {
		head = fmt.Sprintf("**%s a menos que no mês passado.**", brl(-diff))
	}
	out := head + "\n\n" + bullets(fmt.Sprintf("Até o dia %d: %s em gastos variáveis; no mesmo período de %s, %s.\nCom as contas fixas, a projeção do mês é %s (%s fechou em %s).",
		in.Day, brl(in.VariableCents), last, brl(in.LastMonthSameDayCents), brl(in.ProjectedCents), last, brl(in.LastMonthCents)))
	var cats []string
	for _, line := range strings.Split(NewTools(q.Ledger, q.Today, nil).compareMonths("", ""), "\n")[2:] {
		if len(cats) < 5 && !strings.HasSuffix(line, "(+R$ 0,00)") {
			cats = append(cats, line)
		}
	}
	if len(cats) > 0 {
		out += "\n\n**Por categoria** (" + last + " inteiro → este mês até agora)\n\n" + strings.Join(cats, "\n")
	}
	return out
}

func topCategoriesAnswer(q Question, f string) string {
	if growthRe.MatchString(f) || periodRe.MatchString(f) {
		return ""
	}
	m := spendingMonth(f, q.Today)
	sum := finance.Summarize(m, q.Ledger)
	if len(sum.Categories) == 0 {
		return fmt.Sprintf("Nenhuma saída lançada em %s ainda.", m.Label())
	}
	brl := money.Format
	top := sum.Categories[0]
	out := fmt.Sprintf("**%s: %s em %s** (%d%% das saídas).", top.Name, brl(top.SpentCents), m.Label(), top.SharePct)
	if rest := categoryLines(sum.Categories[1:], 4); rest != "" {
		out += "\n\nDepois:\n" + rest
	}
	return out
}

// categoryLines: as n primeiras categorias, uma por linha de lista.
func categoryLines(cats []finance.CategoryLine, n int) string {
	var lines []string
	for i, c := range cats {
		if i == n {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s: %s (%d%%)", c.Name, money.Format(c.SpentCents), c.SharePct))
	}
	return strings.Join(lines, "\n")
}

func spendingAnswer(q Question, f string) string {
	// Período que a conta não cobre, ou dois alvos ("com uber e 99"): a conta daqui pegaria só um.
	if periodRe.MatchString(f) || strings.Contains(f, " e ") {
		return ""
	}
	if s := NewTools(q.Ledger, q.Today, nil).placeSpending(f); s != "" {
		return s
	}
	if !monthTotalRe.MatchString(f) {
		return "" // um lugar ou categoria que o app não achou: a IA busca nos lançamentos
	}
	m := spendingMonth(f, q.Today)
	sum := finance.Summarize(m, q.Ledger)
	brl := money.Format
	out := fmt.Sprintf("**Saídas de %s: %s.** Entradas: %s. Sobra: %s.", m.Label(), brl(sum.OutCents), brl(sum.InCents), brl(sum.BalanceCents()))
	if top := categoryLines(sum.Categories, 3); top != "" {
		out += "\n\nOnde mais saiu:\n" + top
	}
	return out
}

// dueAnswer: o que vence nos próximos dias. "No mês que vem" ou num mês pelo nome: a janela
// daqui (a partir de hoje) não cobre o mês inteiro, então vai para a IA.
func dueAnswer(q Question, f string) string {
	if strings.Contains(f, "que vem") || strings.Contains(f, "proximo mes") || strings.Contains(f, "mes seguinte") || monthRe.MatchString(f) {
		return ""
	}
	s := NewTools(q.Ledger, q.Today, nil).upcoming(dueDays(f))
	if !strings.Contains(s, "\n") {
		return s // "Nada vence nos próximos 7 dias…": uma frase, sem lista
	}
	return bullets(s)
}

// otherMonth: a pergunta fala de outro mês ("mês passado", "em setembro").
func otherMonth(f string) bool {
	return monthRe.MatchString(f) || strings.Contains(f, "passad") || strings.Contains(f, "anterior") || strings.Contains(f, "ultimo")
}

// dueDays: quantos dias olhar nos vencimentos ("" é o padrão, 10). q já vem com fold.
func dueDays(q string) string {
	switch m := daysRe.FindStringSubmatch(q); {
	case m != nil:
		return m[1]
	case strings.Contains(q, "semana"):
		return "7"
	case strings.Contains(q, "mes") || strings.HasPrefix(q, "quando"): // "quando vence o cartão?": o mês todo
		return "31"
	}
	return ""
}
