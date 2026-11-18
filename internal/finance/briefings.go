package finance

import (
	"fmt"
	"slices"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Briefing é o resumo de um personagem do time para o Painel: o que ele viu
// na área dele, em até 3 linhas, e a tela onde agir. Só contas prontas, sem IA.
type Briefing struct {
	Agent Agent    `json:"agent"`
	Title string   `json:"title"`
	Lines []string `json:"lines"`
	Link  string   `json:"link"`
	Ask   string   `json:"ask"` // a pergunta do "Perguntar para ...", com resposta pronta (sem IA)
}

const maxBriefingLines = 3

// Briefings junta o que Otávio (dívidas), Tito (metas) e Lia (gastos) têm a
// dizer hoje. Os vencimentos ficam nos avisos (Alerts). Quem não tem nada fica de fora.
func Briefings(l domain.Ledger, today time.Time) []Briefing {
	out := []Briefing{}
	add := func(agent, title, link, ask string, lines []string) {
		if len(lines) > maxBriefingLines {
			lines = lines[:maxBriefingLines]
		}
		if len(lines) > 0 {
			out = append(out, Briefing{agentByID(agent), title, lines, link, ask})
		}
	}
	add("otavio", "Plano das dívidas", "/dividas", "Qual dívida devo pagar primeiro?", debtLines(l, today))
	add("tito", "Suas metas", "/metas", "Minhas metas estão no ritmo?", goalLines(l, today))
	add("lia", "Achados do mês", "/analise", "Onde estou gastando demais este mês?", spendingLines(l, today))
	return out
}

func agentByID(id string) Agent {
	for _, a := range Team {
		if a.ID == id {
			return a
		}
	}
	return Team[0]
}

// debtLines: a dívida a atacar primeiro e quando tudo acaba pagando o mínimo.
func debtLines(l domain.Ledger, today time.Time) []string {
	var debts []domain.Debt
	for _, d := range l.ActiveDebts() {
		if d.BalanceCents > 0 {
			debts = append(debts, d)
		}
	}
	if len(debts) == 0 {
		return nil
	}
	var lines []string
	if c := Diagnose(l, today).Costliest; c != nil && c.MonthlyInterestCents > 0 {
		first := "" // com uma dívida só, não há "primeiro"
		if len(debts) > 1 {
			first = " primeiro"
		}
		lines = append(lines, fmt.Sprintf("%s%s: %d,%02d%% ao mês (%s de juros por mês).",
			c.Name, first, c.MonthlyRateBP/100, c.MonthlyRateBP%100, money.Format(c.MonthlyInterestCents)))
	}
	if s := PayoffSchedule(debts, 0, true); s.OK {
		end := domain.NewMonth(today.Year(), today.Month())
		for range s.Months {
			end = end.Next()
		}
		lines = append(lines, fmt.Sprintf("Só no mínimo, fica livre de todas em %s (%s), com %s de juros.", Meses(s.Months), end.Label(), money.Format(s.InterestCents)))
	} else {
		lines = append(lines, "Só no mínimo, as dívidas não acabam: a parcela não cobre os juros.")
	}
	return lines
}

// goalLines: quanto guardar por mês em cada meta em andamento.
func goalLines(l domain.Ledger, today time.Time) []string {
	var lines []string
	for _, g := range Goals(l, today) {
		switch {
		case g.Done:
		case g.Late:
			lines = append(lines, fmt.Sprintf("%s: o prazo passou e faltam %s. Ajuste o prazo ou o valor.", g.Name, money.Format(g.RemainingCents)))
		case g.PerMonthCents > 0 && g.Deadline != "":
			lines = append(lines, fmt.Sprintf("%s: guarde %s por mês até %s (faltam %s).",
				g.Name, money.Format(g.PerMonthCents), monthLabel(g.Deadline), money.Format(g.RemainingCents)))
		default:
			lines = append(lines, fmt.Sprintf("%s: %d%% guardado, faltam %s.", g.Name, g.Pct, money.Format(g.RemainingCents)))
		}
	}
	return lines
}

func monthLabel(key string) string {
	t, err := time.Parse("2006-01", key)
	if err != nil {
		return key
	}
	return domain.NewMonth(t.Year(), t.Month()).Label()
}

// spendingLines: categorias acima da média, assinaturas por ano e a maior economia possível.
func spendingLines(l domain.Ledger, today time.Time) []string {
	in := Insights(l, today)
	var lines []string
	for _, c := range in.Categories {
		if c.Avg3Cents > 0 && c.DiffPct >= overAvgPct && c.Cents-c.Avg3Cents >= overAvgMinCents {
			lines = append(lines, fmt.Sprintf("%s: %s este mês, %d%% acima da média (%s).", c.Name, money.Format(c.Cents), c.DiffPct, money.Format(c.Avg3Cents)))
			break // a maior; o resto está na Análise
		}
	}
	// Só assinaturas de desejos (streaming, apps), como no Diagnose: aluguel e contas da casa não contam.
	var n int
	var year int64
	for _, s := range in.Subscriptions {
		if s.Bucket == domain.BucketDesejos {
			n++
			year += s.YearlyCents
		}
	}
	if n > 0 {
		lines = append(lines, fmt.Sprintf("%d assinatura%s: %s por ano.", n, plural(n), money.Format(year)))
	}
	if len(in.Savings) > 0 && in.Savings[0].MonthlyCents > 0 {
		s := in.Savings[0]
		lines = append(lines, fmt.Sprintf("%s: dá para guardar %s por mês.", s.Title, money.Format(s.MonthlyCents)))
	}
	return lines
}

// Meses escreve uma quantidade de meses no singular ou no plural ("1 mês", "7 meses").
func Meses(n int) string {
	if n == 1 {
		return "1 mês"
	}
	return fmt.Sprintf("%d meses", n)
}

// DefaultSuggestions são as perguntas de cada personagem quando os dados não pedem
// outra. As que têm resposta pronta, sem IA, estão em advisor.prepared.
var DefaultSuggestions = map[string][]string{
	"otavio": {"Como foi meu mês?", "Qual dívida devo pagar primeiro?", "O que vence esta semana?"},
	"lia":    {"Onde estou gastando demais este mês?", "Quanto ainda posso gastar?", "Como reduzir meus gastos?"},
	"tito":   {"Minhas metas estão no ritmo?", "Quanto guardar por mês para a reserva?", "Como chego na minha meta mais rápido?"},
}

const maxSuggestions = 3

// StartQuestion abre o começo do zero com o Otávio (advisor: onboardingPrompt).
const StartQuestion = "Me ajuda a começar do zero?"

// Suggestions são as perguntas sugeridas na tela do Consultor: primeiro as que os
// dados pedem (a dívida mais cara, a categoria que disparou, a meta atrasada),
// depois as de sempre, sem as que não fazem sentido (dívida sem dívidas).
func Suggestions(l domain.Ledger, today time.Time, agent string) []string {
	id := agentByID(agent).ID
	var first []string
	skip := map[string]bool{}
	switch id {
	case "otavio":
		if len(l.ActiveTransactions())+len(l.ActiveRecurring())+len(l.ActiveDebts())+len(l.ActiveGoals()) == 0 {
			first = append(first, StartQuestion) // nada cadastrado: ele ajuda a montar o mês
		}
		if c := Diagnose(l, today).Costliest; c != nil && c.MonthlyInterestCents > 0 {
			first = append(first, fmt.Sprintf("Como saio da dívida %s mais rápido?", c.Name))
		}
		if len(debtLines(l, today)) == 0 {
			skip["Qual dívida devo pagar primeiro?"] = true
		}
	case "lia":
		for _, c := range Insights(l, today).Categories {
			if c.Avg3Cents > 0 && c.DiffPct >= overAvgPct && c.Cents-c.Avg3Cents >= overAvgMinCents {
				first = append(first, fmt.Sprintf("Por que gastei mais com %s este mês?", c.Name))
				break
			}
		}
	case "tito":
		goals := 0
		for _, g := range Goals(l, today) {
			if g.Done {
				continue
			}
			goals++
			if g.Late && len(first) == 0 {
				first = append(first, fmt.Sprintf("Como recupero o atraso da meta %s?", g.Name))
			}
		}
		if goals == 0 {
			skip["Minhas metas estão no ritmo?"], skip["Como chego na minha meta mais rápido?"] = true, true
		}
	}
	extra := map[string]string{"otavio": "Qual é o meu maior problema agora?", "tito": "Como começo a guardar dinheiro?"}[id]
	out := first
	for _, q := range append(DefaultSuggestions[id], extra) {
		if q != "" && !skip[q] && !slices.Contains(out, q) && len(out) < maxSuggestions {
			out = append(out, q)
		}
	}
	return out
}
