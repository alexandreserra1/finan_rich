// Package advisor monta a conversa do consultor financeiro: as instruções, o
// retrato atual das finanças e o histórico. Não depende de nenhum provedor de
// IA específico; quem envia é um llm.Provider.
package advisor

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// SystemPrompt são as instruções fixas do consultor. Mantenha-o estável: alguns
// provedores reaproveitam o processamento do início da conversa (cache).
// PromptMarker abre as instruções do consultor. O intermediário grátis
// (internal/proxy) reconhece o app por ela; mude o resto à vontade, ela não.
const PromptMarker = "Você é o consultor financeiro pessoal do usuário dentro do app finan_rich"

const SystemPrompt = PromptMarker + `, um controle de gastos individual.

Como agir:
- Responda sempre em português do Brasil, de forma direta e prática, como um amigo que entende muito de finanças pessoais no Brasil.
- A última mensagem do usuário traz <dados_financeiros> com os números atuais do app (mês, divisão do salário, dívidas, metas, ritmo do mês, onde economizar). Os valores já foram calculados e estão corretos: use-os como estão. Se precisar de uma conta nova, faça uma conta simples e mostre-a (ex.: "R$ 4.500 - R$ 3.550 = R$ 950").
- Nunca invente botões, telas ou recursos. As telas do app são: Painel, Lançamentos, Contas, Dívidas, Metas, Análise, Consultor e Ajustes. Se houver "Contas prontas para esta pergunta" nos dados, os números estão lá: use-os em vez de mandar a pessoa calcular.
- Nunca invente números que não estão nos dados ou na conversa. Se faltar dado, diga o que falta e peça para o usuário lançar no app.
- Personalize sempre: use pelo menos dois números do usuário e os nomes das dívidas, metas e lugares dele. Nada de conselho genérico que serviria para qualquer pessoa.
- Pergunta vaga ("o que eu faço?", "me ajuda"): olhe os pontos de atenção e a análise do mês e ataque o maior problema primeiro, seguindo a ordem de prioridades (contas em dia → rotativo e cheque especial → reserva inicial → outras dívidas caras → reserva completa → metas e investimentos).
- Formato: comece com a conclusão em uma frase em **negrito**; depois o porquê com os números dele; depois um plano em passos numerados com valor em R$ e prazo; termine com um próximo passo que dá para fazer hoje no app (registrar um pagamento, criar uma meta, cortar uma assinatura).
- O bloco <conhecimento> traz regras e boas práticas de finanças no Brasil: aplique ao caso do usuário, não copie.
- Taxas do momento (Selic, CDI, inflação, juros de um banco) mudam: se não estiverem nos dados, não invente; diga para conferir a taxa do dia.
- Você não é consultor de investimentos certificado: explique critérios (risco, prazo, liquidez, custo) sem indicar ação, fundo, cripto ou banco específico, e sem prometer retorno. Investimento de risco só depois de dívidas caras quitadas e reserva montada.
- Para perguntas sobre um estabelecimento ou serviço (ex.: "quanto gastei com iFood?"), use a lista de gastos por estabelecimento.
- Para "onde economizar", passe por todos os itens de "Onde economizar" dos dados (inclusive as assinaturas), do que economiza mais para o que economiza menos.
- Se a pergunta não for de finanças, responda em uma frase e ofereça ajuda com as finanças dele.
- Seja objetivo: até umas 250 palavras. Use markdown simples (listas e **negrito**).

Exemplo do estilo (os números são de outra pessoa):
Pergunta: "o que eu faço com meu dinheiro?"
Resposta: "**Primeiro, tire a Dívida 1 (cartão) do vermelho: ela cresce todo mês.** Os juros são 14% ao mês (R$ 448 em outubro) e a parcela mínima de R$ 320 nem cobre os juros.
1. Este mês: use R$ 1.000 dos R$ 1.689 que sobram para abater o cartão.
2. Corte o delivery da Loja AB pela metade (R$ 342 por mês, segundo a análise) e mande para o cartão.
3. Quitado o cartão, junte 1 mês de gastos essenciais como reserva.
Hoje no app: registre o pagamento de R$ 1.000 na dívida."`

