// Package advisor monta a conversa do consultor financeiro: as instruções, o
// retrato atual das finanças e o histórico. Não depende de nenhum provedor de
// IA específico; quem envia é um llm.Provider.
package advisor

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"

	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/rates"
)

// SystemPrompt são as instruções fixas do consultor. Mantenha-o estável: alguns
// provedores reaproveitam o processamento do início da conversa (cache).
// PromptMarker abre as instruções do consultor. O intermediário grátis
// (internal/proxy) reconhece o app por ela; mude o resto à vontade, ela não.
const PromptMarker = "Você é o consultor financeiro pessoal do usuário dentro do app finan_rich"

const SystemPrompt = PromptMarker + ` (para as pessoas, o app se chama Rich), um controle de gastos individual.

Como agir:
- Responda sempre em português do Brasil, de forma direta e prática, como um amigo que entende muito de finanças pessoais no Brasil.
- A última mensagem do usuário traz <dados_financeiros> com os números atuais do app (mês, divisão do salário, dívidas, metas, ritmo do mês, onde economizar). Os valores já foram calculados e estão corretos: use-os como estão. Se precisar de uma conta nova, faça uma conta simples e mostre-a (ex.: "R$ 4.500 - R$ 3.550 = R$ 950").
- Se vier <conversa_anterior>, é o resumo do começo desta conversa (as mensagens mais antigas): use para lembrar o que já foi dito, sem repetir.
- Nunca invente botões, telas ou recursos. As telas do app são: Painel, Lançamentos, Contas, Dívidas, Metas, Análise, Consultor e Ajustes. Os botões que existem para isso: "Registrar pagamento" (em cada dívida, na tela Dívidas), "Registrar aporte" (em cada meta, na tela Metas), "Novo lançamento" (no topo de qualquer tela) e o cartão de confirmar que aparece aqui quando você prepara algo. Se houver "Contas prontas para esta pergunta" nos dados, os números estão lá: use-os em vez de mandar a pessoa calcular.
- Nunca invente números que não estão nos dados ou na conversa. Se faltar dado, diga o que falta e peça para o usuário lançar no app.
- Personalize sempre: use os números do usuário e os nomes das dívidas, metas e lugares dele. Nada de conselho genérico que serviria para qualquer pessoa.
- Pergunta vaga ("o que eu faço?", "me ajuda"): olhe os pontos de atenção e a análise do mês e ataque o maior problema primeiro, seguindo a ordem de prioridades (contas em dia → rotativo e cheque especial → reserva inicial → outras dívidas caras → reserva completa → metas e investimentos).
- O formato depende da pergunta:
  - Pergunta direta (um valor, uma data, uma lista: "quanto gastei com iFood?", "o que vence esta semana?", "qual a Selic hoje?"): responda já na primeira frase, com o número principal em **negrito**, em até 3 frases ou uma lista curta. Sem plano em passos e sem "hoje no app", a não ser que a pessoa peça.
  - Pedido de conselho, decisão ou plano ("o que eu faço?", "vale a pena?", "onde economizar?", "como saio das dívidas?"): comece com a conclusão em uma frase em **negrito**; depois o porquê com pelo menos dois números dele; depois um plano em passos numerados com valor em R$ e prazo; termine com um próximo passo que dá para fazer hoje no app (registrar um pagamento, criar uma meta, cortar uma assinatura).
- O bloco <conhecimento> traz regras e boas práticas de finanças no Brasil: aplique ao caso do usuário, não copie.
- Taxas do momento: quando vierem "Taxas do Banco Central" nos dados (Selic, CDI e IPCA), use-as e diga a data. As que não estiverem nos dados (juros de um banco, de um CDB, a Selic quando não veio) mudam: não invente; diga para conferir a taxa do dia.
- Você não é consultor de investimentos certificado: explique critérios (risco, prazo, liquidez, custo) sem indicar ação, fundo, cripto ou banco específico, e sem prometer retorno. Investimento de risco só depois de dívidas caras quitadas e reserva montada.
- Para perguntas sobre um estabelecimento ou serviço (ex.: "quanto gastei com iFood?"), use a lista de gastos por estabelecimento.
- Para "onde economizar", passe por todos os itens de "Onde economizar" dos dados (inclusive as assinaturas), do que economiza mais para o que economiza menos.
- Se a pergunta não for de finanças, responda em uma frase e ofereça ajuda com as finanças dele.
- Seja objetivo: resposta direta em até umas 60 palavras; conselho ou plano em até umas 250. Use markdown simples (listas e **negrito**).

Exemplos do estilo (os números são de outra pessoa):
Pergunta: "quanto gastei com delivery este mês?"
Resposta: "Você gastou **R$ 684** com delivery em outubro, em 12 pedidos. É o dobro de setembro (R$ 342)."

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
	var older []llm.Message
	if len(history) > maxHistory {
		older, history = history[:len(history)-maxHistory], history[len(history)-maxHistory:]
	}
	// O histórico precisa começar com uma pergunta do usuário.
	for len(history) > 0 && history[0].Role != llm.RoleUser {
		older, history = append(older, history[0]), history[1:]
	}
	msgs := make([]llm.Message, 0, len(history)+1)
	for _, h := range history {
		msgs = append(msgs, llm.Message{Role: llm.Role(h.Role), Content: h.Content})
	}
	// O conhecimento vem antes dos dados: assim a pergunta fica no fim, sozinha.
	content := "<conhecimento>\n" + knowledgeFor(question) + "\n</conhecimento>\n\n<dados_financeiros>\n" + snapshot + "\n</dados_financeiros>\n\n" + question
	if s := earlier(older); s != "" {
		content = "<conversa_anterior>\n" + s + "\n</conversa_anterior>\n\n" + content
	}
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: content})
	return llm.Request{System: SystemPrompt, Messages: msgs}
}

// earlier resume, sem chamar a IA, as mensagens que ficaram fora do histórico: as
// últimas 12 falas, cada uma só com o começo (a conclusão da resposta vem primeiro).
func earlier(msgs []llm.Message) string {
	const maxLines, maxRunes = 12, 160
	var lines []string
	for i := len(msgs) - 1; i >= 0 && len(lines) < maxLines; i-- {
		who := map[llm.Role]string{llm.RoleUser: "Pessoa", llm.RoleAssistant: "Consultor"}[msgs[i].Role]
		text := []rune(strings.Join(strings.Fields(strings.ReplaceAll(msgs[i].Content, "**", "")), " "))
		if who == "" || len(text) == 0 {
			continue
		}
		if len(text) > maxRunes {
			cut := maxRunes
			for cut > maxRunes/2 && text[cut] != ' ' {
				cut--
			}
			text = append(text[:cut], '…')
		}
		lines = append(lines, "- "+who+": "+string(text))
	}
	slices.Reverse(lines)
	return strings.Join(lines, "\n")
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
		if onboarding(agent, history, question) { // só as de criar: o pedido fica pequeno para a cota grátis
			tools.only = onboardingTools
			req.System += onboardingPrompt
			// Com valores na mensagem, o cartão tem de sair: o modelo grátis dizia "conta
			// criada" sem chamar a ferramenta. Vale só no primeiro passo (veja o laço).
			if strings.ContainsAny(question, "0123456789") {
				req.ToolChoice = "required"
			}
		}
		req.Tools = tools.Defs()
	}
	var answer strings.Builder
	usedTool, glitched, nudged := false, false, false
	starting := req.ToolChoice == "required" // começo do zero, com valores na mensagem
	for round := 1; ; round++ {
		if round > maxToolRounds && req.Tools != nil { // chega de ferramentas: agora responde
			req.Tools, req.Messages = nil, withoutTools(req.Messages)
		}
		reply, err := p.Chat(ctx, req, onText)
		if err != nil && req.Tools != nil && llm.ToolsUnsupported(err) {
			req.System, req.Tools, req.Messages, req.ToolChoice = systemFor(agent), nil, withoutTools(req.Messages), ""
			continue
		}
		if err != nil && !glitched && toolCallGlitch(err) { // JSON quebrado na ferramenta: passa, tenta de novo
			glitched = true
			continue
		}
		if err != nil {
			return "", err
		}
		// "Deixei pronto" sem ter chamado ferramenta nenhuma: nenhum cartão apareceu. Pede de
		// novo uma vez, com um aviso; a resposta final é só a da nova tentativa.
		if len(reply.ToolCalls) == 0 && req.Tools != nil && !usedTool && falseClaimRe.MatchString(reply.Text) {
			if !nudged {
				nudged = true
				answer.Reset()
				onText("\n\n")
				req.Messages = append(req.Messages,
					llm.Message{Role: llm.RoleAssistant, Content: reply.Text},
					llm.Message{Role: llm.RoleUser, Content: falseClaimNudge})
				continue
			}
			reply.Text += "\n\n" + falseClaimNote
			onText("\n\n" + falseClaimNote)
		}
		answer.WriteString(reply.Text)
		if len(reply.ToolCalls) == 0 || req.Tools == nil {
			return strings.TrimSpace(answer.String()), nil
		}
		usedTool, req.ToolChoice = true, "" // com o resultado na mão, o modelo escolhe se chama mais
		if reply.Text != "" {
			answer.WriteString("\n\n")
			onText("\n\n")
		}
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleAssistant, Content: reply.Text, ToolCalls: reply.ToolCalls})
		for _, c := range reply.ToolCalls {
			result := tools.Run(c)
			if starting { // o modelo grátis criava só o primeiro de dois itens da mesma mensagem
				result += " Se a mensagem da pessoa tiver outro item com valor que ainda não virou cartão, chame a ferramenta para ele agora."
			}
			req.Messages = append(req.Messages, llm.Message{Role: llm.RoleTool, ToolCallID: c.ID, ToolName: c.Name, Content: result})
		}
	}
}

// onboardingPrompt: o começo do zero (finance.StartQuestion), com o Otávio
// perguntando uma coisa por vez e preparando os cartões.
const onboardingPrompt = `

