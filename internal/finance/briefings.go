package finance

import (
	"fmt"
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
}

const maxBriefingLines = 3

// Briefings junta o que Bento (dívidas), Tito (metas) e Otávio (gastos) têm
// a dizer hoje. A Luna fala pelos avisos (Alerts). Quem não tem nada fica de fora.
func Briefings(l domain.Ledger, today time.Time) []Briefing {
	out := []Briefing{}
	add := func(agent, title, link string, lines []string) {
		if len(lines) > maxBriefingLines {
			lines = lines[:maxBriefingLines]
		}
		if len(lines) > 0 {
			out = append(out, Briefing{agentByID(agent), title, lines, link})
		}
	}
	add("bento", "Plano das dívidas", "/dividas", debtLines(l, today))
	add("tito", "Suas metas", "/metas", goalLines(l, today))
	add("otavio", "Achados do mês", "/analise", spendingLines(l, today))
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
