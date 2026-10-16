package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
)

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	history, err := s.store.ListChat(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	ai, err := s.aiSettings(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, r, "chat", map[string]any{"History": history, "AI": ai, "Status": s.aiStatus(ctx, ai)})
}

func (s *Server) chatClear(w http.ResponseWriter, r *http.Request) {
	err := s.store.ClearChat(r.Context())
	redirect(w, r, "/chat", "Conversa apagada.", err)
}

// chatSend responde em SSE: {"text": "..."} a cada pedaço, {"proposal": {...}}
// a cada lançamento que a IA propõe (a tela mostra com um botão para aprovar),
// {"error": "..."} em caso de falha e {"done": true} ao final.
func (s *Server) chatSend(w http.ResponseWriter, r *http.Request) {
	s.answer(w, r, r.FormValue("message"))
}

// answer responde à pergunta em SSE e grava a conversa (tela do navegador e
// app do celular usam o mesmo caminho).
func (s *Server) answer(w http.ResponseWriter, r *http.Request, question string) {
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

	provider, _, err := s.provider(ctx)
	if err != nil {
		fail(err)
		return
	}
	ledger, err := s.store.Ledger(ctx)
	if err != nil {
		fail(err)
		return
	}
	now := time.Now()
	snap := advisor.Snapshot(advisor.BuildFacts(ledger, now))
	tools := advisor.NewTools(ledger, now, func(p advisor.Proposal) {
		sse.Send(map[string]any{"proposal": p})
	})
	history, err := s.store.ListChat(ctx)
	if err != nil {
		fail(err)
		return
	}
	answer, err := advisor.Ask(ctx, provider, history, question, snap, tools, func(text string) {
		sse.Send(map[string]string{"text": text})
	})
	if errors.Is(err, context.Canceled) {
		return
	}
	if err == nil && answer == "" {
		err = errors.New("o modelo não devolveu resposta; tente de novo")
	}
	if err != nil {
		fail(err)
		return
	}
	// Contexto sem cancelamento: se a conexão cair agora, a resposta não se perde.
	if err := s.store.AppendChatTurn(context.WithoutCancel(ctx), question, answer); err != nil {
		fail(err)
		return
	}
	sse.Done()
}

// ---- API do consultor para o app do celular (JSON, com o token do aparelho) ----

type chatMessageJSON struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatInfo struct {
	History []chatMessageJSON `json:"history"`
	Model   string            `json:"model"`
	Ready   bool              `json:"ready"`
	Problem string            `json:"problem,omitempty"`
}

// apiChatHistory devolve a conversa e se a IA está pronta.
func (s *Server) apiChatHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	history, err := s.store.ListChat(ctx)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ai, err := s.aiSettings(ctx)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	st := s.aiStatus(ctx, ai)
	info := chatInfo{History: []chatMessageJSON{}, Model: ai.Preset.Name + " · " + ai.Model, Ready: st.Ready, Problem: st.Problem}
	for _, m := range history {
		info.History = append(info.History, chatMessageJSON{m.Role, m.Content})
	}
	writeJSON(w, http.StatusOK, info)
}

// apiChatSend recebe {"message": "..."} e responde em SSE, como /chat/enviar.
func (s *Server) apiChatSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "pedido inválido")
		return
	}
	s.answer(w, r, req.Message)
}

func (s *Server) apiChatClear(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ClearChat(r.Context()); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
