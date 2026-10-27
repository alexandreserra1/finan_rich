package advisor

import (
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// teamLedger: duas dívidas, duas metas, uma conta fixa e gastos de dois meses.
func teamLedger() domain.Ledger {
	l := toolsLedger()
	m := func(id string) domain.Meta { return domain.Meta{ID: id, UpdatedAt: 1} }
	l.Debts = []domain.Debt{
		{Meta: m("d1"), Name: "Cartão", BalanceCents: 100000, MinPaymentCents: 20000, MonthlyRateBP: 1000, DueDay: 20},
		{Meta: m("d2"), Name: "Empréstimo", BalanceCents: 100000, MinPaymentCents: 20000, MonthlyRateBP: 200, DueDay: 5},
	}
	l.Goals = []domain.Goal{
		{Meta: m("g1"), Name: "Viagem", TargetCents: 600000, Deadline: "2027-03", StartDate: "2026-10-01"},
		{Meta: m("g2"), Name: "Notebook", TargetCents: 300000, SavedCents: 300000},
	}
	l.Recurring = []domain.Recurring{
		{Meta: m("r1"), Kind: domain.KindSaida, Description: "Aluguel", AmountCents: 145000, CategoryID: "tra", Day: 25, Start: "2026-01"},
		{Meta: m("r2"), Kind: domain.KindSaida, Description: "Pausado", AmountCents: 1000, CategoryID: "tra", Day: 24, Start: "2026-01", Paused: true},
	}
	l.Transactions = append(l.Transactions,
		domain.Transaction{Meta: m("o1"), Kind: domain.KindSaida, Description: "iFood jantar", AmountCents: 40000, CategoryID: "res", Date: "2026-10-03"},
		domain.Transaction{Meta: m("o2"), Kind: domain.KindSaida, Description: "Uber", AmountCents: 2000, CategoryID: "tra", Date: "2026-10-04"},
	)
	return l
}

var teamDay = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func TestSimulateDebtsTool(t *testing.T) {
	tools := NewTools(teamLedger(), teamDay, nil)
	got := tools.Run(call("simular_dividas", `{"valor_extra":"200"}`))
	for _, want := range []string{
		"Ordem (maior juros primeiro): Cartão → Empréstimo",
		"Pagando R$ 600,00 por mês (mínimos + R$ 200,00 a mais)",
		"Só o mínimo (R$ 400,00 por mês): livre em 7 meses (maio de 2027)",
		"Com R$ 200,00 a mais: livre em 4 meses (fevereiro de 2027)", "sai 3 meses antes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sem %q:\n%s", want, got)
		}
	}
	if got := tools.Run(call("simular_dividas", `{"ordem":"menor saldo"}`)); !strings.Contains(got, "Ordem (menor saldo primeiro)") {
		t.Errorf("bola de neve:\n%s", got)
	}
	if got := NewTools(toolsLedger(), teamDay, nil).Run(call("simular_dividas", `{}`)); !strings.Contains(got, "Nenhuma dívida") {
		t.Errorf("sem dívidas:\n%s", got)
	}
	if got := tools.Run(call("simular_dividas", `{"valor_extra":"abc"}`)); !strings.Contains(got, "não deu") {
		t.Errorf("valor inválido:\n%s", got)
	}
}

func TestSimulateGoalTool(t *testing.T) {
	tools := NewTools(teamLedger(), teamDay, nil)
	for args, want := range map[string]string{
		`{"meta":"viagem"}`:                   "guarde R$ 1.000,00 por mês durante 6 meses",
		`{"meta":"viagem","prazo":"2026-12"}`: "Para chegar até dezembro de 2026: guarde R$ 2.000,00 por mês durante 3 meses",
		`{"meta":"viagem","valor_mes":"500"}`: "Guardando R$ 500,00 por mês, chega em 12 meses (setembro de 2027), 6 meses depois do prazo",
		`{"meta":"viagem","prazo":"2026-01"}`: "esse prazo já passou",
		`{"meta":"notebook"}`:                 "já foi atingida",
		`{"meta":"casa"}`:                     "não deu",
	} {
		if got := tools.Run(call("simular_meta", args)); !strings.Contains(got, want) {
			t.Errorf("%s: sem %q:\n%s", args, want, got)
		}
	}
}

