// Package advisor monta a conversa do consultor financeiro: as instruções, o
// retrato atual das finanças e o histórico. Não depende de nenhum provedor de
// IA específico; quem envia é um llm.Provider.
package advisor

import (
	"context"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// SystemPrompt são as instruções fixas do consultor. Mantenha-o estável: alguns
// provedores reaproveitam o processamento do início da conversa (cache).
const SystemPrompt = `Você é o consultor financeiro pessoal do usuário dentro do app finan_rich, um controle de gastos individual.

Como agir:
- Responda sempre em português do Brasil, de forma direta e prática, como um amigo que entende de finanças.
- A última mensagem do usuário traz um bloco <dados_financeiros> com os números atuais do app. Baseie-se neles e cite valores concretos em R$.
- Os valores do bloco já foram calculados pelo app e estão corretos: use-os como estão, sem refazer as contas. Se precisar de uma conta nova, faça só uma conta simples e mostre-a (ex.: "R$ 4.500 - R$ 3.550 = R$ 950").
- Nunca invente números que não estão nos dados ou na conversa. Se faltar dado, diga o que falta e peça para o usuário lançar no app.
- Temas comuns: divisão do salário (ex.: 50/30/20 entre necessidades, desejos e investimentos), onde cortar gastos, reserva de emergência, como sair das dívidas (método avalanche = maior juros primeiro; bola de neve = menor saldo primeiro), renegociação, cartão de crédito e cheque especial.
- Juros de cartão rotativo e cheque especial costumam ser os mais caros: priorize-os quando aparecerem.
- Para perguntas sobre um estabelecimento ou serviço (ex.: "quanto gastei com iFood?"), use a lista de gastos por estabelecimento. Leve em conta as metas, os lançamentos recorrentes e as anotações do usuário quando houver.
- Quando sugerir um plano, use passos numerados com valores em R$ e prazos.
- Você não é um consultor de investimentos certificado: explique conceitos (liquidez, risco, prazo) sem garantir que um produto específico é o certo.
- Seja objetivo: responda em até uns 250 palavras, começando pela conclusão. Use markdown simples (listas e **negrito**).`

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
		Role:    llm.RoleUser,
		Content: "<dados_financeiros>\n" + snapshot + "\n</dados_financeiros>\n\n" + question,
	})
	return llm.Request{System: SystemPrompt, Messages: msgs}
}

// maxToolRounds limita as idas e voltas de ferramentas numa pergunta.
const maxToolRounds = 4

// Ask envia a pergunta ao provedor e devolve a resposta completa; onText recebe
// os pedaços conforme chegam. Com tools, a IA pode buscar lançamentos e propor
// lançamentos; se o modelo não aceitar ferramentas, pergunta sem elas.
func Ask(ctx context.Context, p llm.Provider, history []llm.Message, question, snapshot string, tools *Tools, onText func(string)) (string, error) {
	req := BuildRequest(history, question, snapshot)
	if tools != nil {
		req.System += toolsPrompt
		req.Tools = tools.Defs()
	}
	var answer strings.Builder
	for round := 1; ; round++ {
		if round > maxToolRounds { // chega de ferramentas: agora responde
			req.Tools = nil
		}
		reply, err := p.Chat(ctx, req, onText)
		if err != nil && req.Tools != nil && llm.ToolsUnsupported(err) {
			req.System, req.Tools = SystemPrompt, nil
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

// Question é uma pergunta ao consultor com tudo que ele precisa.
type Question struct {
	History   []llm.Message
	Text      string
	Ledger    domain.Ledger
	Today     time.Time
	Anonymize bool // IA de fora do aparelho: nomes viram apelidos (ver anon.go)
}

// Answer responde à pergunta com as ferramentas ligadas. Com Anonymize, a IA
// só vê apelidos e a resposta chega inteira no fim (já com os nomes de volta),
// porque um apelido pode vir partido entre dois pedaços do streaming.
func Answer(ctx context.Context, p llm.Provider, q Question, onText func(string), onProposal func(Proposal)) (string, []Proposal, error) {
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
	answer, err := Ask(ctx, p, history, text, Snapshot(BuildFacts(ledger, q.Today)), tools, emit)
	if err != nil {
		return "", nil, err
	}
	if anon != nil {
		answer = anon.Reveal(answer)
		onText(answer)
	}
	return answer, proposals, nil
}
