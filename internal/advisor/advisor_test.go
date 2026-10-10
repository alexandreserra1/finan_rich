package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/alexandreserra1/finan_rich/internal/store"
)

// fakeAPI responde como a API de Messages em streaming e guarda o corpo recebido.
func fakeAPI(t *testing.T, stopReason string, got *map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, got); err != nil {
			t.Errorf("corpo inválido: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		events := []string{
			`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Pague o "}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"cartão primeiro."}}`,
			`{"type":"content_block_stop","index":0}`,
			fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q},"usage":{"output_tokens":5}}`, stopReason),
			`{"type":"message_stop"}`,
		}
		for _, e := range events {
			var typ struct{ Type string }
			json.Unmarshal([]byte(e), &typ)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ.Type, e)
		}
	}))
}

func TestReplyStreamsAndReplaysHistory(t *testing.T) {
	var body map[string]any
	srv := fakeAPI(t, "end_turn", &body)
	defer srv.Close()
	a := New("test-key", "claude-opus-5-5", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))

	var streamed strings.Builder
	turn1, err := a.Reply(context.Background(), nil, "Qual dívida pago primeiro?", "saldo X", func(s string) { streamed.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if streamed.String() != "Pague o cartão primeiro." || turn1.Assistant.Display != streamed.String() {
		t.Fatalf("texto: streamed=%q display=%q", streamed.String(), turn1.Assistant.Display)
	}
	if body["fallbacks"] != "default" || body["model"] != "claude-opus-5-5" {
		t.Errorf("parâmetros: fallbacks=%v model=%v", body["fallbacks"], body["model"])
	}
	if turn1.User.Display != "Qual dívida pago primeiro?" {
		t.Errorf("display do usuário = %q", turn1.User.Display)
	}

	// Segunda pergunta: o histórico gravado deve ser reenviado byte a byte.
	history := []store.ChatMessage{turn1.User, turn1.Assistant}
	if _, err := a.Reply(context.Background(), history, "E depois?", "saldo Y", func(string) {}); err != nil {
		t.Fatal(err)
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("esperava 3 mensagens, veio %d", len(msgs))
	}
	for i, h := range history {
		sent, _ := json.Marshal(msgs[i])
		var want, gotV any
		json.Unmarshal(h.Content, &want)
		json.Unmarshal(sent, &gotV)
		if fmt.Sprint(want) != fmt.Sprint(gotV) {
			t.Errorf("mensagem %d alterada no reenvio:\n gravada: %s\n enviada: %s", i, h.Content, sent)
		}
	}
	if !strings.Contains(fmt.Sprint(msgs[2]), "saldo Y") {
		t.Errorf("snapshot novo não foi enviado: %v", msgs[2])
	}
}

func TestReplyRefusal(t *testing.T) {
	var body map[string]any
	srv := fakeAPI(t, "refusal", &body)
	defer srv.Close()
	a := New("test-key", "claude-opus-5-5", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	if _, err := a.Reply(context.Background(), nil, "?", "", func(string) {}); err != ErrRefused {
		t.Fatalf("esperava ErrRefused, veio %v", err)
	}
}
