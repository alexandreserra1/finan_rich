package advisor

import (
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestSnapshotSeparaOrcamentoDaQuitacao(test *testing.T) {
	book := domain.Ledger{
		Categories: domain.DefaultCategories(1),
		Transactions: []domain.Transaction{
			{Kind: domain.KindEntrada, AmountCents: 360000, CategoryID: "padrao-salario", Date: "2026-10-05"},
			{Kind: domain.KindSaida, AmountCents: 43990, CategoryID: "padrao-mercado", Date: "2026-10-08"},
		},
		Debts: []domain.Debt{{Name: "Empréstimo", BalanceCents: 240000, MonthlyRateBP: 900, MinPaymentCents: 30000}},
		Recurring: []domain.Recurring{{Meta: domain.Meta{ID: "fixa"}, Kind: domain.KindSaida,
			Description: "Internet", AmountCents: 9000, Day: 5, Start: "2026-01"}},
	}
	facts := BuildFacts(book, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
	if facts.Plan.FreeCents != 277010 || facts.Plan.TargetPaymentCents != 261600 {
		test.Fatalf("plano não reservou fixos ou excedeu a quitação estimada: %+v", facts.Plan)
	}
	snapshot := Snapshot(facts)
	if strings.Contains(snapshot, "mandar toda a sobra livre") || !strings.Contains(snapshot, "não é o valor de quitação") {
		test.Fatal("contexto não distingue orçamento de quitação:", snapshot)
	}
}

func TestSimulacoesExplicamHipoteses(test *testing.T) {
	tools := NewTools(teamLedger(), teamDay, nil)
	debt := tools.Run(call("simular_dividas", `{"valor_extra":"200"}`))
	for _, required := range []string{"taxas constantes", "próximo mês", "limites legais"} {
		if !strings.Contains(debt, required) {
			test.Error("simulação da dívida sem hipótese:", required)
		}
	}
	goal := tools.Run(call("simular_meta", `{"meta":"Viagem","valor_mes":"350"}`))
	if !strings.Contains(goal, "a partir deste mês") || !strings.Contains(goal, "março de 2028 (18 aportes)") {
		test.Fatal("meta não explica quando começa:", goal)
	}
	if strings.Contains(knowledgeFor("Como quitar minha dívida?"), "cresce para sempre") {
		test.Fatal("conhecimento contém previsão absoluta de crescimento")
	}
}

// A resposta sem IA mostra a simulação para a pessoa: a conta vem primeiro e a
// ressalva, curta, no fim (revisão do #52).
func TestPlanoDasDividasComecaPelaConta(test *testing.T) {
	q := offlineQ("otavio", "Quanto adianta pagar R$ 200 a mais por mês?")
	got := appAnswer(q)
	if !strings.HasPrefix(got, "**Plano das dívidas**\n\n- Ordem (") || !strings.HasSuffix(got, "Para quitar, confirme o valor com o credor.") {
		test.Errorf("plano das dívidas:\n%s", got)
	}
}

// Com histórico, o plano reserva o que os meses anteriores gastaram depois de hoje antes
// do extra para a dívida (revisão do corpus, 26/11/2026: o pagamento levava 100% da sobra).
func TestPlanoReservaGastosDoRestoDoMes(test *testing.T) {
	m := func(id string) domain.Meta { return domain.Meta{ID: id, UpdatedAt: 1} }
	l := domain.Ledger{
		Categories: []domain.Category{{Meta: m("sal"), Name: "Salário", Bucket: domain.BucketRenda}, {Meta: m("mer"), Name: "Mercado", Bucket: domain.BucketNecessidades}},
		Debts:      []domain.Debt{{Meta: m("emp"), Name: "Empréstimo", BalanceCents: 600000, MonthlyRateBP: 900, MinPaymentCents: 30000}},
	}
	tx := func(id, kind string, cents int64, cat, date string) {
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: m(id), Kind: kind, Description: id, AmountCents: cents, CategoryID: cat, Date: date})
	}
	tx("sal-set", domain.KindEntrada, 385000, "sal", "2026-09-05")
	tx("mer-set-1", domain.KindSaida, 40000, "mer", "2026-09-08")
	tx("mer-set-2", domain.KindSaida, 60000, "mer", "2026-09-27") // depois do dia 22: o resto do mês costuma ter R$ 600
	tx("sal-out", domain.KindEntrada, 385000, "sal", "2026-10-05")
	tx("mer-out-1", domain.KindSaida, 40000, "mer", "2026-10-08")
	got := Snapshot(BuildFacts(l, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)))
	for _, want := range []string{
		"- Gastos do dia a dia que ainda devem vir até o fim do mês (pelos meses anteriores): R$ 600,00",
		"- Sobra livre depois dos fixos, das parcelas mínimas e dos gastos que ainda vêm: R$ 3.450,00 - R$ 0,00 - R$ 300,00 - R$ 600,00 = R$ 2.550,00",
	} {
		if !strings.Contains(got, want) {
			test.Errorf("sem %q:\n%s", want, got)
		}
	}
}
