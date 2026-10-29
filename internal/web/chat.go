package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// chatEntry é uma mensagem da conversa com as propostas (e se já valem).
type chatEntry struct {
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	Proposals []advisor.Proposal `json:"proposals,omitempty"`
	Agent     string             `json:"agent,omitempty"` // quem do time respondeu
	Who       finance.Agent      `json:"-"`               // o mesmo, com nome e imagem (página do computador)
}

// member devolve o personagem pelo id; vazio ou desconhecido é a Jade.
func member(id string) finance.Agent {
	for _, a := range finance.Team {
		if a.ID == id {
			return a
		}
	}
	return finance.Team[0]
}

// chatHistory lê a conversa e marca as propostas que a pessoa já confirmou.
func (s *Server) chatHistory(ctx context.Context) ([]chatEntry, error) {
	history, err := s.store.ListChat(ctx)
	if err != nil {
		return nil, err
	}
	var ledger *domain.Ledger
	out := make([]chatEntry, len(history))
	for i, m := range history {
		out[i] = chatEntry{Role: m.Role, Content: m.Content}
		if m.Role == store.RoleAssistant {
			out[i].Who = member(m.Agent)
			out[i].Agent = out[i].Who.ID
		}
		if len(m.Proposals) == 0 || json.Unmarshal(m.Proposals, &out[i].Proposals) != nil {
			continue
		}
		if ledger == nil {
			l, err := s.store.Ledger(ctx)
			if err != nil {
				return nil, err
			}
			ledger = &l
		}
		for j := range out[i].Proposals {
			out[i].Proposals[j].Applied = advisor.Applied(*ledger, out[i].Proposals[j])
		}
	}
	return out, nil
}

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	history, err := s.chatHistory(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	ai, err := s.aiSettings(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, r, "chat", map[string]any{"History": history, "AI": ai, "Status": s.aiStatus(ctx, ai), "Team": finance.Team, "Suggestions": advisor.Suggestions})
}

func (s *Server) chatClear(w http.ResponseWriter, r *http.Request) {
	err := s.store.ClearChat(r.Context())
	redirect(w, r, "/chat", "Conversa apagada.", err)
}

// chatSend responde em SSE: {"text": "..."} a cada pedaço, {"proposal": {...}}
// a cada lançamento que a IA propõe (a tela mostra com um botão para aprovar),
// {"error": "..."} em caso de falha e {"done": true} ao final.
func (s *Server) chatSend(w http.ResponseWriter, r *http.Request) {
	s.answer(w, r, r.FormValue("message"), r.FormValue("agent"))
}

// answer responde à pergunta em SSE e grava a conversa (tela do navegador e
// app do celular usam o mesmo caminho).
func (s *Server) answer(w http.ResponseWriter, r *http.Request, question, agent string) {
	agent = member(agent).ID // só ids do time chegam à IA e ao banco
	question = strings.TrimSpace(question)
	if question == "" {
		http.Error(w, "mensagem vazia", http.StatusBadRequest)
		return
	}
	sse, ok := newSSE(w)
	if !ok {
		return
	}
	ctx := r.Context()
	fail := func(err error) {
		log.Printf("chat: %v", err)
		sse.Error(err)
	}

	ledger, err := s.store.Ledger(ctx)
	if err != nil {
		fail(err)
		return
	}
	history, err := s.store.ListChat(ctx)
	if err != nil {
		fail(err)
		return
	}
	// IA de nuvem: nomes de lojas, dívidas e metas saem com apelidos.
	past := make([]llm.Message, len(history))
	for i, m := range history {
		past[i] = llm.Message{Role: llm.Role(m.Role), Content: m.Content}
	}
	q := advisor.Question{History: past, Text: question, Ledger: ledger, Today: time.Now(), Agent: agent}
	var reply advisor.Reply
	provider, ai, err := s.provider(ctx)
	if err != nil { // IA mal configurada: responde sem IA, com o motivo
		reply = advisor.Fallback(q, err)
	} else {
		q.Anonymize = !llm.IsLocal(ai.Config)
		reply, err = advisor.Answer(ctx, provider, q,
			func(text string) { sse.Send(map[string]string{"text": text}) },
			func(p advisor.Proposal) { sse.Send(map[string]any{"proposal": p}) })
		if err != nil { // a pessoa saiu da tela
			return
		}
	}
	answer, proposals := reply.Text, reply.Proposals
	// Resposta do app (sem IA): a tela troca o texto e marca de onde veio.
	if reply.App {
		ev := map[string]any{"text": answer}
		if reply.AIError != nil {
			log.Printf("chat: IA falhou, resposta sem IA: %v", reply.AIError)
			ev["reason"] = reply.AIError.Error()
			ev["limit"] = ai.Preset.ID == llm.FreeID && advisor.OverLimit(reply.AIError)
		}
		sse.Send(map[string]any{"app": ev})
	}
	// Contexto sem cancelamento: se a conexão cair agora, a resposta não se perde.
	var saved json.RawMessage
	if len(proposals) > 0 {
		saved, _ = json.Marshal(proposals)
	}
	if err := s.store.AppendChatTurn(context.WithoutCancel(ctx), question, answer, agent, saved); err != nil {
		fail(err)
		return
	}
	// O assunto é de outro do time: a tela oferece perguntar para ele.
	if other := finance.Route(question); other != "" && other != agent {
		sse.Send(map[string]any{"suggest": member(other)})
	}
	sse.Done()
}

// ---- API do consultor para o app do celular (JSON, com o token do aparelho) ----

type chatInfo struct {
	History []chatEntry `json:"history"`
	Model   string      `json:"model"`
	Ready   bool        `json:"ready"`
	Problem string      `json:"problem,omitempty"`
}

// apiChatHistory devolve a conversa e se a IA está pronta.
func (s *Server) apiChatHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	history, err := s.chatHistory(ctx)
	if err != nil {
		internalError(w, err)
		return
	}
	ai, err := s.aiSettings(ctx)
	if err != nil {
		internalError(w, err)
		return
	}
	st := s.aiStatus(ctx, ai)
	info := chatInfo{History: append([]chatEntry{}, history...), Model: ai.Preset.Name + " · " + ai.Model, Ready: st.Ready, Problem: st.Problem}
	writeJSON(w, http.StatusOK, info)
}

// apiChatSend recebe {"message": "..."} e responde em SSE, como /chat/enviar.
func (s *Server) apiChatSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
		Agent   string `json:"agent"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "pedido inválido")
		return
	}
	s.answer(w, r, req.Message, req.Agent)
}

func (s *Server) apiChatClear(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ClearChat(r.Context()); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