// maxHistory limita quantas mensagens anteriores vão junto. Modelos locais têm
// contexto pequeno; 12 mensagens = as últimas 6 perguntas e respostas.
const maxHistory = 12

// BuildRequest monta a requisição: histórico recente + pergunta nova com o
// retrato financeiro atual. Só a última pergunta leva o retrato, para não
// repetir dados velhos e economizar contexto.
func BuildRequest(history []llm.Message, question, snapshot string) llm.Request {
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	// O histórico precisa começar com uma pergunta do usuário.
	for len(history) > 0 && history[0].Role != llm.RoleUser {
		history = history[1:]
	}
	msgs := make([]llm.Message, 0, len(history)+1)
	for _, h := range history {
		msgs = append(msgs, llm.Message{Role: llm.Role(h.Role), Content: h.Content})
	}
	msgs = append(msgs, llm.Message{
		Role: llm.RoleUser,
		// O conhecimento vem antes dos dados: assim a pergunta fica no fim, sozinha.
		Content: "<conhecimento>\n" + knowledgeFor(question) + "\n</conhecimento>\n\n<dados_financeiros>\n" + snapshot + "\n</dados_financeiros>\n\n" + question,
	})
	return llm.Request{System: SystemPrompt, Messages: msgs}
}

// maxToolRounds limita as idas e voltas de ferramentas numa pergunta.
const maxToolRounds = 4

// Ask envia a pergunta ao provedor e devolve a resposta completa; onText recebe
// os pedaços conforme chegam. Com tools, a IA pode buscar lançamentos e propor
// lançamentos; se o modelo não aceitar ferramentas, pergunta sem elas.
func Ask(ctx context.Context, p llm.Provider, agent string, history []llm.Message, question, snapshot string, tools *Tools, onText func(string)) (string, error) {
	req := BuildRequest(history, question, snapshot)
	req.System = systemFor(agent)
	if tools != nil {
		tools.only = persona(agent).Tools
		req.System += toolsPrompt
		req.Tools = tools.Defs()
	}
	var answer strings.Builder
	for round := 1; ; round++ {
		if round > maxToolRounds && req.Tools != nil { // chega de ferramentas: agora responde
			req.Tools, req.Messages = nil, withoutTools(req.Messages)
		}
		reply, err := p.Chat(ctx, req, onText)
		if err != nil && req.Tools != nil && llm.ToolsUnsupported(err) {
			req.System, req.Tools, req.Messages = systemFor(agent), nil, withoutTools(req.Messages)
			continue
		}
		if err != nil {
			return "", err
		}
		answer.WriteString(reply.Text)
		if len(reply.ToolCalls) == 0 || req.Tools == nil {
			return strings.TrimSpace(answer.String()), nil
		}
		if reply.Text != "" {
			answer.WriteString("\n\n")
			onText("\n\n")
		}
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleAssistant, Content: reply.Text, ToolCalls: reply.ToolCalls})
		for _, c := range reply.ToolCalls {
			req.Messages = append(req.Messages, llm.Message{Role: llm.RoleTool, ToolCallID: c.ID, ToolName: c.Name, Content: tools.Run(c)})
		}
	}
}

// withoutTools troca os pedidos e resultados de ferramentas por texto: pedido
// sem "tools" mas com blocos de ferramenta é recusado (Anthropic, por exemplo).
func withoutTools(msgs []llm.Message) []llm.Message {
	var out []llm.Message
	var results []string
	flush := func() {
		if len(results) > 0 {
			out = append(out, llm.Message{Role: llm.RoleUser, Content: "Resultados das consultas aos dados:\n" + strings.Join(results, "\n")})
			results = nil
		}
	}
	for _, m := range msgs {
		switch {
		case m.Role == llm.RoleTool:
			results = append(results, "- "+m.ToolName+": "+m.Content)
		case len(m.ToolCalls) > 0:
			flush()
			if m.Content != "" {
				out = append(out, llm.Message{Role: llm.RoleAssistant, Content: m.Content})
			}
		default:
			flush()
			out = append(out, m)
		}
	}
	flush()
	return out
}

