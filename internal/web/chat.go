package web

import (
	"context"
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

// snapshot monta o retrato financeiro enviado junto com cada pergunta.
func (s *Server) snapshot(ctx context.Context) (string, error) {
	l, err := s.store.Ledger(ctx)
	if err != nil {
		return "", err
	}
	return advisor.Snapshot(advisor.BuildFacts(l, time.Now())), nil
}

// chatSend responde em SSE: {"text": "..."} a cada pedaço, {"error": "..."}
// em caso de falha e {"done": true} ao final.
func (s *Server) chatSend(w http.ResponseWriter, r *http.Request) {
	question := strings.TrimSpace(r.FormValue("message"))
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
	snap, err := s.snapshot(ctx)
	if err != nil {
		fail(err)
		return
	}
	history, err := s.store.ListChat(ctx)
	if err != nil {
		fail(err)
		return
	}
	answer, err := advisor.Ask(ctx, provider, history, question, snap, func(text string) {
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
