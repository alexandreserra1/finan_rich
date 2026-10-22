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

type AskReply struct {
	Answer    string             `json:"answer"`
	Proposals []advisor.Proposal `json:"proposals"`
}

// Ask responde à pergunta. Bloqueia até a resposta: quem chama no navegador
// roda numa goroutine e devolve uma Promise (cmd/finanwasm).
func Ask(ctx context.Context, reqJSON string) string {
	var req AskRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	p, err := provider(req.AI)
	if err != nil {
		return reply(nil, err)
	}
	today, err := time.ParseInLocation(domain.DateLayout, req.Today, time.Local)
	if err != nil {
		today = time.Now()
	}
	history := make([]llm.Message, len(req.History))
	for i, h := range req.History {
		history[i] = llm.Message{Role: llm.Role(h.Role), Content: h.Content}
	}
	answer, proposals, err := advisor.Answer(ctx, p, advisor.Question{
		History: history, Text: req.Question, Ledger: req.Ledger, Today: today, Anonymize: req.Anonymize,
	}, func(string) {}, func(advisor.Proposal) {})
	if err == nil && answer == "" {
		err = errors.New("a IA não devolveu resposta; tente de novo")
	}
	if proposals == nil {
		proposals = []advisor.Proposal{}
	}
	return reply(AskReply{answer, proposals}, err)
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
