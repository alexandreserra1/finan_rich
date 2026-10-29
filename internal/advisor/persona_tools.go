package advisor

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Ferramentas de cada personagem do time: a IA escolhe a ferramenta e os
// valores; a conta é sempre do Go (finance), igual à das telas. Só leem os
// dados: nada é gravado aqui.

func teamTools() []llm.Tool {
	return []llm.Tool{
		tool("simular_dividas", "Simula pagar as dívidas com um valor a mais por mês: ordem de pagamento, mês em que fica livre de todas e juros no caminho, comparando com pagar só o mínimo e com a outra ordem. Use para \"e se eu pagar X a mais?\" ou \"qual pagar primeiro?\".",
			`{"type":"object","properties":{
				"valor_extra":{"type":"string","description":"quanto a mais por mês, em reais; padrão 0"},
				"ordem":{"type":"string","description":"\"maior juros\" (padrão, economiza mais) ou \"menor saldo\" (vitórias rápidas)"}}}`),
		tool("simular_meta", "Calcula uma meta: quanto guardar por mês para chegar num prazo, ou em que mês chega guardando um valor por mês. Use para \"quanto preciso guardar?\" ou \"se eu guardar X, quando chego?\".",
			`{"type":"object","properties":{
				"meta":{"type":"string","description":"nome da meta"},
				"prazo":{"type":"string","description":"AAAA-MM; padrão: o prazo da meta"},
				"valor_mes":{"type":"string","description":"quanto guardar por mês, em reais"}},
				"required":["meta"]}`),
		tool("achados", "Os achados do mês: ritmo dos gastos e projeção, categorias acima da média, assinaturas e contas fixas, onde dá para economizar e os maiores gastos (com [id]).",
			`{"type":"object","properties":{}}`),
		tool("comparar_meses", "Compara dois meses: entradas, saídas e o que mudou em cada categoria, da maior diferença para a menor.",
			`{"type":"object","properties":{
				"de":{"type":"string","description":"AAAA-MM; padrão: mês passado"},
				"ate":{"type":"string","description":"AAAA-MM; padrão: este mês"}}}`),
		tool("proximos_vencimentos", "O que vence nos próximos dias: parcelas de dívidas, faturas de cartão e contas fixas, e os orçamentos perto do teto.",
			`{"type":"object","properties":{"dias":{"type":"string","description":"quantos dias olhar; padrão 10, máximo 31"}}}`),
	}
}

// monthAfter: o mês n meses depois do atual ("maio de 2027").
func (t *Tools) monthAfter(n int) domain.Month {
	m := domain.NewMonth(t.today.Year(), t.today.Month())
	for range n {
		m = m.Next()
	}
	return m
}

// parseMonthArg aceita "AAAA-MM" (vazio = padrão).
func parseMonthArg(s string, def domain.Month) (domain.Month, error) {
	if s == "" {
		return def, nil
	}
	if _, err := time.Parse("2006-01", s); err != nil {
		return def, fmt.Errorf("mês %q inválido: use AAAA-MM", s)
	}
	return domain.ParseMonth(s), nil
}

func (t *Tools) simulateDebts(extra, order string) string {
	var debts []domain.Debt
	var mins int64
	for _, d := range t.ledger.ActiveDebts() {
		if d.BalanceCents > 0 {
			debts = append(debts, d)
			mins += d.MinPaymentCents
		}
	}
	if len(debts) == 0 {
		return "Nenhuma dívida cadastrada com saldo."
	}
	var x int64
	if extra != "" {
		v, err := cents(extra, "valor a mais")
		if err != nil {
			return "não deu para simular: " + err.Error()
		}
		x = v
	}
	avalanche := !strings.Contains(fold(order), "menor") && !strings.Contains(fold(order), "bola")
	name := map[bool]string{true: "maior juros primeiro", false: "menor saldo primeiro"}
	describe := func(s finance.Schedule) string {
		if !s.OK {
			return "não acaba: o pagamento não cobre os juros"
		}
		return fmt.Sprintf("livre em %s (%s), com %s de juros", finance.Meses(s.Months), t.monthAfter(s.Months).Label(), money.Format(s.InterestCents))
	}
	plan := finance.PayoffSchedule(debts, x, avalanche)
	base := finance.PayoffSchedule(debts, 0, avalanche)
	other := finance.PayoffSchedule(debts, x, !avalanche)

	var b strings.Builder
	fmt.Fprintf(&b, "Ordem (%s): %s.\n", name[avalanche], strings.Join(plan.Names, " → "))
	fmt.Fprintf(&b, "Só o mínimo (%s por mês): %s.\n", money.Format(mins), describe(base))
	if x > 0 {
		fmt.Fprintf(&b, "Pagando %s por mês (mínimos + %s a mais).\n", money.Format(mins+x), money.Format(x))
		fmt.Fprintf(&b, "Com %s a mais: %s.\n", money.Format(x), describe(plan))
		switch {
		case plan.OK && !base.OK:
			b.WriteString("Com o valor a mais, as dívidas passam a ter fim.\n")
		case plan.OK && base.OK:
			fmt.Fprintf(&b, "Diferença: sai %s antes e paga %s a menos de juros.\n", finance.Meses(base.Months-plan.Months), money.Format(base.InterestCents-plan.InterestCents))
		}
	}
	fmt.Fprintf(&b, "Na outra ordem (%s), com o mesmo valor: %s.\n", name[!avalanche], describe(other))
	if len(plan.PaidOff) > 0 {
		var each []string
		for _, p := range plan.PaidOff {
			each = append(each, fmt.Sprintf("%s em %s", p.Name, t.monthAfter(p.Month).Label()))
		}
		b.WriteString("Cada uma zera: " + strings.Join(each, "; ") + ".")
	}
	return strings.TrimSpace(b.String())
}

