package finance

import (
	"slices"
	"strings"
	"unicode"

	"github.com/alexandreserra1/finan_rich/internal/importer"
)

// Agent é um personagem do time da Rich. Cada um cuida de um assunto e assina
// os avisos dele; o retrato fica em static/img/time/<ID>.webp (e <ID>-mini.webp,
// o rosto pequeno).
type Agent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Team: Otávio organiza o mês e as dívidas (e conversa sobre tudo), Lia cuida
// dos hábitos e do orçamento, Tito das metas e da reserva. O primeiro é o padrão:
// id desconhecido ou de quem saiu do time (Jade, Bento, Luna) vira o Otávio.
var Team = []Agent{
	{"otavio", "Otávio", "Organização e dívidas"},
	{"lia", "Lia", "Hábitos e orçamento"},
	{"tito", "Tito", "Metas e reserva"},
}

// agentKinds diz quem avisa cada tipo de alerta. Otávio cuida do que vence e
// das dívidas; Lia do orçamento, da divisão do salário e do gasto fora do padrão;
// Tito das metas.
var agentKinds = map[string]string{
	"debt-due": "otavio", "debt-growing": "otavio",
	"bucket-over": "lia", "budget-near": "lia", "budget-over": "lia", "unusual": "lia",
	"goal-late": "tito", "goal-done": "tito",
}

// agentFor devolve o personagem do tipo de alerta; tipo desconhecido fica com o Otávio.
func agentFor(kind string) Agent {
	id := agentKinds[kind]
	for _, a := range Team {
		if a.ID == id {
			return a
		}
	}
	return Team[0]
}

// routes: palavras que indicam o assunto de cada personagem, na ordem de
// prioridade (dívida e vencimento antes de tudo). "x*" vale para palavras que começam com x.
var routes = []struct {
	agent string
	words []string
}{
	{"otavio", []string{"divida*", "juros", "rotativo", "cheque especial", "emprestimo*", "financiamento*", "quitar", "devendo", "negociar",
		"vence", "vencimento*", "fatura*", "boleto*", "atrasad*", "lembra*", "lembrete*"}},
	{"tito", []string{"meta", "metas", "reserva", "guardar", "juntar", "poupar", "poupanca", "aposentadoria", "investir"}},
	{"lia", []string{"orcamento*", "teto", "gastei", "gasto*", "gastando", "assinatura*", "categoria*", "mes passado", "relatorio",
		"comparar", "analise", "reduzir", "economizar", "cortar"}},
}

// Route diz quem do time cuida do assunto da pergunta ("" se nenhum em
// especial). A tela usa para sugerir trocar de personagem.
func Route(question string) string {
	words := strings.FieldsFunc(importer.Normalize(question), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	text := " " + strings.Join(words, " ") + " "
	for _, r := range routes {
		for _, w := range r.words {
			if stem, ok := strings.CutSuffix(w, "*"); ok {
				if slices.ContainsFunc(words, func(x string) bool { return strings.HasPrefix(x, stem) }) {
					return r.agent
				}
			} else if strings.Contains(text, " "+w+" ") {
				return r.agent
			}
		}
	}
	return ""
}
