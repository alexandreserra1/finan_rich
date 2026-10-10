package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var testReq = Request{
	System: "sistema",
	Messages: []Message{
		{Role: RoleUser, Content: "oi"},
		{Role: RoleAssistant, Content: "olá"},
		{Role: RoleUser, Content: "qual dívida pago primeiro?"},
	},
}

// server registra o corpo recebido e responde com handler.
func server(t *testing.T, body *map[string]any, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body != nil {
			raw, _ := io.ReadAll(r.Body)
			*body = nil
			json.Unmarshal(raw, body)
		}
		handler(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func collectChat(t *testing.T, p Provider) (streamed, full string, err error) {
	t.Helper()
	var b strings.Builder
	full, err = p.Chat(context.Background(), testReq, func(s string) { b.WriteString(s) })
	return b.String(), full, err
}

func TestOllamaChat(t *testing.T) {
	var body map[string]any
	srv := server(t, &body, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %s", r.URL.Path)
		}
		for _, c := range []string{"<think>hmm</think>", "Pague o ", "cartão."} {
			fmt.Fprintf(w, `{"message":{"role":"assistant","content":%q},"done":false}`+"\n", c)
		}
		fmt.Fprintln(w, `{"message":{"role":"assistant","content":""},"done":true}`)
	})
	streamed, full, err := collectChat(t, NewOllama(srv.URL, "qwen3:4b"))
	if err != nil || streamed != "Pague o cartão." || full != streamed {
		t.Fatalf("streamed=%q full=%q err=%v", streamed, full, err)
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 4 || msgs[0].(map[string]any)["role"] != "system" {
		t.Errorf("mensagens enviadas = %v", msgs)
	}
	if body["think"] != false || body["model"] != "qwen3:4b" || body["options"].(map[string]any)["num_ctx"] != float64(contextTokens) || body["options"].(map[string]any)["temperature"] != 0.7 {
		t.Errorf("parâmetros = %v", body)
	}
}

func TestOllamaErrors(t *testing.T) {
	srv := server(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"model \"qwen3:4b\" not found, try pulling it first"}`)
	})
	_, _, err := collectChat(t, NewOllama(srv.URL, "qwen3:4b"))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || !strings.Contains(err.Error(), "ainda não foi baixado") {
		t.Errorf("modelo ausente: err = %v", err)
	}

	srv.Close() // servidor fora do ar = Ollama não instalado/aberto
	if _, _, err := collectChat(t, NewOllama(srv.URL, "x")); !errors.Is(err, ErrOllamaOffline) {
		t.Errorf("offline: err = %v", err)
	}
}

func TestOllamaCloudSignin(t *testing.T) {
	srv := server(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized","signin_url":"https://ollama.com/connect?key=abc"}`)
	})
	_, _, err := collectChat(t, NewOllama(srv.URL, "glm-5.1:cloud"))
	var signin *SigninError
	if !errors.As(err, &signin) || signin.URL != "https://ollama.com/connect?key=abc" || !strings.Contains(err.Error(), "ollama signin") {
		t.Errorf("err = %v", err)
	}
	if has, _ := NewOllama(srv.URL, "glm-5.1:cloud").HasModel(context.Background()); !has {
		t.Error("modelo da nuvem não precisa de download")
	}
}

func TestIsCloudModel(t *testing.T) {
	for model, want := range map[string]bool{"glm-5.1:cloud": true, "kimi-k2:1t-cloud": true, "qwen3.5:4b": false, "cloud": false} {
		if got := IsCloudModel(model); got != want {
			t.Errorf("IsCloudModel(%q) = %v", model, got)
		}
	}
}

func TestOllamaModelsAndPull(t *testing.T) {
	srv := server(t, nil, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"qwen3:4b"},{"name":"llama3.2:latest"}]}`)
		case "/api/pull":
			fmt.Fprintln(w, `{"status":"pulling manifest"}`)
			fmt.Fprintln(w, `{"status":"downloading","total":100,"completed":50}`)
			fmt.Fprintln(w, `{"status":"success"}`)
		}
	})
	ctx := context.Background()
	for model, want := range map[string]bool{"qwen3:4b": true, "llama3.2": true, "qwen3:8b": false} {
		if got, err := NewOllama(srv.URL, model).HasModel(ctx); err != nil || got != want {
			t.Errorf("HasModel(%s) = %v, %v", model, got, err)
		}
	}
	var statuses []string
	if err := NewOllama(srv.URL, "qwen3:4b").Pull(ctx, func(p PullProgress) { statuses = append(statuses, p.Status) }); err != nil {
		t.Fatal(err)
	}
	if strings.Join(statuses, ",") != "pulling manifest,downloading,success" {
		t.Errorf("progresso = %v", statuses)
	}
}

func TestOpenAIChat(t *testing.T) {
	var body map[string]any
	var auth string
	srv := server(t, &body, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": comentário keep-alive\n\n")
		for _, c := range []string{"Pague ", "o cartão."} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	streamed, full, err := collectChat(t, NewOpenAI(srv.URL+"/v1/", "chave", "qwen/qwen3-32b"))
	if err != nil || streamed != "Pague o cartão." || full != streamed {
		t.Fatalf("streamed=%q full=%q err=%v", streamed, full, err)
	}
	if auth != "Bearer chave" || body["stream"] != true || len(body["messages"].([]any)) != 4 {
		t.Errorf("auth=%q body=%v", auth, body)
	}
}

func TestOpenAIError(t *testing.T) {
	srv := server(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Invalid API Key","type":"invalid_request_error"}}`)
	})
	_, _, err := collectChat(t, NewOpenAI(srv.URL, "errada", "m"))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 401 || apiErr.Message != "Invalid API Key" {
		t.Errorf("err = %v", err)
	}
}

func TestAnthropicChat(t *testing.T) {
	var body map[string]any
	srv := server(t, &body, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, e := range []string{
			`{"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Pague o cartão."}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
			`{"type":"message_stop"}`,
		} {
			var typ struct{ Type string }
			json.Unmarshal([]byte(e), &typ)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ.Type, e)
		}
	})
	streamed, full, err := collectChat(t, NewAnthropic("chave", "claude-opus-5-5", srv.URL))
	if err != nil || streamed != "Pague o cartão." || full != streamed {
		t.Fatalf("streamed=%q full=%q err=%v", streamed, full, err)
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 3 || msgs[1].(map[string]any)["role"] != "assistant" || body["fallbacks"] != "default" {
		t.Errorf("corpo = %v", body)
	}
}

func TestNew(t *testing.T) {
	for _, cfg := range []Config{
		{Kind: KindOllama},                // sem modelo
		{Kind: KindOpenAI, Model: "m"},    // sem URL
		{Kind: KindAnthropic, Model: "m"}, // sem chave
		{Kind: "desconhecido", Model: "m"},
	} {
		if _, err := New(cfg); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("New(%+v): err = %v", cfg, err)
		}
	}
	if p, err := New(Config{Kind: KindOllama, Model: "qwen3:4b"}); err != nil || p.(*Ollama).baseURL != DefaultOllamaURL {
		t.Errorf("ollama padrão: %v %v", p, err)
	}
}
