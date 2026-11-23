package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// Avaliação do consultor com uma IA de verdade: perguntas típicas sobre a
// situação de uma pessoa endividada, cada uma com critérios automáticos (usa
// os números dela? segue a ordem de prioridades? não inventa taxa? não indica
// ação?). Fica desligada; para rodar (gasta cota do provedor):
//
//	FINAN_EVAL_PRESET=gemini FINAN_EVAL_KEY=... [FINAN_EVAL_MODEL=...] go test ./internal/advisor -run TestEval -v
//
// FINAN_EVAL_LOCAL=1 manda sem apelidos (como uma IA local); FINAN_EVAL_TEAM=1
// roda só as perguntas de cada personagem do time; FINAN_EVAL_ONLY (regexp) escolhe
// as perguntas. O grátis do finan_rich também serve: FINAN_EVAL_PRESET=finan e,
// como chave, um nome qualquer de aparelho (conta no limite dele).

func evalLedger() domain.Ledger {
	m := func(id string) domain.Meta { return domain.Meta{ID: id, UpdatedAt: 1} }
	l := domain.Ledger{
		Categories: domain.DefaultCategories(1),
		Debts:      []domain.Debt{{Meta: m("d1"), Name: "Cartão Nubank", Creditor: "Nu", BalanceCents: 320000, MonthlyRateBP: 1400, MinPaymentCents: 32000, DueDay: 10}},
		Goals:      []domain.Goal{{Meta: m("g1"), Name: "Viagem", TargetCents: 600000, Deadline: "2027-07", StartDate: "2026-01-01"}},
		Recurring:  []domain.Recurring{{Meta: m("r1"), Kind: domain.KindSaida, Description: "Aluguel", AmountCents: 150000, CategoryID: "padrao-moradia", Day: 5, Start: "2026-01"}},
	}
	tx := func(id, kind, desc string, cents int64, cat, date string) {
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: m(id), Kind: kind, Description: desc, AmountCents: cents, CategoryID: cat, Date: date})
	}
	for _, mk := range []string{"2026-08", "2026-09", "2026-10"} {
		tx("sal"+mk, domain.KindEntrada, "Salário", 450000, "padrao-salario", mk+"-05")
		tx(domain.RecurringTxID("r1", domain.ParseMonth(mk)), domain.KindSaida, "Aluguel", 150000, "padrao-moradia", mk+"-05")
		tx("me"+mk, domain.KindSaida, "Mercado Extra", 70000, "padrao-mercado", mk+"-08")
		tx("nf"+mk, domain.KindSaida, "Netflix", 5590, "padrao-lazer", mk+"-10")
		n := 4
		if mk == "2026-10" {
			n = 12 // delivery disparou em outubro
		}
		for i := 0; i < n; i++ {
			tx(fmt.Sprintf("if%s%d", mk, i), domain.KindSaida, "iFood pedido", 5500, "padrao-restaurantes", fmt.Sprintf("%s-%02d", mk, 1+i*2))
		}
		tx("ub"+mk, domain.KindSaida, "Uber *Trip", 3200, "padrao-transporte", mk+"-15")
	}
	return l
}

type evalCase struct {
	q     string
	must  []string // expressões que precisam aparecer (regexp, sem diferenciar maiúsculas)
	never []string // expressões que não podem aparecer
}

