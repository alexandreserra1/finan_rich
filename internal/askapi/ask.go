// Package askapi é o consultor rodando no próprio celular (WebAssembly), sem
// computador: a pergunta vai direto para uma IA na nuvem, com os nomes
// anonimizados. Fica separado de wasmapi porque precisa da rede (net/http), que
// mais que dobra o WebAssembly: só o app do celular leva, e só carrega quando o
// Consultor é aberto (cmd/finanadvisor).
package askapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/importer"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func reply(v any, err error) string {
	r := struct {
		OK    any    `json:"ok,omitempty"`
		Error string `json:"error,omitempty"`
	}{OK: v}
	if err != nil {
		r.OK, r.Error = nil, err.Error()
	}
	b, _ := json.Marshal(r)
	return string(b)
}

// AskRequest é o pedido do app (JSON).
type AskRequest struct {
	Ledger    domain.Ledger `json:"ledger"`
	History   []ChatTurn    `json:"history"`
	Question  string        `json:"question"`
	Today     string        `json:"today"` // AAAA-MM-DD
	AI        AIConfig      `json:"ai"`
	Anonymize bool          `json:"anonymize"`
	Agent     string        `json:"agent"` // quem do time responde; vazio = Jade
}

type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIConfig struct {
	Preset  string `json:"preset"`
	Model   string `json:"model"`
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl,omitempty"` // outro endereço do mesmo provedor (testes, proxy próprio)
}

// AskReply é a resposta. App: texto do app, sem IA (pergunta pronta ou a IA
// falhou); AIError: por que a IA falhou; Limit: acabou o grátis do finan_rich
// (a tela mostra como usar a própria chave grátis).
type AskReply struct {
	Answer    string             `json:"answer"`
	Proposals []advisor.Proposal `json:"proposals"`
	App       bool               `json:"app,omitempty"`
	AIError   string             `json:"aiError,omitempty"`
	Limit     bool               `json:"limit,omitempty"`
}

// question monta a pergunta do consultor a partir do pedido do app.
func (req AskRequest) question() advisor.Question {
	today, err := time.ParseInLocation(domain.DateLayout, req.Today, time.Local)
	if err != nil {
		today = time.Now()
	}
	history := make([]llm.Message, len(req.History))
	for i, h := range req.History {
		history[i] = llm.Message{Role: llm.Role(h.Role), Content: h.Content}
	}
	return advisor.Question{History: history, Text: req.Question, Ledger: req.Ledger, Today: today, Anonymize: req.Anonymize, Agent: req.Agent}
}

func askReply(r advisor.Reply, preset string) AskReply {
	out := AskReply{Answer: r.Text, Proposals: r.Proposals, App: r.App}
	if out.Proposals == nil {
		out.Proposals = []advisor.Proposal{}
	}
	if r.AIError != nil {
		out.AIError = r.AIError.Error()
		out.Limit = preset == llm.FreeID && advisor.OverLimit(r.AIError)
	}
	return out
}

// Ask responde à pergunta. Bloqueia até a resposta: quem chama no navegador
// roda numa goroutine e devolve uma Promise (cmd/finanwasm). Se a IA falhar,
// a resposta é a do app, sem IA (advisor.Fallback).
func Ask(ctx context.Context, reqJSON string) string {
	var req AskRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	p, err := provider(req.AI)
	if err != nil {
		return reply(nil, err)
	}
	r, err := advisor.Answer(ctx, p, req.question(), func(string) {}, func(advisor.Proposal) {})
	if err != nil {
		return reply(nil, err)
	}
	return reply(askReply(r, req.AI.Preset), nil)
}

// Prepared devolve a resposta pronta do app para a pergunta, ou {} se ela
// precisa da IA. A IA dentro do celular pergunta aqui antes de gerar.
func Prepared(reqJSON string) string {
	var req AskRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	q := req.question()
	if !advisor.Prepared(q) {
		return reply(nil, nil)
	}
	return reply(askReply(advisor.Reply{Text: advisor.Offline(q), App: true}, ""), nil)
}

// Offline é a resposta sem IA, para quando a IA nem pôde ser chamada (sem
// internet, IA do celular com erro, computador fora do alcance).
func Offline(reqJSON string) string {
	var req AskRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	return reply(askReply(advisor.Fallback(req.question(), errors.New("a IA não respondeu")), ""), nil)
}

