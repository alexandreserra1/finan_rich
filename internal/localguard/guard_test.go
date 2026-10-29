package localguard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const key = "chave-do-mac-com-mais-de-24-letras"

// fakeOllama imita o endpoint OpenAI do Ollama e guarda o que recebeu.
func fakeOllama(t *testing.T, delay time.Duration) (*httptest.Server, *[]map[string]any, *[]string) {
	var mu sync.Mutex
	var bodies []map[string]any
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		bodies = append(bodies, b)
		mu.Unlock()
		time.Sleep(delay)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies, &paths
}

func guard(t *testing.T, ollama string, busy int) http.Handler {
	h, err := New(Config{Key: key, Ollama: ollama, Model: "qwen2.5:7b-instruct", MaxBusy: busy})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func do(h http.Handler, method, path, auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGuardNeedsTheKey(t *testing.T) {
	ollama, bodies, _ := fakeOllama(t, 0)
	h := guard(t, ollama.URL, 2)
	for _, auth := range []string{"", "errada", key + "x"} {
		if rec := do(h, "POST", "/v1/chat/completions", auth, `{"messages":[]}`); rec.Code != http.StatusUnauthorized {
			t.Errorf("chave %q: %d", auth, rec.Code)
		}
	}
	if len(*bodies) != 0 {
		t.Error("sem a chave certa, nada pode chegar ao Ollama")
	}
	if _, err := New(Config{Key: "curta", Ollama: ollama.URL, Model: "m"}); err == nil {
		t.Error("chave curta deveria ser recusada")
	}
}

// Só a conversa passa, e sempre com o modelo escolhido aqui (quem chama não
// escolhe outro, nem chega em /api/pull, /api/delete etc. do Ollama).
func TestGuardOnlyChatWithTheChosenModel(t *testing.T) {
	ollama, bodies, paths := fakeOllama(t, 0)
	h := guard(t, ollama.URL, 2)
	rec := do(h, "POST", "/v1/chat/completions", key, `{"model":"llama-gigante","stream":true,"reasoning_effort":"low","messages":[{"role":"user","content":"oi"}]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"content":"ok"`) {
		t.Fatalf("conversa = %d %s", rec.Code, rec.Body.String())
	}
	if got := (*bodies)[0]["model"]; got != "qwen2.5:7b-instruct" {
		t.Errorf("modelo repassado = %v", got)
	}
	if _, has := (*bodies)[0]["reasoning_effort"]; has {
		t.Error("reasoning_effort não vale para o Qwen local")
	}
	for _, p := range []string{"/api/pull", "/api/delete", "/api/chat", "/v1/embeddings", "/"} {
		if rec := do(h, "POST", p, key, `{}`); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d", p, rec.Code)
		}
	}
	if strings.Join(*paths, ",") != "POST /v1/chat/completions" {
		t.Errorf("chegou ao Ollama: %v", *paths)
	}
	// A lista de modelos mostra só o escolhido (o proxy da nuvem pergunta).
	rec = do(h, "GET", "/v1/models", key, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"qwen2.5:7b-instruct"`) || strings.Count(rec.Body.String(), `"id"`) != 1 {
		t.Errorf("modelos = %d %s", rec.Code, rec.Body.String())
	}
}

// Um computador responde uma pergunta por vez: com a fila cheia, avisa logo
// (429) e o proxy da nuvem passa adiante, em vez de deixar a pessoa esperando.
func TestGuardBusy(t *testing.T) {
	ollama, _, _ := fakeOllama(t, 300*time.Millisecond)
	h := guard(t, ollama.URL, 1)
	done := make(chan int)
	go func() { done <- do(h, "POST", "/v1/chat/completions", key, `{"messages":[]}`).Code }()
	time.Sleep(50 * time.Millisecond)
	if rec := do(h, "POST", "/v1/chat/completions", key, `{"messages":[]}`); rec.Code != http.StatusTooManyRequests {
		t.Errorf("segunda pergunta com o Mac ocupado = %d", rec.Code)
	}
	if code := <-done; code != http.StatusOK {
		t.Errorf("primeira = %d", code)
	}
}