func (t *Tools) simulateGoal(name, deadline, perMonth string) string {
	g, err := t.goal(name)
	if err != nil {
		return "não deu para calcular: " + err.Error()
	}
	var v finance.GoalView
	for _, x := range finance.Goals(t.ledger, t.today) {
		if x.ID == g.ID {
			v = x
		}
	}
	if v.Done {
		return fmt.Sprintf("A meta %s já foi atingida: %s de %s.", g.Name, money.Format(v.ProgressCents), money.Format(g.TargetCents))
	}
	lines := []string{fmt.Sprintf("Meta %s: guardado %s de %s, faltam %s.", g.Name, money.Format(v.ProgressCents), money.Format(g.TargetCents), money.Format(v.RemainingCents))}
	if deadline == "" && perMonth == "" {
		deadline = g.Deadline
	}
	if deadline != "" {
		if _, err := parseMonthArg(deadline, domain.Month{}); err != nil {
			return "não deu para calcular: " + err.Error()
		}
		months, per, late := finance.GoalPlan(v.RemainingCents, deadline, t.today)
		if late {
			lines = append(lines, "Até "+domain.ParseMonth(deadline).Label()+": esse prazo já passou. Escolha um mês a partir de agora.")
		} else {
			lines = append(lines, fmt.Sprintf("Para chegar até %s: guarde %s por mês durante %s.", domain.ParseMonth(deadline).Label(), money.Format(per), finance.Meses(months)))
		}
	}
	if perMonth != "" {
		per, err := cents(perMonth, "valor por mês")
		if err != nil || per <= 0 {
			return "não deu para calcular: informe quanto guardar por mês"
		}
		months := int((v.RemainingCents + per - 1) / per)
		line := fmt.Sprintf("Guardando %s por mês, chega em %s (%s)", money.Format(per), finance.Meses(months), t.monthAfter(months-1).Label()) // o mês atual conta, como no GoalPlan
		if g.Deadline != "" {
			if left, _, _ := finance.GoalPlan(v.RemainingCents, g.Deadline, t.today); left > 0 && months > left {
				line += fmt.Sprintf(", %s depois do prazo (%s)", finance.Meses(months-left), domain.ParseMonth(g.Deadline).Label())
			}
		}
		lines = append(lines, line+".")
	}
	if len(lines) == 1 {
		lines = append(lines, "Sem prazo: diga um prazo ou quanto quer guardar por mês para eu calcular.")
	}
	return strings.Join(lines, "\n")
}

