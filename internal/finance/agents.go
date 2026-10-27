package finance

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