func TestFindingsAndCompareTools(t *testing.T) {
	tools := NewTools(teamLedger(), teamDay, nil)
	got := tools.Run(call("comparar_meses", `{"de":"2026-09","ate":"2026-10"}`))
	for _, want := range []string{"setembro de 2026 → outubro de 2026", "Restaurantes e delivery: R$ 30,00 → R$ 400,00 (+R$ 370,00)"} {
		if !strings.Contains(got, want) {
			t.Errorf("comparar sem %q:\n%s", want, got)
		}
	}
	if got := tools.Run(call("comparar_meses", `{"de":"setembro"}`)); !strings.Contains(got, "não deu") {
		t.Errorf("mês inválido:\n%s", got)
	}
	if got := tools.Run(call("achados", `{}`)); !strings.Contains(got, "outubro de 2026") || !strings.Contains(got, "Maiores gastos") || !strings.Contains(got, "[o1]") {
		t.Errorf("achados:\n%s", got)
	}
}

func TestUpcomingTool(t *testing.T) {
	tools := NewTools(teamLedger(), teamDay, nil)
	got := tools.Run(call("proximos_vencimentos", `{"dias":"10"}`))
	for _, want := range []string{"Próximos 10 dias (até 26/10)", "Cartão: parcela mínima R$ 200,00 em 20/10 (em 4 dias)", "Aluguel: R$ 1.450,00 em 25/10"} {
		if !strings.Contains(got, want) {
			t.Errorf("sem %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Empréstimo") || strings.Contains(got, "Pausado") {
		t.Errorf("fora da janela ou pausado não entra:\n%s", got)
	}
	if got := tools.Run(call("proximos_vencimentos", `{"dias":"2"}`)); !strings.Contains(got, "Nada vence") {
		t.Errorf("janela vazia:\n%s", got)
	}
}

func TestPersonaToolSets(t *testing.T) {
	for agent, want := range map[string]string{
		"bento":  "simular_dividas",
		"tito":   "simular_meta",
		"otavio": "achados",
		"luna":   "proximos_vencimentos",
	} {
		if p := persona(agent); !strings.Contains(strings.Join(p.Tools, ","), want) {
			t.Errorf("%s sem %s: %v", agent, want, p.Tools)
		}
		if !strings.Contains(persona(agent).Voice, want) {
			t.Errorf("a voz de %s não diz quando usar %s", agent, want)
		}
	}
	if persona("jade").Tools != nil {
		t.Error("a Jade usa todas")
	}
	for _, name := range []string{"simular_dividas", "simular_meta", "achados", "comparar_meses", "proximos_vencimentos"} {
		found := false
		for _, d := range allTools() {
			found = found || d.Name == name
		}
		if !found {
			t.Errorf("%s sem definição", name)
		}
	}
}

func TestQuestionMoney(t *testing.T) {
	for q, want := range map[string]string{
		"e se eu pagar R$ 500 a mais por mês?":   "500",
		"pagar 1.200,50 a mais":                  "1.200,50",
		"se eu guardar 400 reais, quando chego?": "400",
		"o que vence nos próximos 20 dias?":      "",
		"juros de 14% ao mês":                    "",
		"oi":                                     "",
	} {
		if got := questionMoney(q); got != want {
			t.Errorf("questionMoney(%q) = %q, quer %q", q, got, want)
		}
	}
}

func TestPrefetch(t *testing.T) {
	tools := NewTools(teamLedger(), teamDay, nil)
	for agent, c := range map[string]struct{ q, want string }{
		"bento":  {"e se eu pagar R$ 200 a mais?", "Com R$ 200,00 a mais: livre em 4 meses"},
		"tito":   {"se eu guardar 500 por mês, quando chego na viagem?", "Guardando R$ 500,00 por mês, chega em 12 meses"},
		"otavio": {"o que mudou?", "setembro de 2026 → outubro de 2026"},
		"luna":   {"o que vence esta semana?", "Próximos 7 dias"},
	} {
		if got := prefetch(agent, tools, c.q); !strings.Contains(got, c.want) {
			t.Errorf("%s: sem %q:\n%s", agent, c.want, got)
		}
	}
	if got := prefetch("jade", tools, "oi"); got != "" {
		t.Errorf("a Jade não pré-calcula: %q", got)
	}
}
