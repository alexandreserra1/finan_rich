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
	"time"
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
	reply, err := p.Chat(context.Background(), testReq, func(s string) { b.WriteString(s) })
	return b.String(), reply.Text, err
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

func TestOllamaPullError(t *testing.T) {
	srv := server(t, nil, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"status":"pulling manifest"}`)
		fmt.Fprintln(w, `{"error":"pull model manifest: file does not exist"}`)
	})
	err := NewOllama(srv.URL, "qwen9:1b").Pull(context.Background(), func(PullProgress) {})
	if err == nil || !strings.Contains(err.Error(), "não foi possível baixar o modelo \"qwen9:1b\"") {
		t.Errorf("err = %v", err)
	}
}

func TestOllamaSlowIsNotOffline(t *testing.T) {
	srv := server(t, nil, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond) // "carregando o modelo"
	})
	old := httpClient.Transport.(*http.Transport).ResponseHeaderTimeout
	httpClient.Transport.(*http.Transport).ResponseHeaderTimeout = 50 * time.Millisecond
	defer func() { httpClient.Transport.(*http.Transport).ResponseHeaderTimeout = old }()

	if _, _, err := collectChat(t, NewOllama(srv.URL, "m")); !errors.Is(err, ErrOllamaSlow) {
		t.Errorf("demora deve virar ErrOllamaSlow, não 'não instalado': %v", err)
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

// Conversa com ferramentas: o modelo pediu uma, o app respondeu, e a pergunta
// segue. Cada provedor tem seu formato; aqui confere ida e volta.
var toolReq = Request{
	System: "sistema",
	Tools: []Tool{{Name: "buscar", Description: "busca lançamentos", Parameters: json.RawMessage(
		`{"type":"object","properties":{"texto":{"type":"string"}},"required":["texto"]}`)}},
	Messages: []Message{
		{Role: RoleUser, Content: "quanto gastei com ifood?"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "buscar", Args: json.RawMessage(`{"texto":"ifood"}`),
			Extra: json.RawMessage(`{"google":{"thought_signature":"sig"}}`)}}},
		{Role: RoleTool, ToolCallID: "c1", ToolName: "buscar", Content: "total R$ 50,00"},
	},
}

func TestOllamaTools(t *testing.T) {
	var body map[string]any
	srv := server(t, &body, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"buscar","arguments":{"texto":"uber"}}}]},"done":false}`)
		fmt.Fprintln(w, `{"message":{"role":"assistant","content":""},"done":true}`)
	})
	reply, err := NewOllama(srv.URL, "qwen3.5:9b").Chat(context.Background(), toolReq, func(string) {})
	if err != nil || len(reply.ToolCalls) != 1 || reply.ToolCalls[0].Name != "buscar" || string(reply.ToolCalls[0].Args) != `{"texto":"uber"}` {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	msgs := body["messages"].([]any)
	asked := msgs[2].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)["function"].(map[string]any)
	result := msgs[3].(map[string]any)
	tools := body["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)
	if asked["name"] != "buscar" || asked["arguments"].(map[string]any)["texto"] != "ifood" ||
		result["role"] != "tool" || result["tool_name"] != "buscar" || tools["name"] != "buscar" {
		t.Errorf("corpo = %v", body)
	}
}

func TestOpenAITools(t *testing.T) {
	var body map[string]any
	srv := server(t, &body, func(w http.ResponseWriter, r *http.Request) {
		for _, d := range []string{
			`{"tool_calls":[{"index":0,"id":"x1","type":"function","function":{"name":"buscar","arguments":""},"extra_content":{"google":{"thought_signature":"s2"}}}]}`,
			`{"tool_calls":[{"index":0,"function":{"arguments":"{\"texto\":"}}]}`,
			`{"tool_calls":[{"index":0,"function":{"arguments":"\"uber\"}"}}]}`,
		} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":%s}]}\n\n", d)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	reply, err := NewOpenAI(srv.URL, "", "m").Chat(context.Background(), toolReq, func(string) {})
	if err != nil || len(reply.ToolCalls) != 1 || reply.ToolCalls[0].ID != "x1" || string(reply.ToolCalls[0].Args) != `{"texto":"uber"}` ||
		string(reply.ToolCalls[0].Extra) != `{"google":{"thought_signature":"s2"}}` {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	msgs := body["messages"].([]any)
	asked := msgs[2].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
	// Gemini: a assinatura do pedido de ferramenta volta junto, senão ele recusa (400).
	sig, _ := json.Marshal(asked["extra_content"])
	if asked["id"] != "c1" || asked["function"].(map[string]any)["arguments"] != `{"texto":"ifood"}` || string(sig) != `{"google":{"thought_signature":"sig"}}` ||
		msgs[3].(map[string]any)["tool_call_id"] != "c1" || body["tools"] == nil {
		t.Errorf("corpo = %v", body)
	}
}

func TestAnthropicTools(t *testing.T) {
	var body map[string]any
	srv := server(t, &body, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, e := range []string{
			`{"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"buscar","input":{}}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"texto\": \"uber\"}"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":3}}`,
			`{"type":"message_stop"}`,
		} {
			var typ struct{ Type string }
			json.Unmarshal([]byte(e), &typ)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ.Type, e)
		}
	})
	reply, err := NewAnthropic("chave", "claude-opus-5-5", srv.URL).Chat(context.Background(), toolReq, func(string) {})
	if err != nil || len(reply.ToolCalls) != 1 || reply.ToolCalls[0].ID != "t1" {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	var got map[string]any
	if json.Unmarshal(reply.ToolCalls[0].Args, &got); got["texto"] != "uber" {
		t.Errorf("args = %s", reply.ToolCalls[0].Args)
	}
	msgs := body["messages"].([]any)
	use := msgs[1].(map[string]any)["content"].([]any)[0].(map[string]any)
	res := msgs[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	tool := body["tools"].([]any)[0].(map[string]any)
	if use["type"] != "tool_use" || use["id"] != "c1" || res["type"] != "tool_result" || res["tool_use_id"] != "c1" ||
		tool["name"] != "buscar" || tool["input_schema"].(map[string]any)["required"] == nil {
		t.Errorf("corpo = %v", body)
	}
}

func TestToolsUnsupported(t *testing.T) {
	if !ToolsUnsupported(&APIError{Status: 400, Message: "registry.ollama.ai/library/gemma:2b does not support tools"}) {
		t.Error("deveria reconhecer modelo sem ferramentas")
	}
	if ToolsUnsupported(&APIError{Status: 500, Message: "tools"}) || ToolsUnsupported(errors.New("tools")) {
		t.Error("só erro 4xx do provedor conta")
	}
}

func TestIsLocal(t *testing.T) {
	for cfg, want := range map[Config]bool{
		{Kind: KindOllama, Model: "qwen3.5:9b"}:                                              true,
		{Kind: KindOllama, BaseURL: "http://192.168.0.10:11434", Model: "qwen3.5:9b"}:        true,
		{Kind: KindOllama, Model: "qwen3.5:cloud"}:                                           false,
		{Kind: KindOpenAI, BaseURL: "http://localhost:1234/v1", Model: "m"}:                  true,
		{Kind: KindOpenAI, BaseURL: "https://api.groq.com/openai/v1", Model: "m"}:            false,
		{Kind: KindOpenAI, BaseURL: "https://generativelanguage.googleapis.com", Model: "m"}: false,
		{Kind: KindAnthropic, Model: "claude-opus-5-5"}:                                      false,
	} {
		if got := IsLocal(cfg); got != want {
			t.Errorf("IsLocal(%+v) = %v", cfg, got)
		}
	}
}