// Os pedidos de conselho também precisam começar pela conclusão em negrito e
// terminar com um próximo passo no app; as perguntas diretas, ser curtas: ver
// personal e direct abaixo.
var evalCases = []evalCase{
	{"o que eu faço com meu dinheiro?", []string{`cart[aã]o|d[ií]vida`, `14\s?%|448`, `\n\s*1\.`, `reserva`}, nil},
	{"onde posso economizar?", []string{`ifood|delivery`, `660|330|R\$\s?\d{3}`, `assinatura|netflix`}, nil},
	{"vale a pena investir em ações agora?", []string{`d[ií]vida|cart[aã]o`, `14\s?%|juros`, `reserva`}, []string{`\b[A-Z]{4}[0-9]{1,2}\b`, `recomendo (comprar|a a[cç][aã]o)`}},
	{"como montar minha reserva de emergência?", []string{`tesouro selic|cdb|liquidez|resgate`, `FGC|liquidez di[aá]ria|resgate (di[aá]rio|no mesmo dia|imediato)|CDI`, `\d\s?(a \d+ )?meses`, `d[ií]vida|cart[aã]o`}, nil},
	{"qual a taxa selic hoje?", []string{`confir|consult|verifi`}, []string{`selic[^.\n]{0,40}\d+[,.]\d+\s?%`}},
	{"quanto gastei com uber esse mês?", []string{`uber`, `32[,.]00`}, nil},
	{"devo pagar só o mínimo do cartão?", []string{`n[aã]o`, `rotativo|juros`, `fatura (inteira|total)|valor total|pagar tudo|(n[aã]o pague|evite)[^.]{0,25}m[ií]nimo`, `rotativo|parcel|100\s?%|14\.690`}, nil},
	{"recebi o 13º de R$ 4.500, o que faço com ele?", []string{`cart[aã]o|d[ií]vida`, `4[.,]500|4,5 mil`, `3\.200|448`, `reserva`}, nil},
	{"me conta uma piada", []string{`finan|dinheiro|gasto|or[cç]amento`}, nil},
	{"quero financiar um carro de R$ 60 mil, o que acha?", []string{`CET`, `entrada|SAC`, `d[ií]vida|cart[aã]o`, `seguro|IPVA|manuten`}, nil},
}

// teamEvalCases: uma pergunta por personagem do time, que só fecha com a
// ferramenta dele (as contas abaixo foram feitas à mão com evalLedger).
var teamEvalCases = []struct {
	agent string
	evalCase
}{
	// R$ 3.200 a 14% pagando 320 + 500 por mês: zera no 7º mês.
	{"otavio", evalCase{"e se eu pagar R$ 500 a mais por mês no cartão?", []string{`7 meses|maio de 2027`, `820|500`, `juros`}, nil}},
	// Faltam R$ 6.000; R$ 400 por mês = 15 meses contando outubro (dezembro de 2027), 5 depois de julho.
	{"tito", evalCase{"se eu guardar R$ 400 por mês, quando chego na viagem?", []string{`15 meses|dezembro de 2027`, `prazo|julho`}, nil}},
	// iFood: 4 pedidos de R$ 55 em setembro, 12 em outubro.
	{"lia", evalCase{"o que mudou do mês passado para este?", []string{`ifood|delivery|restaurante`, `220|440|660`}, nil}},
	// De 22/10 a 11/11: aluguel no dia 5 e o cartão no dia 10.
	{"otavio", evalCase{"o que vence nos próximos 20 dias?", []string{`aluguel`, `cart[aã]o`, `10/11|dia 10`}, []string{`\n\s*3\.`}}},
}

// personal: formato pedido nas instruções para conselho (conclusão primeiro,
// próximo passo no app). Pergunta direta (direct) pede só a resposta, curta.
var (
	personal = []string{`^\s*\*\*`, `no app|app:|registr|crie|cadastr`}
	direct   = regexp.MustCompile(`(?i)quanto gastei|selic|vence`)
)

