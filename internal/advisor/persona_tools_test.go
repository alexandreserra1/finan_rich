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
		{Meta: m("d2"), Name: "Empréstimo", BalanceCents: 100000, MinPaymentCents: 20000, MonthlyRateBP: 200, DueDay: 28},
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
		`{"meta":"viagem","valor_mes":"500"}`: "Guardando R$ 500,00 por mês a partir deste mês, chega em setembro de 2027 (12 aportes), 6 meses depois do prazo",
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
	// Com vencimento no dia 5 e nenhum pagamento lançado em outubro, a parcela venceu: vem antes de tudo.
	l := teamLedger()
	l.Debts[1].DueDay = 5
	late := "Empréstimo: parcela mínima R$ 200,00 venceu em 05/10 e não há pagamento lançado neste mês (se já pagou, registre o pagamento)"
	if got := NewTools(l, teamDay, nil).Run(call("proximos_vencimentos", `{"dias":"10"}`)); !strings.HasPrefix(got, "Próximos 10 dias (até 26/10):\n- "+late) || strings.Contains(got, "05/11") {
		t.Errorf("sem a parcela vencida no começo:\n%s", got)
	}
	if got := NewTools(l, teamDay, nil).Run(call("proximos_vencimentos", `{"dias":"1"}`)); !strings.HasPrefix(got, "Próximo dia (até 17/10):\n- "+late) {
		t.Errorf("um dia só, no singular:\n%s", got)
	}
	if got := tools.Run(call("proximos_vencimentos", `{"dias":"2"}`)); !strings.Contains(got, "Nada vence") {
		t.Errorf("janela vazia:\n%s", got)
	}
}

func TestPersonaToolSets(t *testing.T) {
	for _, c := range []struct{ agent, want string }{
		{"tito", "simular_meta"}, {"lia", "achados"}, {"lia", "comparar_meses"},
	} {
		if p := persona(c.agent); !strings.Contains(strings.Join(p.Tools, ","), c.want) {
			t.Errorf("%s sem %s: %v", c.agent, c.want, p.Tools)
		}
		if !strings.Contains(persona(c.agent).Voice, c.want) {
			t.Errorf("a voz de %s não diz quando usar %s", c.agent, c.want)
		}
	}
	// O Otávio atende primeiro: a voz diz quando usar as de dívida e vencimento
	// (a lista dele está em TestPersonaToolsFitFreeQuota).
	if !strings.Contains(persona("otavio").Voice, "simular_dividas") || !strings.Contains(persona("otavio").Voice, "proximos_vencimentos") {
		t.Error("a voz do Otávio não explica as ferramentas dele")
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
		// Data não é dinheiro, e o valor com "R$" vale mais que o número do nome
		// (revisão do corpus, 27/11/2026).
		"Confira meus gastos registrados em 2026-10; não autorizei apagar nada.": "",
		"a compra de 08/07/2026 veio duplicada":                                  "",
		"Quero criar a meta Passeio 1 de R$ 2500":                                "2500",
		"Cadastre o contrato Oficina 1: saldo R$ 1700, parcela R$ 100":           "1700",
	} {
		if got := questionMoney(q); got != want {
			t.Errorf("questionMoney(%q) = %q, quer %q", q, got, want)
		}
	}
}