Começo do zero: a pessoa está começando no app e pediu ajuda para montar o mês. Faça só UMA pergunta por resposta, curta e com um exemplo; não mostre a lista de perguntas, pergunte apenas a próxima desta ordem:
1. Quanto recebe por mês e em que dia (ex.: "4.500 no dia 5").
2. As contas fixas do mês, com valor e dia (aluguel, condomínio, luz, internet, celular, escola).
3. As dívidas: nome, quanto deve, juros ao mês e parcela mínima (ou "não tenho").
4. Uma meta: o que quer juntar, quanto e até quando (ou "agora não").
Quando a pessoa der valores, chame a ferramenta nessa mesma resposta, uma vez para cada item: criar_conta_fixa para a renda (tipo entrada) e para cada conta fixa (tipo saida), criar_divida para cada dívida, criar_meta para a meta. Não pergunte a categoria: o app escolhe pela descrição (internet e luz em Contas da casa, aluguel em Moradia). Só diga que deixou um cartão pronto depois de chamar a ferramenta. Não invente valor nem dia: se faltar, pergunte. Se a pessoa disser que não tem, siga para a próxima pergunta. Depois da última, diga em uma frase o que ficou pronto e que é só tocar em "Adicionar" em cada cartão.`

var onboardingTools = []string{"propor_lancamento", "criar_meta", "criar_divida", "criar_conta_fixa"}

// onboarding diz se a conversa está no começo do zero, só com o Otávio: a pergunta
// de abertura é esta ou veio há até 3 respostas (são 4: renda, contas, dívidas, meta).
func onboarding(agent string, history []llm.Message, question string) bool {
	if persona(agent).ID != "otavio" {
		return false
	}
	start := fold(finance.StartQuestion)
	if fold(question) == start {
		return true
	}
	answers := 0
	for i := len(history) - 1; i >= 0 && answers < 4; i-- {
		if history[i].Role != llm.RoleUser {
			continue
		}
		if fold(history[i].Content) == start {
			return true
		}
		answers++
	}
	return false
}

// falseClaimRe: a resposta diz que deixou um cartão pronto. Sem nenhuma ferramenta chamada
// na pergunta, é falso (aconteceu com o modelo grátis no começo do zero).
var falseClaimRe = regexp.MustCompile(`(?i)deixei (?:pront|preparad)|preparei (?:o|a|os|as|um|uma) (?:cart|conta|lan[cç]|meta|d[ií]vida|pagamento|aporte)|pront[oa]s? para (?:voc[eê] )?confirmar`)

const (
	falseClaimNudge = "Aviso do app (não é da pessoa): nenhum cartão foi criado, porque nenhuma ferramenta foi chamada. Chame agora a ferramenta certa com os valores que a pessoa já deu; se faltar algum dado, pergunte, sem dizer que deixou pronto."
	falseClaimNote  = "(Não consegui preparar o cartão desta vez. Mande os valores de novo, ou cadastre à mão em Lançamentos.)"
)

// toolCallGlitch: o provedor recusou no meio da resposta porque o modelo escreveu os
// argumentos da ferramenta em JSON quebrado. Costuma passar na tentativa seguinte.
func toolCallGlitch(err error) bool {
	var apiErr *llm.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	msg := strings.ToLower(apiErr.Message)
	return strings.Contains(msg, "tool call") || strings.Contains(msg, "tool_use_failed") || strings.Contains(msg, "failed to call a function")
}

// withoutTools troca os pedidos e resultados de ferramentas por texto: pedido
// sem "tools" mas com blocos de ferramenta é recusado por alguns provedores.
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
	Anonymize bool        // IA de fora do aparelho: nomes viram apelidos (ver anon.go)
	Agent     string      // quem do time responde (personas.go); vazio = Otávio
	Rates     rates.Rates // taxas do Banco Central; vazio se o app não conseguiu buscar
	AskAI     bool        // "Perguntar à IA": vai para a IA mesmo quando o app tem a resposta
}

// Answer responde à pergunta com as ferramentas ligadas. Com Anonymize, a IA
// só vê apelidos; a resposta chega aos poucos já com os nomes de volta
// (RevealStream segura o fim que ainda pode ser um apelido partido).
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
	var stream *RevealStream
	if q.Anonymize {
		ledger, anon = Anonymize(q.Ledger)
		history = make([]llm.Message, len(q.History))
		for i, m := range q.History {
			m.Content = anon.Text(m.Content)
			history[i] = m
		}
		stream = anon.Stream()
		text = anon.Text(q.Text)
		emit = func(s string) {
			if out := stream.Write(s); out != "" {
				onText(out)
			}
		}
	}
	var proposals []Proposal
	tools := NewTools(ledger, q.Today, func(pr Proposal) {
		if anon != nil {
			pr = anon.RevealProposal(pr)
		}
		proposals = append(proposals, pr)
		onProposal(pr)
	})
	snapshot := SnapshotFor(BuildFacts(ledger, q.Today), q.Agent)
	if persona(q.Agent).ID == "otavio" {
		// Com ferramentas ele busca nos lançamentos; a lista de lugares (assunto da
		// Lia) faria o pedido passar da cota por minuto da IA grátis.
		snapshot = withoutSection(snapshot, "Gastos por estabelecimento")
	}
	if r := q.Rates.Text(); r != "" {
		snapshot += "\n\n" + r
	}
	answer, err := Ask(ctx, p, q.Agent, history, text, withPrefetch(snapshot, q.Agent, tools, text), tools, emit)
	if err == nil && answer == "" {
		err = errors.New("a IA não devolveu resposta")
	}
	if err != nil {
		return answerOrFallback(ctx, q, err)
	}
	if anon != nil {
		answer = anon.Reveal(answer)
		if out := stream.Flush(); out != "" {
			onText(out)
		}
	}
	return Reply{Text: answer, Proposals: proposals}, nil
}
