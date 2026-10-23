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
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// groq falso: o primeiro modelo está sem cota (429); o segundo responde.
// fakeModels é o que o Groq falso diz ter em /models (vazio: a lista falha).
var fakeModels []string

func fakeGroq(t *testing.T, seen *[]map[string]any) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			if fakeModels == nil {
				http.NotFound(w, r)
				return
			}
			data := []map[string]string{}
			for _, m := range fakeModels {
				data = append(data, map[string]string{"id": m})
			}
			json.NewEncoder(w).Encode(map[string]any{"data": data})
			return
		}
		if r.Header.Get("Authorization") != "Bearer chave-do-servidor" {
			t.Errorf("chave = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*seen = append(*seen, body)
		if body["model"] == "aposentado" { // como o Groq responde a um modelo desligado
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"The model has been decommissioned","code":"model_decommissioned"}}`)
			return
		}
		if body["model"] == "sumiu" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"model not found","code":"model_not_found"}}`)
			return
		}
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
	// O teste de conexão (celular e botão "Testar" do computador) e a
	// categorização de extrato também passam.
	if rec := ask(h, "aparelho-1", `{"messages":[{"role":"user","content":"`+llm.PingMessage+`"}]}`); rec.Code != http.StatusOK {
		t.Errorf("teste de conexão = %d", rec.Code)
	}
	cat, _ := json.Marshal(map[string]any{"messages": []map[string]string{{"role": "system", "content": advisor.CategorizeMarker + " ..."}, {"role": "user", "content": "1. ifood"}}})
	if rec := ask(h, "aparelho-1", string(cat)); rec.Code != http.StatusOK {
		t.Errorf("categorização = %d", rec.Code)
	}
}

// O Groq desliga modelos com frequência: um modelo que sumiu não pode
// derrubar o consultor; o intermediário passa para o próximo da lista.
func TestProxySkipsRetiredModels(t *testing.T) {
	var seen []map[string]any
	h := New(Config{Upstream: fakeGroq(t, &seen).URL, APIKey: "chave-do-servidor", Models: []string{"aposentado", "sumiu", "llama"}, Limits: DefaultLimits})
	rec := ask(h, "aparelho-1", appBody("x", 100))
	if rec.Code != http.StatusOK || len(seen) != 3 || seen[2]["model"] != "llama" {
		t.Fatalf("resposta %d, modelos tentados %d: %s", rec.Code, len(seen), rec.Body.String())
	}
}

// O intermediário pergunta ao Groq quais modelos existem e pula direto os
// que saíram do ar; os gpt-oss vão com pouco raciocínio (gasta menos cota).
func TestProxyUsesAvailableModels(t *testing.T) {
	fakeModels = []string{"whisper-large-v3", "openai/gpt-oss-120b", "llama"}
	t.Cleanup(func() { fakeModels = nil })
	var seen []map[string]any
	h := New(Config{Upstream: fakeGroq(t, &seen).URL, APIKey: "chave-do-servidor", Models: []string{"aposentado", "openai/gpt-oss-120b", "llama"}, Limits: DefaultLimits})
	if rec := ask(h, "aparelho-1", appBody("x", 100)); rec.Code != http.StatusOK {
		t.Fatalf("resposta %d %s", rec.Code, rec.Body.String())
	}
	if len(seen) != 1 || seen[0]["model"] != "openai/gpt-oss-120b" || seen[0]["reasoning_effort"] != "low" {
		t.Errorf("deveria ir direto no gpt-oss com raciocínio baixo: %v", seen)
	}
	seen = nil
	h = New(Config{Upstream: fakeGroq(t, &seen).URL, APIKey: "chave-do-servidor", Models: []string{"llama"}, Limits: DefaultLimits})
	ask(h, "aparelho-1", appBody("x", 100))
	if len(seen) != 1 || seen[0]["reasoning_effort"] != nil {
		t.Errorf("modelo sem raciocínio configurável não recebe reasoning_effort: %v", seen)
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

// O começo do X-Forwarded-For é do cliente (pode ser inventado); o Google
// acrescenta o IP verdadeiro no fim. Inventar IPs não burla o limite, nem sem
// identificação de aparelho.
func TestProxySpoofedForwardedFor(t *testing.T) {
	var seen []map[string]any
	h := New(Config{Upstream: fakeGroq(t, &seen).URL, APIKey: "chave-do-servidor", Models: []string{"llama"},
		Limits: Limits{DeviceHour: 100, DeviceDay: 100, IPHour: 3, IPDay: 100}})
	ok := 0
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(appBody("m", 10)))
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("9.9.9.%d, 177.10.20.30, 35.191.1.2", i)) // inventado, real, Google
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			ok++
		}
	}
	if ok != 3 {
		t.Errorf("passaram %d de 10 com IPs inventados; o limite da rede real é 3", ok)
	}
}

// Uma rede não cria aparelhos sem fim (cada um ocuparia memória do limitador):
// passando de maxDevicesPerIP no dia, os novos dividem o balde da rede.
func TestLimiterCapsDevicesPerIP(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	l := newLimiter(func() time.Time { return now })
	lim := Limits{DeviceHour: 1 << 30, DeviceDay: 1 << 30, IPHour: 1 << 30, IPDay: 1 << 30}
	for i := range 300 {
		l.allow(fmt.Sprintf("falso%d", i), "1.1.1.1", lim)
	}
	if n := len(l.counts); n > maxDevicesPerIP+2 {
		t.Errorf("uma rede criou %d chaves", n)
	}
	// IPv6: o mesmo /64 é uma rede só.
	l.allow("x", "2001:db8:1:2::1", lim)
	l.allow("y", "2001:db8:1:2:ffff::9", lim)
	if l.counts["ip:2001:db8:1:2::/64"] == nil || l.counts["ip:2001:db8:1:2::/64"].inDay != 2 {
		t.Error("endereços do mesmo /64 deveriam contar juntos")
	}
}

func TestLimiterFullDoesNotReset(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	l := newLimiter(func() time.Time { return now })
	lim := Limits{DeviceHour: 1, DeviceDay: 1, IPHour: 1000, IPDay: 1000}
	l.allow("barrado", "1.1.1.1", lim)
	for i := 0; len(l.counts) < maxKeys; i++ {
		l.allow(fmt.Sprintf("a%d", i), fmt.Sprintf("10.%d.%d.%d", i>>16&255, i>>8&255, i&255), Limits{DeviceHour: 9, DeviceDay: 9, IPHour: 1 << 30, IPDay: 1 << 30})
	}
	if msg := l.allow("barrado", "1.1.1.1", lim); msg == "" {
		t.Error("encher o limitador soltou quem estava barrado")
	}
	if msg := l.allow("novo", "2.2.2.2", lim); !strings.Contains(msg, "movimento demais") {
		t.Errorf("com a memória cheia, gente nova espera: %q", msg)
	}
	now = now.Add(24 * time.Hour)
	if msg := l.allow("novo", "2.2.2.2", lim); msg != "" {
		t.Errorf("no dia seguinte a memória é liberada: %q", msg)
	}
}
