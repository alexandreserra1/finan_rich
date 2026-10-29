package advisor

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// Suggestions são as perguntas sugeridas de cada personagem (a tela do
// computador usa estas; o app tem a mesma lista em js/pages/advisor.js).
var Suggestions = map[string][]string{
	"jade":   {"O que eu faço com o meu dinheiro este mês?", "Qual é o meu maior problema agora?", "Me ajuda a montar um plano para o mês?"},
	"bento":  {"Qual dívida devo pagar primeiro?", "Quando eu fico livre das dívidas?", "Quanto adianta pagar R$ 200 a mais por mês?"},
	"luna":   {"O que vence esta semana?", "Algum orçamento perto de estourar?", "Quanto é a fatura do cartão?"},
	"tito":   {"Como chego na minha meta mais rápido?", "Quanto guardar por mês para a reserva?", "Minhas metas estão no ritmo?"},
	"otavio": {"Onde estou gastando demais este mês?", "Quanto gastei com iFood este ano?", "Tenho assinatura esquecida?"},
}

// prepared: as sugestões que o Go responde sozinho, por personagem. As outras
// pedem conversa e vão para a IA.
var prepared = map[string][]string{
	"jade":   {"Qual é o meu maior problema agora?"},
	"bento":  {"Qual dívida devo pagar primeiro?", "Quando eu fico livre das dívidas?", "Quanto adianta pagar R$ 200 a mais por mês?"},
	"luna":   {"O que vence esta semana?"},
	"tito":   {"Quanto guardar por mês para a reserva?", "Minhas metas estão no ritmo?"},
	"otavio": {"Onde estou gastando demais este mês?", "Tenho assinatura esquecida?"},
}

// Prepared diz se a pergunta tem resposta pronta do app (sem IA) para este personagem.
func Prepared(q Question) bool {
	text := fold(q.Text)
	for _, p := range prepared[persona(q.Agent).ID] {
		if fold(p) == text {
			return true
		}
	}
	return false
}

// OverLimit diz se a IA recusou por limite de uso (HTTP 429).
func OverLimit(err error) bool {
	var apiErr *llm.APIError
	return errors.As(err, &apiErr) && apiErr.Status == 429
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

// Offline é a resposta do personagem só com as contas do app.
func Offline(q Question) string {
	t := NewTools(q.Ledger, q.Today, nil)
	switch id := persona(q.Agent).ID; id {
	case "bento":
		return "**Plano das dívidas**\n\n" + bullets(prefetch(id, t, q.Text))
	case "luna":
		return bullets(prefetch(id, t, q.Text))
	case "tito":
		return titoOffline(t, q)
	case "otavio":
		return otavioOffline(q)
	}
	return jadeOffline(q)
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
	if e := BuildFacts(q.Ledger, q.Today).Emergency; e.AvgMonthlyOutCents > 0 && (goals == "" || strings.Contains(fold(q.Text), "reserva")) {
		lines := []string{fmt.Sprintf("- Mínima: %s (3 meses de gastos); ideal: %s (6 meses).", money.Format(e.MinCents), money.Format(e.IdealCents))}
		if e.MonthsToMin > 0 {
			lines = append(lines, fmt.Sprintf("- Guardando a sobra livre (%s por mês), chega na mínima em %s e na ideal em %s.", money.Format(e.MonthlySavingCents), finance.Meses(e.MonthsToMin), finance.Meses(e.MonthsToIdeal)))
		} else {
			lines = append(lines, "- Hoje não sobra dinheiro no mês: sem cortar gastos, não dá para formar a reserva.")
		}
		parts = append(parts, "**Reserva de emergência**\n\n"+strings.Join(lines, "\n"))
	}
	if len(parts) == 0 {
		return "Nenhuma meta em andamento. Crie uma na tela Metas e eu digo quanto guardar por mês."
	}
	return strings.Join(parts, "\n\n")
}

// maxFindings: o Otávio traz até 3 achados.
const maxFindings = 3

func otavioOffline(q Question) string {
	in := finance.Insights(q.Ledger, q.Today)
	var lines []string
	if strings.Contains(fold(q.Text), "assinatura") {
		for _, s := range in.Subscriptions {
			lines = append(lines, fmt.Sprintf("%s: %s por mês, %s por ano.", s.Name, money.Format(s.MonthlyCents), money.Format(s.YearlyCents)))
		}
		if len(lines) == 0 {
			return "Não achei nada cobrado todo mês nos seus lançamentos."
		}
		return "**Cobranças todo mês**\n\n" + bullets(strings.Join(lines, "\n"))
	}
	if in.Day > 0 {
		lines = append(lines, fmt.Sprintf("Até o dia %d: %s em gastos variáveis; no ritmo de agora, o mês fecha em %s (mês passado: %s).",
			in.Day, money.Format(in.VariableCents), money.Format(in.ProjectedCents), money.Format(in.LastMonthCents)))
	}
	for _, b := range finance.Briefings(q.Ledger, q.Today) {
		if b.Agent.ID == "otavio" {
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

func jadeOffline(q Question) string {
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
		if id != "" && id != "jade" {
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