func (t *Tools) findings() string {
	in := finance.Insights(t.ledger, t.today)
	brl := money.Format
	var b strings.Builder
	fmt.Fprintf(&b, "%s, até o dia %d: gastos variáveis %s; o mês deve fechar em %s (mês passado: %s).\n", in.Label, in.Day, brl(in.VariableCents), brl(in.ProjectedCents), brl(in.LastMonthCents))
	var above []string
	for _, c := range in.Categories {
		if c.Avg3Cents > 0 && c.DiffPct >= 25 {
			above = append(above, fmt.Sprintf("- %s: %s este mês, %d%% acima da média (%s)", c.Name, brl(c.Cents), c.DiffPct, brl(c.Avg3Cents)))
		}
	}
	if len(above) > 0 {
		b.WriteString("Acima da média:\n" + strings.Join(above, "\n") + "\n")
	}
	if len(in.Subscriptions) > 0 {
		b.WriteString("Cobranças todo mês:\n")
		for _, s := range in.Subscriptions {
			kind := "conta fixa"
			if s.Bucket == domain.BucketDesejos {
				kind = "assinatura"
			}
			fmt.Fprintf(&b, "- %s (%s): %s por mês, %s por ano\n", s.Name, kind, brl(s.MonthlyCents), brl(s.YearlyCents))
		}
	}
	if len(in.Savings) > 0 {
		b.WriteString("Onde economizar:\n")
		for i, s := range in.Savings {
			if i == 3 {
				break
			}
			fmt.Fprintf(&b, "- %s: %s\n", s.Title, s.Text)
		}
	}
	if len(in.Biggest) > 0 {
		b.WriteString("Maiores gastos do mês:\n")
		for i, tx := range in.Biggest {
			if i == 5 {
				break
			}
			fmt.Fprintf(&b, "- [%s] %s\n", tx.ID, t.txLabel(tx))
		}
	}
	return strings.TrimSpace(b.String())
}

func (t *Tools) compareMonths(from, to string) string {
	now := domain.NewMonth(t.today.Year(), t.today.Month())
	a, err := parseMonthArg(from, now.Prev())
	if err != nil {
		return "não deu para comparar: " + err.Error()
	}
	b, err := parseMonthArg(to, now)
	if err != nil {
		return "não deu para comparar: " + err.Error()
	}
	sa, sb := finance.Summarize(a, t.ledger), finance.Summarize(b, t.ledger)
	brl := money.Format
	spent := map[string][2]int64{}
	for _, c := range sa.Categories {
		v := spent[c.Name]
		v[0] = c.SpentCents
		spent[c.Name] = v
	}
	for _, c := range sb.Categories {
		v := spent[c.Name]
		v[1] = c.SpentCents
		spent[c.Name] = v
	}
	names := make([]string, 0, len(spent))
	for n := range spent {
		names = append(names, n)
	}
	abs := func(n string) int64 { d := spent[n][1] - spent[n][0]; return max(d, -d) }
	sort.Slice(names, func(i, j int) bool {
		if abs(names[i]) != abs(names[j]) {
			return abs(names[i]) > abs(names[j])
		}
		return names[i] < names[j]
	})
	lines := []string{
		fmt.Sprintf("%s → %s", a.Label(), b.Label()),
		fmt.Sprintf("Entradas: %s → %s. Saídas: %s → %s.", brl(sa.InCents), brl(sb.InCents), brl(sa.OutCents), brl(sb.OutCents)),
	}
	for i, n := range names {
		if i == 8 {
			break
		}
		d := spent[n][1] - spent[n][0]
		sign := "+"
		if d < 0 {
			sign, d = "−", -d
		}
		lines = append(lines, fmt.Sprintf("- %s: %s → %s (%s%s)", n, brl(spent[n][0]), brl(spent[n][1]), sign, brl(d)))
	}
	return strings.Join(lines, "\n")
}

func (t *Tools) upcoming(days string) string {
	n := 10
	if days != "" {
		v, err := strconv.Atoi(days)
		if err != nil || v < 1 {
			return "não deu: informe quantos dias olhar (1 a 31)"
		}
		n = min(v, 31)
	}
	day := time.Date(t.today.Year(), t.today.Month(), t.today.Day(), 0, 0, 0, 0, time.UTC)
	end := day.AddDate(0, 0, n)
	current := domain.NewMonth(t.today.Year(), t.today.Month())
	// next: a próxima data (a partir de hoje) com esse dia do mês.
	next := func(d int) time.Time {
		at := current.Day(d)
		if at.Before(day) {
			at = current.Next().Day(d)
		}
		return at
	}
	type item struct {
		at   time.Time
		text string
	}
	var items []item
	in := func(at time.Time) bool { return !at.After(end) }
	when := func(at time.Time) string {
		d := int(at.Sub(day).Hours() / 24)
		switch d {
		case 0:
			return "hoje"
		case 1:
			return "amanhã"
		}
		return fmt.Sprintf("em %d dias", d)
	}
	for _, d := range t.ledger.ActiveDebts() {
		if d.DueDay > 0 && d.BalanceCents > 0 {
			if at := next(d.DueDay); in(at) {
				items = append(items, item{at, fmt.Sprintf("%s: parcela mínima %s em %s (%s)", d.Name, money.Format(d.MinPaymentCents), at.Format("02/01"), when(at))})
			}
		}
	}
	for _, a := range finance.Accounts(t.ledger, t.today).Accounts {
		if inv := a.Current; inv != nil && inv.TotalCents > 0 {
			if at, err := time.Parse(domain.DateLayout, inv.DueDate); err == nil && in(at) {
				state := "ainda aberta"
				if inv.Closed {
					state = "já fechou"
				}
				items = append(items, item{at, fmt.Sprintf("Fatura do %s: %s em %s (%s, %s)", a.Name, money.Format(inv.TotalCents), at.Format("02/01"), when(at), state)})
			}
		}
	}
	for _, r := range t.ledger.ActiveRecurring() {
		if r.Kind == domain.KindSaida && !r.Paused && r.Day > 0 {
			if at := next(r.Day); in(at) && (r.End == "" || r.End >= domain.NewMonth(at.Year(), at.Month()).Key()) {
				items = append(items, item{at, fmt.Sprintf("%s: %s em %s (%s, conta fixa)", r.Description, money.Format(r.AmountCents), at.Format("02/01"), when(at))})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].at.Before(items[j].at) })
	lines := []string{fmt.Sprintf("Próximos %d dias (até %s):", n, end.Format("02/01"))}
	for _, it := range items {
		lines = append(lines, "- "+it.text)
	}
	for _, b := range finance.Budgets(t.ledger, current) {
		if b.UsedPct >= 80 {
			lines = append(lines, fmt.Sprintf("- Orçamento de %s: %s de %s no mês (%d%%)", b.Name, money.Format(b.SpentCents), money.Format(b.BudgetCents), b.UsedPct))
		}
	}
	if len(lines) == 1 {
		return fmt.Sprintf("Nada vence nos próximos %d dias, e nenhum orçamento passou de 80%%.", n)
	}
	return strings.Join(lines, "\n")
}