// evalProvider é a IA de verdade das avaliações (pula o teste sem FINAN_EVAL_KEY).
func evalProvider(t *testing.T) llm.Provider {
	t.Helper()
	key := os.Getenv("FINAN_EVAL_KEY")
	if key == "" {
		t.Skip("avaliação com IA real: defina FINAN_EVAL_PRESET e FINAN_EVAL_KEY")
	}
	preset := llm.PresetByID(os.Getenv("FINAN_EVAL_PRESET"))
	model := os.Getenv("FINAN_EVAL_MODEL")
	if model == "" {
		model = preset.Model
	}
	p, err := llm.New(llm.Config{Kind: preset.Kind, BaseURL: preset.BaseURL, Model: model, APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEval(t *testing.T) {
	p := evalProvider(t)
	var err error
	today := time.Date(2026, 10, 22, 12, 0, 0, 0, time.Local)
	passed, total := 0, 0
	type agentCase struct {
		agent string
		evalCase
	}
	all := []agentCase{}
	if os.Getenv("FINAN_EVAL_TEAM") == "" { // FINAN_EVAL_TEAM=1: só as perguntas do time (gasta menos cota)
		for _, c := range evalCases {
			all = append(all, agentCase{"", c})
		}
	}
	for _, c := range teamEvalCases {
		all = append(all, agentCase{c.agent, c.evalCase})
	}
	only := regexp.MustCompile(`(?i)` + os.Getenv("FINAN_EVAL_ONLY")) // FINAN_EVAL_ONLY="uber|selic": só essas perguntas
	for _, c := range all {
		if !only.MatchString(c.q) {
			continue
		}
		var answer string
		for try := 0; try < 3; try++ { // cota grátis: espera e tenta de novo
			var r Reply
			// AskAI: mede a IA também nas perguntas que o app responde sozinho (TestAppAnswersMeetEvalCriteria).
			r, err = Answer(context.Background(), p, Question{Text: c.q, Ledger: evalLedger(), Today: today, Anonymize: os.Getenv("FINAN_EVAL_LOCAL") == "", Agent: c.agent, AskAI: true},
				func(string) {}, func(Proposal) {})
			answer = r.Text
			if r.AIError != nil { // a avaliação mede a IA: resposta sem IA conta como erro
				err = r.AIError
			}
			if err == nil || !strings.Contains(err.Error(), "limite") && !strings.Contains(err.Error(), "ocupada") {
				break
			}
			time.Sleep(20 * time.Second)
		}
		if err != nil {
			t.Errorf("%q: %v", c.q, err)
			total += len(c.must) + len(c.never)
			continue
		}
		var fails []string
		must := c.must
		switch {
		case strings.Contains(c.q, "piada"):
		case direct.MatchString(c.q): // pergunta direta: resposta curta, sem plano
			total++
			if n := len(strings.Fields(answer)); n <= 90 {
				passed++
			} else {
				fails = append(fails, fmt.Sprintf("pergunta direta com %d palavras", n))
			}
		default:
			must = append(append([]string{}, must...), personal...)
		}
		for _, re := range must {
			total++
			if regexp.MustCompile(`(?i)` + re).MatchString(answer) {
				passed++
			} else {
				fails = append(fails, "faltou /"+re+"/")
			}
		}
		for _, re := range c.never {
			total++
			if !regexp.MustCompile(`(?i)` + re).MatchString(answer) {
				passed++
			} else {
				fails = append(fails, "não podia /"+re+"/")
			}
		}
		status := "OK   "
		if len(fails) > 0 {
			status = "FALHA"
		}
		t.Logf("%s %s%q %v\n%s\n", status, c.agent+" ", c.q, fails, answer)
	}
	t.Logf("NOTA: %d de %d critérios (%d%%)", passed, total, passed*100/max(total, 1))
}

// Começo do zero com IA real: a conversa inteira, como a pessoa faria, confirmando os
// cartões entre um turno e outro. Mede se a IA pergunta uma coisa por vez e prepara os
// cartões certos (conta fixa, meta), sem inventar valor. Gasta uns 10 pedidos de cota:
//
//	FINAN_EVAL_PRESET=finan FINAN_EVAL_KEY=<nome qualquer> go test ./internal/advisor -run TestEvalOnboarding -v
func TestEvalOnboarding(t *testing.T) {
	p := evalProvider(t)
	today := time.Date(2026, 11, 18, 12, 0, 0, 0, time.Local)
	ledger := domain.Ledger{Categories: domain.DefaultCategories(1)}
	var history []llm.Message
	recurring := func(props []Proposal, kind string, cents int64, day int) bool {
		for _, pr := range props {
			var r domain.Recurring
			if pr.Type == domain.TypeRecurring && json.Unmarshal(pr.Record, &r) == nil && r.Kind == kind && r.AmountCents == cents && r.Day == day {
				return true
			}
		}
		return false
	}
	goal := func(props []Proposal, cents int64, deadline string) bool {
		for _, pr := range props {
			var g domain.Goal
			if pr.Type == domain.TypeGoal && json.Unmarshal(pr.Record, &g) == nil && g.TargetCents == cents && g.Deadline == deadline {
				return true
			}
		}
		return false
	}
	ofType := func(props []Proposal, typ string) bool {
		return slices.ContainsFunc(props, func(pr Proposal) bool { return pr.Type == typ })
	}
	asks := func(re string) func(string, []Proposal) bool {
		return func(text string, _ []Proposal) bool { return regexp.MustCompile(`(?i)` + re).MatchString(text) }
	}
	turns := []struct {
		say    string
		checks map[string]func(string, []Proposal) bool
	}{
		{finance.StartQuestion, map[string]func(string, []Proposal) bool{
			"pergunta a renda": asks(`recebe|renda|sal[aá]rio|ganha`),
			"uma pergunta por vez": func(text string, _ []Proposal) bool {
				return !regexp.MustCompile(`(?i)d[ií]vida|\bmeta\b`).MatchString(text)
			},
			"ainda sem cartão": func(_ string, p []Proposal) bool { return len(p) == 0 },
		}},
		{"Recebo 4.500 de salário no dia 5", map[string]func(string, []Proposal) bool{
			"renda fixa de R$ 4.500 no dia 5": func(_ string, p []Proposal) bool { return recurring(p, domain.KindEntrada, 450000, 5) },
			"pergunta as contas":              asks(`conta|aluguel|despesa|gasto fixo`),
		}},
		{"Pago aluguel de 1.500 no dia 10 e internet de 100 no dia 15", map[string]func(string, []Proposal) bool{
			"aluguel de R$ 1.500 no dia 10": func(_ string, p []Proposal) bool { return recurring(p, domain.KindSaida, 150000, 10) },
			"internet de R$ 100 no dia 15":  func(_ string, p []Proposal) bool { return recurring(p, domain.KindSaida, 10000, 15) },
		}},
		{"Não tenho dívidas", map[string]func(string, []Proposal) bool{
			"sem dívida inventada": func(_ string, p []Proposal) bool { return !ofType(p, domain.TypeDebt) },
			"pergunta a meta":      asks(`meta|objetivo|juntar|guardar|sonho`),
		}},
		{"Quero juntar 6 mil para uma viagem até julho de 2027", map[string]func(string, []Proposal) bool{
			"meta de R$ 6.000 até julho de 2027": func(_ string, p []Proposal) bool { return goal(p, 600000, "2027-07") },
		}},
	}
	passed, total := 0, 0
	for _, turn := range turns {
		var r Reply
		var props []Proposal
		var err error
		for try := 0; try < 3; try++ { // cota grátis: espera e tenta de novo
			props = nil
			r, err = Answer(context.Background(), p, Question{History: history, Text: turn.say, Ledger: ledger, Today: today, Anonymize: true},
				func(string) {}, func(pr Proposal) { props = append(props, pr) })
			if r.AIError != nil {
				err = r.AIError
			}
			if err == nil || !strings.Contains(err.Error(), "limite") && !strings.Contains(err.Error(), "ocupada") {
				break
			}
			time.Sleep(20 * time.Second)
		}
		if err != nil {
			t.Fatalf("%q: %v", turn.say, err)
		}
		var fails []string
		for name, ok := range turn.checks {
			total++
			if ok(r.Text, props) {
				passed++
			} else {
				fails = append(fails, name)
			}
		}
		status := "OK    "
		if len(fails) > 0 {
			status = "FALHA "
		}
		var labels []string
		for _, pr := range props {
			labels = append(labels, pr.Label)
		}
		t.Logf("%s %q %v\n  cartões: %v\n  %s", status, turn.say, fails, labels, strings.ReplaceAll(r.Text, "\n", "\n  "))
		// A pessoa confirma os cartões: o próximo turno já vê os dados.
		for _, pr := range props {
			switch pr.Type {
			case domain.TypeRecurring:
				var rec domain.Recurring
				if json.Unmarshal(pr.Record, &rec) == nil {
					ledger.Recurring = append(ledger.Recurring, rec)
				}
			case domain.TypeGoal:
				var g domain.Goal
				if json.Unmarshal(pr.Record, &g) == nil {
					ledger.Goals = append(ledger.Goals, g)
				}
			}
		}
		history = append(history, llm.Message{Role: llm.RoleUser, Content: turn.say}, llm.Message{Role: llm.RoleAssistant, Content: r.Text})
	}
	t.Logf("NOTA: %d de %d critérios (%d%%)", passed, total, passed*100/max(total, 1))
}
