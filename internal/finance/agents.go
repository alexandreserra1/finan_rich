package finance

import (
	"slices"
	"strings"
	"unicode"

	"github.com/alexandreserra1/finan_rich/internal/importer"
)

// Agent é um personagem do time do finan_rich. Cada um cuida de um assunto e
// assina os avisos dele; a imagem fica em static/img/avatar-<Animal>.png.
type Agent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Animal string `json:"animal"`
}

// Team: Jade conversa sobre tudo e encaminha; os outros vigiam um assunto cada.
var Team = []Agent{
	{"jade", "Jade", "Consultora", "pantera"},
	{"bento", "Bento", "Dívidas", "urso"},
	{"luna", "Luna", "Alertas", "raposa"},
	{"tito", "Tito", "Metas", "texugo"},
	{"otavio", "Otávio", "Análise", "coruja"},
}

// agentKinds diz quem avisa cada tipo de alerta. Luna cuida do que tem data ou
// teto (vencimento, orçamento, divisão do salário); Bento, de dívida que cresce;
// Tito, das metas; Otávio, do gasto fora do padrão.
var agentKinds = map[string]string{
	"debt-due": "luna", "bucket-over": "luna", "budget-near": "luna", "budget-over": "luna",
	"debt-growing": "bento",
	"goal-late":    "tito", "goal-done": "tito",
	"unusual": "otavio",
}

// agentFor devolve o personagem do tipo de alerta; tipo desconhecido fica com a Jade.
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
// prioridade (dívida antes de tudo). "x*" vale para palavras que começam com x.
var routes = []struct {
	agent string
	words []string
}{
	{"bento", []string{"divida*", "juros", "rotativo", "cheque especial", "emprestimo*", "financiamento*", "quitar", "devendo", "negociar"}},
	{"tito", []string{"meta", "metas", "reserva", "guardar", "juntar", "poupar", "poupanca", "aposentadoria", "investir"}},
	{"luna", []string{"vence", "vencimento*", "fatura*", "lembra*", "lembrete*", "orcamento*", "teto", "boleto*", "atrasad*"}},
	{"otavio", []string{"gastei", "gasto*", "gastando", "assinatura*", "categoria*", "mes passado", "relatorio", "comparar", "analise"}},
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