// ---- contas prontas antes de perguntar ----
//
// Modelo pequeno ou grátis às vezes pula a ferramenta e chuta o número. Por
// isso, antes de perguntar, o Go já roda a conta principal do personagem com
// o que dá para tirar da pergunta, e o resultado vai junto dos dados. A
// ferramenta continua lá para outras contas.

var moneyRe = regexp.MustCompile(`\d[\d.]*(?:,\d{1,2})?`)

// questionMoney devolve o primeiro valor em reais da pergunta ("R$ 500",
// "400 reais", "1.200,50"); números de dias, meses, anos ou % ficam de fora.
// shortcut: lê só o primeiro valor; com dois valores na pergunta, a IA ainda pode chamar a ferramenta com o outro.
func questionMoney(q string) string {
	for _, loc := range moneyRe.FindAllStringIndex(q, -1) {
		rest := strings.TrimSpace(fold(q[loc[1]:]))
		if strings.HasPrefix(rest, "%") || strings.HasPrefix(rest, "dia") || strings.HasPrefix(rest, "mes") ||
			strings.HasPrefix(rest, "ano") || strings.HasPrefix(rest, "x ") || strings.HasPrefix(rest, "vez") {
			continue
		}
		return strings.TrimRight(q[loc[0]:loc[1]], ".")
	}
	return ""
}

var daysRe = regexp.MustCompile(`(\d+)\s*dias`)

// prefetch é a conta principal do personagem para esta pergunta ("" para a Jade).
func prefetch(agent string, t *Tools, question string) string {
	q := fold(question)
	switch persona(agent).ID {
	case "bento":
		order := ""
		if strings.Contains(q, "menor") || strings.Contains(q, "bola") {
			order = "menor saldo"
		}
		return t.simulateDebts(questionMoney(question), order)
	case "tito":
		var goals, named []string
		for _, g := range finance.Goals(t.ledger, t.today) {
			if g.Done {
				continue
			}
			goals = append(goals, g.Name)
			if strings.Contains(q, fold(g.Name)) {
				named = append(named, g.Name)
			}
		}
		if len(named) > 0 {
			goals = named
		}
		var out []string
		for _, g := range goals {
			out = append(out, t.simulateGoal(g, "", questionMoney(question)))
		}
		return strings.Join(out, "\n\n")
	case "otavio":
		return t.findings() + "\n\n" + t.compareMonths("", "")
	case "luna":
		days := ""
		switch m := daysRe.FindStringSubmatch(q); {
		case m != nil:
			days = m[1]
		case strings.Contains(q, "semana"):
			days = "7"
		case strings.Contains(q, "mes"):
			days = "31"
		}
		return t.upcoming(days)
	}
	return ""
}

// withPrefetch junta a conta pronta ao retrato financeiro.
func withPrefetch(snapshot, agent string, t *Tools, question string) string {
	pre := prefetch(agent, t, question)
	if pre == "" {
		return snapshot
	}
	return snapshot + "\n\nContas prontas para esta pergunta (feitas pelo app; use estes números):\n" + pre
}
