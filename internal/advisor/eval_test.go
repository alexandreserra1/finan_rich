package advisor

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// Avaliação do consultor com uma IA de verdade: perguntas típicas sobre a
// situação de uma pessoa endividada, cada uma com critérios automáticos (usa
// os números dela? segue a ordem de prioridades? não inventa taxa? não indica
// ação?). Fica desligada; para rodar (gasta cota do provedor):
//
//	FINAN_EVAL_PRESET=gemini FINAN_EVAL_KEY=... [FINAN_EVAL_MODEL=...] go test ./internal/advisor -run TestEval -v
//
// FINAN_EVAL_LOCAL=1 manda sem apelidos (como uma IA local).

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

// Todas as respostas (menos a piada) também precisam começar pela conclusão
// em negrito e terminar com um próximo passo no app: ver personal abaixo.
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

// personal: formato pedido nas instruções (conclusão primeiro, próximo passo no app).
var personal = []string{`^\s*\*\*`, `no app|app:|registr|crie|cadastr`}

func TestEval(t *testing.T) {
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
	today := time.Date(2026, 10, 22, 12, 0, 0, 0, time.Local)
	passed, total := 0, 0
	for _, c := range evalCases {
		var answer string
		for try := 0; try < 3; try++ { // cota grátis: espera e tenta de novo
			answer, _, err = Answer(context.Background(), p, Question{Text: c.q, Ledger: evalLedger(), Today: today, Anonymize: os.Getenv("FINAN_EVAL_LOCAL") == ""},
				func(string) {}, func(Proposal) {})
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
		if !strings.Contains(c.q, "piada") {
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
		t.Logf("%s %q %v\n%s\n", status, c.q, fails, answer)
	}
	t.Logf("NOTA: %d de %d critérios (%d%%)", passed, total, passed*100/max(total, 1))
}