func TestPrefetch(t *testing.T) {
	tools := NewTools(teamLedger(), teamDay, nil)
	for _, c := range []struct{ agent, q, want string }{
		{"otavio", "e se eu pagar R$ 200 a mais?", "Com R$ 200,00 a mais: livre em 4 meses"},
		{"otavio", "o que vence esta semana?", "Próximos 7 dias"},
		{"tito", "se eu guardar 500 por mês, quando chego na viagem?", "Guardando R$ 500,00 por mês a partir deste mês, chega em setembro de 2027 (12 aportes)"},
		{"lia", "o que mudou?", "setembro de 2026 → outubro de 2026"},
		// A IA às vezes dizia "nenhum lançamento de Uber" sem buscar: o total vai pronto.
		{"otavio", "quanto gastei com uber esse mês?", "Gastos com Uber: R$ 20,00 em outubro de 2026 (1 lançamento). Desde o começo: R$ 45,00 (2 lançamentos)."},
	} {
		if got := prefetch(c.agent, tools, c.q); !strings.Contains(got, c.want) {
			t.Errorf("%s: sem %q:\n%s", c.agent, c.want, got)
		}
	}
	// Pedido de cadastro não é simulação: nada de "R$ 1.700,00 a mais" nem de aporte de R$ 1 por mês.
	if got := prefetch("otavio", tools, "Quero cadastrar o contrato novo Oficina 1: saldo R$ 1700, juros 2% ao mês, parcela R$ 100"); got != "" {
		t.Errorf("cadastro de dívida não pré-calcula simulação: %q", got)
	}
	if got := prefetch("tito", tools, "Quero criar a meta Passeio 1 de R$ 2500"); strings.Contains(got, "por mês a partir deste mês") {
		t.Errorf("criar meta não vira aporte por mês: %q", got)
	}
	// "Antes de registrar..." é uma pergunta de cálculo, não o pedido de registrar: o valor é lido
	// (quinta revisão do corpus, 27/11/2026).
	if got := prefetch("otavio", tools, "Antes de registrar pagamento, simule R$ 450 a mais por mês nas duas dívidas."); !strings.Contains(got, "Com R$ 450,00 a mais") {
		t.Errorf("antes de registrar descartou o valor pedido: %q", got)
	}
	if got := prefetch("tito", tools, "Antes de registrar aporte, calcule quando chego na viagem guardando R$ 350 por mês."); !strings.Contains(got, "Guardando R$ 350,00 por mês") {
		t.Errorf("antes de registrar descartou o valor da meta: %q", got)
	}
	if got := otavioTopic("Quero apagar a compra Mercearia em 2026-10"); got == "dividas" {
		t.Errorf("apagar não é pagar: tema %q", got)
	}
	for _, q := range []string{"se eu pagar 300 a mais por mês", "quanto pagarei de juros?", "já paguei a parcela"} {
		if got := otavioTopic(q); got != "dividas" {
			t.Errorf("otavioTopic(%q) = %q, quer dividas", q, got)
		}
	}
	if got := prefetch("otavio", tools, "e se eu pagar R$ 1 a mais?"); !strings.Contains(got, "Diferença: termina no mesmo mês") || strings.Contains(got, "0 meses") {
		t.Errorf("valor a mais que não adianta o mês: %q", got)
	}
	if got := prefetch("otavio", tools, "oi"); got != "" {
		t.Errorf("o mês todo não pré-calcula (o retrato já tem os números): %q", got)
	}
	if got := prefetch("otavio", tools, "quanto gastei com padaria?"); got != "" {
		t.Errorf("lugar que não está nos lançamentos não pré-calcula: %q", got)
	}
	// Com apelidos, a descrição é o apelido e a pergunta também.
	anonLedger, _ := Anonymize(teamLedger())
	anonTools := NewTools(anonLedger, teamDay, nil)
	var alias string
	for _, tx := range anonLedger.Transactions {
		if tx.ID == "o2" {
			alias = tx.Description
		}
	}
	if got := prefetch("otavio", anonTools, "quanto gastei com "+alias+"?"); !strings.Contains(got, "Gastos com "+alias+": R$ 20,00") {
		t.Errorf("com apelido (%s) sem o total:\n%s", alias, got)
	}
	if got := prefetch("otavio", anonTools, "quanto gastei na loja?"); got != "" {
		t.Errorf("\"loja\" sozinha não é um lugar (com apelidos pegaria todos): %q", got)
	}
}

// Gasto por categoria e por mês, pronto antes da IA e na resposta sem IA. A categoria vale
// pelo nome ou por uma palavra marcante dele; o lugar mais longo ("mercado extra") vence.
func TestSpendingAnswers(t *testing.T) {
	l := teamLedger()
	l.Categories = append(l.Categories, domain.Category{Meta: domain.Meta{ID: "casa", UpdatedAt: 1}, Name: "Contas da casa (luz, água, internet)", Bucket: domain.BucketNecessidades})
	l.Transactions = append(l.Transactions,
		domain.Transaction{Meta: domain.Meta{ID: "c1", UpdatedAt: 1}, Kind: domain.KindSaida, Description: "Enel", AmountCents: 18000, CategoryID: "casa", Date: "2026-10-08"},
		domain.Transaction{Meta: domain.Meta{ID: "c2", UpdatedAt: 1}, Kind: domain.KindSaida, Description: "Vivo Fibra", AmountCents: 10000, CategoryID: "casa", Date: "2026-09-08"},
	)
	tools := NewTools(l, teamDay, nil)
	for q, want := range map[string]string{
		"quanto gastei com transporte?":                "Gastos com Transporte: R$ 20,00 em outubro de 2026 (1 lançamento). Em setembro de 2026: R$ 25,00.",
		"quanto gastei com restaurantes em setembro?":  "Gastos com Restaurantes e delivery: R$ 30,00 em setembro de 2026 (1 lançamento). Em agosto de 2026: R$ 0,00.",
		"quanto gastei de água e luz mês passado?":     "Gastos com Contas da casa (luz, água, internet): R$ 100,00 em setembro de 2026 (1 lançamento).",
		"quanto gastei com uber mes passado?":          "Gastos com Uber: R$ 25,00 em setembro de 2026 (1 lançamento). Desde o começo: R$ 45,00 (2 lançamentos).",
		"quanto gastei com delivery em março de 2026?": "Gastos com Restaurantes e delivery: R$ 52,30 em março de 2026 (1 lançamento).",
		"quanto gastei com uber em dezembro?":          "Gastos com Uber: R$ 0,00 em dezembro de 2025 (0 lançamentos).",
	} {
		if got := tools.placeSpending(fold(q)); !strings.Contains(got, want) {
			t.Errorf("%q:\n  veio %q\n  quer %q", q, got, want)
		}
	}
	for _, q := range []string{"quanto gastei?", "como reduzir meus gastos?", "quanto gastei com padaria?"} {
		if got := tools.placeSpending(fold(q)); got != "" {
			t.Errorf("%q não cita lugar nem categoria: %q", q, got)
		}
	}
	// A Lia também recebe a conta pronta, e a resposta sem IA dela responde a pergunta.
	if got := prefetch("lia", tools, "quanto gastei com transporte?"); !strings.HasPrefix(got, "Gastos com Transporte") {
		t.Errorf("lia sem o total:\n%s", got)
	}
	q := offlineQ("lia", "quanto gastei com transporte?")
	q.Ledger = l
	if got := Offline(q); !strings.Contains(got, "Gastos com Transporte: R$ 20,00") {
		t.Errorf("lia sem IA:\n%s", got)
	}
}
