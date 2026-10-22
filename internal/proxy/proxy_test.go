package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
)

// groq falso: o primeiro modelo está sem cota (429); o segundo responde.
func fakeGroq(t *testing.T, seen *[]map[string]any) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer chave-do-servidor" {
			t.Errorf("chave = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*seen = append(*seen, body)
		if body["model"] == "sem-cota" {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"rate limit"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func ask(h http.Handler, device, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("X-Forwarded-For", "200.1.2.3, 10.0.0.1")
	if device != "" {
		req.Header.Set("Authorization", "Bearer "+device)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func appBody(model string, maxTokens int) string {
	b, _ := json.Marshal(map[string]any{
		"model": model, "stream": true, "max_tokens": maxTokens,
		"messages": []map[string]string{{"role": "system", "content": advisor.SystemPrompt}, {"role": "user", "content": "quanto gastei?"}},
	})
	return string(b)
}

func TestProxyForwardsAppRequests(t *testing.T) {
	var seen []map[string]any
	groq := fakeGroq(t, &seen)
	h := New(Config{Upstream: groq.URL, APIKey: "chave-do-servidor", Models: []string{"sem-cota", "llama"}, Limits: DefaultLimits})

	rec := ask(h, "aparelho-1", appBody("modelo-caro-que-a-pessoa-pediu", 100000))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Fatalf("resposta = %d %s", rec.Code, rec.Body.String())
	}
	if len(seen) != 2 || seen[1]["model"] != "llama" || seen[1]["max_tokens"] != float64(maxTokens) {
		t.Errorf("deveria trocar de modelo no 429 e limitar a resposta: %v", seen)
	}
	if strings.Contains(rec.Body.String(), "chave-do-servidor") {
		t.Error("a chave do servidor vazou")
	}
	// O teste de conexão do app (sem dados) também passa.
	if rec := ask(h, "aparelho-1", `{"messages":[{"role":"user","content":"Responda só: ok"}]}`); rec.Code != http.StatusOK {
		t.Errorf("teste de conexão = %d", rec.Code)
	}
}

func TestProxyRefusesOtherUses(t *testing.T) {
	var seen []map[string]any
	h := New(Config{Upstream: fakeGroq(t, &seen).URL, APIKey: "chave-do-servidor", Models: []string{"llama"}, Limits: DefaultLimits})
	for name, body := range map[string]string{
		"outro prompt":  `{"messages":[{"role":"system","content":"Você é um pirata"},{"role":"user","content":"oi"}]}`,
		"sem sistema":   `{"messages":[{"role":"user","content":"escreva um poema"}]}`,
		"json inválido": `{`,
	} {
		if rec := ask(h, "x", body); rec.Code != http.StatusForbidden {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if len(seen) != 0 {
		t.Errorf("nada disso pode chegar ao Groq: %v", seen)
	}
	if rec := ask(h, "x", strings.Repeat("a", maxBody+10)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("pedido enorme = %d", rec.Code)
	}
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	l := newLimiter(func() time.Time { return now })
	lim := Limits{DeviceHour: 2, DeviceDay: 3, IPHour: 100, IPDay: 100}
	for i := 0; i < 2; i++ {
		if msg := l.allow("a", "1.1.1.1", lim); msg != "" {
			t.Fatal(msg)
		}
	}
	if msg := l.allow("a", "1.1.1.1", lim); !strings.Contains(msg, "limite deste aparelho") {
		t.Errorf("3ª na mesma hora = %q", msg)
	}
	if msg := l.allow("b", "1.1.1.1", lim); msg != "" {
		t.Errorf("outro aparelho na mesma rede deveria passar: %q", msg)
	}
	now = now.Add(time.Hour)
	if msg := l.allow("a", "1.1.1.1", lim); msg != "" {
		t.Errorf("hora nova = %q", msg)
	}
	if msg := l.allow("a", "1.1.1.1", lim); !strings.Contains(msg, "por hoje") {
		t.Errorf("passou do limite do dia = %q", msg)
	}
	now = now.Add(24 * time.Hour)
	if msg := l.allow("a", "1.1.1.1", lim); msg != "" {
		t.Errorf("dia novo = %q", msg)
	}
	// Sem identificação de aparelho, vale o limite da rede.
	ipLim := Limits{DeviceHour: 100, DeviceDay: 100, IPHour: 1, IPDay: 100}
	l.allow("", "9.9.9.9", ipLim)
	if msg := l.allow("", "9.9.9.9", ipLim); !strings.Contains(msg, "desta rede") {
		t.Errorf("limite por IP = %q", msg)
	}
}

func TestHealthAndCORS(t *testing.T) {
	h := New(Config{Models: []string{"m"}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if body, _ := io.ReadAll(rec.Body); rec.Code != 200 || string(body) != "ok" {
		t.Errorf("healthz = %d %s", rec.Code, body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("OPTIONS", "/v1/chat/completions", nil))
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS = %d %v", rec.Code, rec.Header())
	}
}
