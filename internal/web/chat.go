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
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	history, err := s.store.ListChat(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, r, "chat", map[string]any{"History": history})
}

func (s *Server) chatClear(w http.ResponseWriter, r *http.Request) {
	err := s.store.ClearChat(r.Context())
	redirect(w, r, "/chat", "Conversa apagada.", err)
}

// snapshot monta o retrato financeiro enviado junto com cada pergunta.
func (s *Server) snapshot(ctx context.Context) (string, error) {
	now := time.Now()
	m := store.CurrentMonth()
	sum, _, err := s.monthSummary(ctx, m)
	if err != nil {
		return "", err
	}
	var recent []store.Totals
	for _, year := range []int{m.Year() - 1, m.Year()} {
		totals, err := s.store.MonthlyTotals(ctx, year)
		if err != nil {
			return "", err
		}
		for _, t := range totals {
			// últimos 6 meses fechados, ignorando meses sem movimento
			if t.Month.Before(m.Time) && !t.Month.Before(m.AddDate(0, -6, 0)) && (t.InCents > 0 || t.OutCents > 0) {
				recent = append(recent, t)
			}
		}
	}
	debts, err := s.store.ListDebts(ctx)
	if err != nil {
		return "", err
	}
	return advisor.Snapshot(now, sum, recent, finance.PlanDebts(debts)), nil
}

// chatSend responde em Server-Sent Events: {"text": "..."} a cada pedaço,
// {"error": "..."} em caso de falha e {"done": true} ao final.
func (s *Server) chatSend(w http.ResponseWriter, r *http.Request) {
	if s.advisor == nil {
		http.Error(w, "Chat desativado: defina ANTHROPIC_API_KEY.", http.StatusServiceUnavailable)
		return
	}
	question := strings.TrimSpace(r.FormValue("message"))
	if question == "" {
		http.Error(w, "mensagem vazia", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming não suportado", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	send := func(v any) {
		b, _ := json.Marshal(v)
		w.Write([]byte("data: "))
		w.Write(b)
		w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	ctx := r.Context()
	fail := func(err error) {
		log.Printf("chat: %v", err)
		send(map[string]string{"error": err.Error()})
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
	turn, err := s.advisor.Reply(ctx, history, question, snap, func(text string) {
		send(map[string]string{"text": text})
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		fail(err)
		return
	}
	// Contexto sem cancelamento: se a conexão cair agora, a resposta (já paga) não se perde.
	if err := s.store.AppendChatTurn(context.WithoutCancel(ctx), turn.User, turn.Assistant); err != nil {
		fail(err)
		return
	}
	send(map[string]bool{"done": true})
}