// Numbers são as contas do personagem para a tela mostrar na hora, antes da
// resposta da IA; {} se a pergunta é pronta (aí o app já responde com Prepared).
func Numbers(reqJSON string) string {
	var req AskRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	n := advisor.Numbers(req.question())
	if n == "" {
		return reply(nil, nil)
	}
	return reply(askReply(advisor.Reply{Text: n, App: true}, ""), nil)
}

// LocalPrompt monta as mensagens (formato OpenAI: system, user, assistant)
// para a IA que roda dentro do celular (wllama). As instruções e o retrato
// financeiro vêm daqui, das mesmas regras do computador; só a geração do
// texto roda no wllama. Sem ferramentas e sem apelidos: nada sai do aparelho.
func LocalPrompt(reqJSON string) string {
	var req AskRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	r := advisor.LocalRequest(req.question())
	msgs := []ChatTurn{{Role: "system", Content: r.System}}
	for _, m := range r.Messages {
		msgs = append(msgs, ChatTurn{Role: string(m.Role), Content: m.Content})
	}
	return reply(msgs, nil)
}

// Check confere a chave com uma pergunta mínima (sem dados da pessoa), para a
// tela dizer na hora se a chave funciona. Bloqueia como Ask.
func Check(ctx context.Context, aiJSON string) string {
	var ai AIConfig
	if err := json.Unmarshal([]byte(aiJSON), &ai); err != nil {
		return reply(nil, err)
	}
	p, err := provider(ai)
	if err == nil {
		_, err = p.Chat(ctx, llm.Request{Messages: []llm.Message{{Role: llm.RoleUser, Content: llm.PingMessage}}}, func(string) {})
	}
	return reply(err == nil, err)
}

func provider(ai AIConfig) (llm.Provider, error) {
	preset := llm.PresetByID(ai.Preset)
	if preset.ID != ai.Preset || preset.Kind != llm.KindOpenAI || !preset.NeedsKey && preset.ID != llm.FreeID {
		return nil, errors.New("escolha um provedor de IA na nuvem nos Ajustes do app")
	}
	// No grátis, a "chave" é só a identificação aleatória deste aparelho (limite por aparelho).
	if ai.APIKey == "" {
		return nil, errors.New("falta a chave da IA: cole a chave nos Ajustes do app")
	}
	model, base := ai.Model, preset.BaseURL
	if model == "" {
		model = preset.Model
	}
	if ai.BaseURL != "" {
		base = ai.BaseURL
	}
	return llm.New(llm.Config{Kind: preset.Kind, BaseURL: base, Model: model, APIKey: ai.APIKey})
}

// CategorizeRequest é o pedido da importação de extrato ("IA sugere").
type CategorizeRequest struct {
	Categories []domain.Category        `json:"categories"`
	Items      []advisor.CategorizeItem `json:"items"`
	AI         AIConfig                 `json:"ai"`
}

// Categorize sugere a categoria de cada lançamento do extrato com a IA na
// nuvem. Vai só a assinatura da descrição (sem CPF, conta, "PIX ENVIADO"),
// como faz o computador com IA na nuvem. Devolve o ID de cada um ("" = sem sugestão).
func Categorize(ctx context.Context, reqJSON string) string {
	var req CategorizeRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	p, err := provider(req.AI)
	if err != nil {
		return reply(nil, err)
	}
	for i := range req.Items {
		req.Items[i].Description = importer.DescKey(req.Items[i].Description)
	}
	return reply(advisor.Categorize(ctx, p, req.Categories, req.Items))
}

// CloudPresets são os provedores que o app do celular oferece (nuvem, com chave).
func CloudPresets() string {
	var out []map[string]any
	for _, p := range llm.All() {
		if free := p.ID == llm.FreeID; free || p.Kind == llm.KindOpenAI && p.NeedsKey {
			out = append(out, map[string]any{"id": p.ID, "name": p.Name, "description": p.Description, "model": p.Model, "models": p.Models, "keyUrl": p.KeyURL, "keySteps": p.KeySteps, "free": free})
		}
	}
	return reply(out, nil)
}

// ProposalApplied diz se a proposta já foi confirmada (cartões da conversa).
func ProposalApplied(ledgerJSON, proposalJSON string) string {
	var l domain.Ledger
	var p advisor.Proposal
	if err := json.Unmarshal([]byte(ledgerJSON), &l); err != nil {
		return reply(nil, err)
	}
	if err := json.Unmarshal([]byte(proposalJSON), &p); err != nil {
		return reply(nil, err)
	}
	return reply(advisor.Applied(l, p), nil)
}