// Question é uma pergunta ao consultor com tudo que ele precisa.
type Question struct {
	History   []llm.Message
	Text      string
	Ledger    domain.Ledger
	Today     time.Time
	Anonymize bool   // IA de fora do aparelho: nomes viram apelidos (ver anon.go)
	Agent     string // quem do time responde (personas.go); vazio = Jade
}

// Answer responde à pergunta com as ferramentas ligadas. Com Anonymize, a IA
// só vê apelidos e a resposta chega inteira no fim (já com os nomes de volta),
// porque um apelido pode vir partido entre dois pedaços do streaming.
//
// Pergunta pronta (Prepared) é respondida pelo app, sem chamar a IA. Se a IA
// falhar por qualquer motivo, a resposta é a do app (Fallback) com o erro em
// AIError. Nesses dois casos (App), onText não recebe o texto: quem chama
// mostra Reply.Text. O erro só volta quando a pessoa cancelou.
func Answer(ctx context.Context, p llm.Provider, q Question, onText func(string), onProposal func(Proposal)) (Reply, error) {
	if Prepared(q) {
		return Reply{Text: Offline(q), App: true}, nil
	}
	ledger, history, text, emit := q.Ledger, q.History, q.Text, onText
	var anon *Anonymizer
	if q.Anonymize {
		ledger, anon = Anonymize(q.Ledger)
		history = make([]llm.Message, len(q.History))
		for i, m := range q.History {
			m.Content = anon.Text(m.Content)
			history[i] = m
		}
		text, emit = anon.Text(q.Text), func(string) {}
	}
	var proposals []Proposal
	tools := NewTools(ledger, q.Today, func(pr Proposal) {
		if anon != nil {
			pr = anon.RevealProposal(pr)
		}
		proposals = append(proposals, pr)
		onProposal(pr)
	})
	answer, err := Ask(ctx, p, q.Agent, history, text, withPrefetch(Snapshot(BuildFacts(ledger, q.Today)), q.Agent, tools, text), tools, emit)
	if err == nil && answer == "" {
		err = errors.New("a IA não devolveu resposta")
	}
	if err != nil {
		return answerOrFallback(ctx, q, err)
	}
	if anon != nil {
		answer = anon.Reveal(answer)
		onText(answer)
	}
	return Reply{Text: answer, Proposals: proposals}, nil
}

// Na IA dentro do celular (llama.cpp em WebAssembly, 1 thread), cada palavra
// do pedido custa tempo: vai só a última pergunta e resposta e os maiores
// estabelecimentos.
const (
	maxLocalHistory   = 2
	maxLocalMerchants = 10
)

// LocalRequest monta o pedido para a IA que roda dentro do celular: sem
// ferramentas (modelos minúsculos não sabem usá-las; a resposta é só texto),
// sem apelidos (nada sai do aparelho) e com histórico curto.
func LocalRequest(q Question) llm.Request {
	history := q.History
	if len(history) > maxLocalHistory {
		history = history[len(history)-maxLocalHistory:]
	}
	f := BuildFacts(q.Ledger, q.Today)
	if len(f.Merchants) > maxLocalMerchants {
		f.Merchants = f.Merchants[:maxLocalMerchants]
	}
	// Sem ferramentas aqui: a conta pronta do personagem é tudo que a IA pequena terá.
	r := BuildRequest(history, q.Text, withPrefetch(Snapshot(f), q.Agent, NewTools(q.Ledger, q.Today, nil), q.Text))
	r.System = systemFor(q.Agent)
	return r
}
