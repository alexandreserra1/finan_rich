package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
)

// fakeProvider é um provedor no formato OpenAI: lista models em /models (nil:
// a lista falha), confere a chave e responde 429 aos modelos em full.
type fakeProvider struct {
	key          string
	models       []string
	full         map[string]bool
	status       int    // se diferente de 0, toda conversa responde isto
	noStreamOpts bool   // recusa (400) pedido com stream_options, como provedor que não conhece o campo
	reply        string // corpo da resposta (padrão: streaming com usage no fim)
	ctype        string

	mu   sync.Mutex
	seen []map[string]any
}

func (f *fakeProvider) start(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.key {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/models") {
			if f.models == nil {
				http.NotFound(w, r)
				return
			}
			data := []map[string]string{}
			for _, m := range f.models {
				data = append(data, map[string]string{"id": m})
			}
			json.NewEncoder(w).Encode(map[string]any{"data": data})
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.seen = append(f.seen, body)
		f.mu.Unlock()
		if _, has := body["stream_options"]; has && f.noStreamOpts {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"unknown field stream_options"}}`)
			return
		}
		if f.status != 0 {
			w.WriteHeader(f.status)
			fmt.Fprint(w, `{"error":{"message":"falhou"}}`)
			return
		}
		if f.full[fmt.Sprint(body["model"])] {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"rate limit"}}`)
			return
		}
		reply, ctype := f.reply, f.ctype
		if reply == "" {
			reply, ctype = "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"+
				"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1234,\"completion_tokens\":56,\"total_tokens\":1290}}\n\n"+
				"data: [DONE]\n\n", "text/event-stream"
		}
		w.Header().Set("Content-Type", ctype)
		fmt.Fprint(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (f *fakeProvider) models_() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, b := range f.seen {
		out = append(out, fmt.Sprint(b["model"]))
	}
	return out
}

// captureLog troca a saída do log durante o teste.
func captureLog(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	prev := log.Writer()
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func personaBody(persona string, stream bool) string {
	system := advisor.SystemPrompt + "\n\nQuem fala:\nVocê é " + persona + ", o especialista. Firme."
	b, _ := json.Marshal(map[string]any{
		"stream": stream, "max_tokens": 100,
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": "quanto devo no cartão nubank?"}},
	})
	return string(b)
}

// usageLines devolve as linhas "ia {...}" do log, já lidas.
func usageLines(t *testing.T, logs string) []map[string]any {
	var out []map[string]any
	for _, line := range strings.Split(logs, "\n") {
		_, js, ok := strings.Cut(line, "ia {")
		if !ok {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte("{"+js), &m); err != nil {
			t.Fatalf("linha de log inválida %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

// B3: cada resposta do provedor vira uma linha com tokens, modelo e
// personagem, sem nada do texto da pessoa e sem o id cru do aparelho.
func TestProxyLogsTokensPerRequest(t *testing.T) {
	logs := captureLog(t)
	groq := &fakeProvider{key: "k-groq", full: map[string]bool{"sem-cota": true}}
	h := New(Config{Providers: []Provider{{Name: "groq", Upstream: groq.start(t).URL, APIKey: "k-groq", Models: []string{"sem-cota", "openai/gpt-oss-120b"}, StreamUsage: true}}, Limits: DefaultLimits})

	rec := ask(h, "aparelho-secreto-1", personaBody("o Bento", true))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Fatalf("resposta = %d %s", rec.Code, rec.Body.String())
	}
	if so, _ := groq.seen[1]["stream_options"].(map[string]any); so["include_usage"] != true {
		t.Errorf("no streaming deveria pedir o usage: %v", groq.seen[1])
	}
	out := logs.String()
	for _, secret := range []string{"aparelho-secreto-1", "nubank", "quanto devo", "k-groq"} {
		if strings.Contains(out, secret) {
			t.Errorf("o log não pode ter %q:\n%s", secret, out)
		}
	}
	lines := usageLines(t, out)
	if len(lines) != 2 {
		t.Fatalf("esperava 2 linhas (429 e a resposta), veio:\n%s", out)
	}
	if l := lines[0]; l["status"] != float64(429) || l["modelo"] != "sem-cota" {
		t.Errorf("tentativa sem cota = %v", l)
	}
	l := lines[1]
	if l["provedor"] != "groq" || l["modelo"] != "openai/gpt-oss-120b" || l["status"] != float64(200) ||
		l["entrada"] != float64(1234) || l["saida"] != float64(56) || l["personagem"] != "bento" {
		t.Errorf("linha = %v", l)
	}
	if dev, _ := l["aparelho"].(string); len(dev) != 12 || dev != lines[0]["aparelho"] {
		t.Errorf("aparelho deveria ser um hash curto e estável: %v", l["aparelho"])
	}
	if _, ok := l["ms"].(float64); !ok {
		t.Errorf("falta o tempo: %v", l)
	}
}

func TestProxyLogsUsageWithoutStreaming(t *testing.T) {
	logs := captureLog(t)
	p := &fakeProvider{key: "k", ctype: "application/json",
		reply: `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":10,"completion_tokens":3}}`}
	h := New(Config{Providers: []Provider{{Name: "cerebras", Upstream: p.start(t).URL, APIKey: "k", Models: []string{"gpt-oss-120b"}}}, Limits: DefaultLimits})
	ask(h, "", personaBody("a Luna", false))
	if _, has := p.seen[0]["stream_options"]; has {
		t.Errorf("sem streaming não pede stream_options: %v", p.seen[0])
	}
	if p.seen[0]["reasoning_effort"] != "low" {
		t.Errorf("gpt-oss de qualquer provedor vai com raciocínio baixo: %v", p.seen[0])
	}
	lines := usageLines(t, logs.String())
	if len(lines) != 1 || lines[0]["entrada"] != float64(10) || lines[0]["saida"] != float64(3) || lines[0]["personagem"] != "luna" || lines[0]["aparelho"] != "" {
		t.Errorf("linhas = %v", lines)
	}
}

func TestPersonaOf(t *testing.T) {
	for system, want := range map[string]string{
		advisor.SystemPrompt: "consultor",
		advisor.SystemPrompt + "\n\nVocê é o Otávio, o da análise": "otavio",
		advisor.SystemPrompt + "\n\nVocê é a Jade, a consultora":   "jade",
		advisor.CategorizeMarker + " nas categorias":               "categorize",
		"": "ping",
	} {
		msgs := []chatMessage{{Role: "user", Content: "oi"}}
		if system != "" {
			msgs = []chatMessage{{Role: "system", Content: system}}
		}
		if got := personaOf(msgs); got != want {
			t.Errorf("personaOf(%.40q) = %q, quer %q", system, got, want)
		}
	}
}

// B4: os modelos de um provedor primeiro; esgotados, o próximo provedor, cada
// um com a sua chave.
func TestProxyFallsToNextProvider(t *testing.T) {
	captureLog(t)
	groq := &fakeProvider{key: "k-groq", models: []string{"g1", "g2"}, full: map[string]bool{"g1": true, "g2": true}}
	gemini := &fakeProvider{key: "k-gemini", models: []string{"models/gemini-3.8-flash", "models/text-embedding-004"}}
	h := New(Config{Providers: []Provider{
		{Name: "groq", Upstream: groq.start(t).URL, APIKey: "k-groq", Models: []string{"g1", "g2"}},
		{Name: "gemini", Upstream: gemini.start(t).URL, APIKey: "k-gemini", Models: []string{"gemini-3.8-flash"}},
	}, Limits: DefaultLimits})
	rec := ask(h, "a", personaBody("o Tito", true))
	if rec.Code != http.StatusOK {
		t.Fatalf("resposta = %d %s", rec.Code, rec.Body.String())
	}
	if got := strings.Join(groq.models_(), ","); got != "g1,g2" {
		t.Errorf("groq tentou %s", got)
	}
	if got := strings.Join(gemini.models_(), ","); got != "gemini-3.8-flash" {
		t.Errorf("gemini tentou %s", got)
	}
	if gemini.seen[0]["reasoning_effort"] != nil {
		t.Errorf("reasoning_effort só para gpt-oss: %v", gemini.seen[0])
	}
}

// Chave errada ou provedor fora do ar não derrubam o consultor: pula para o
// próximo provedor sem gastar tentativas nos outros modelos do mesmo.
func TestProxySkipsBrokenProvider(t *testing.T) {
	captureLog(t)
	bad := &fakeProvider{key: "outra"}
	ok := &fakeProvider{key: "k2"}
	h := New(Config{Providers: []Provider{
		{Name: "groq", Upstream: bad.start(t).URL, APIKey: "k1", Models: []string{"a", "b"}},
		{Name: "cerebras", Upstream: "http://127.0.0.1:1", APIKey: "k", Models: []string{"c"}}, // não conecta
		{Name: "gemini", Upstream: ok.start(t).URL, APIKey: "k2", Models: []string{"d"}},
	}, Limits: DefaultLimits})
	rec := ask(h, "a", personaBody("o Bento", false))
	if rec.Code != http.StatusOK || len(ok.seen) != 1 {
		t.Fatalf("resposta = %d %s; tentativas no bom: %d", rec.Code, rec.Body.String(), len(ok.seen))
	}
	// Todos falhando: a pessoa recebe o erro do último.
	h = New(Config{Providers: []Provider{{Name: "groq", Upstream: bad.start(t).URL, APIKey: "k1", Models: []string{"a"}}}, Limits: DefaultLimits})
	if rec := ask(h, "a", personaBody("o Bento", false)); rec.Code != http.StatusUnauthorized {
		t.Errorf("só um provedor, chave errada = %d", rec.Code)
	}
}

// OpenRouter: só modelos grátis (":free" e o roteador openrouter/free).
func TestProxyOpenRouterOnlyFree(t *testing.T) {
	captureLog(t)
	or := &fakeProvider{key: "k", models: []string{"anthropic/caro", "meta/llama:free", "openrouter/free"}, full: map[string]bool{"meta/llama:free": true}}
	h := New(Config{Providers: []Provider{{Name: "openrouter", Upstream: or.start(t).URL, APIKey: "k", Models: []string{"openrouter/free"}, OnlyFree: true}}, Limits: DefaultLimits})
	ask(h, "a", personaBody("o Bento", true))
	if got := strings.Join(or.models_(), ","); got != "openrouter/free" {
		t.Errorf("tentou %s; só os grátis, preferido primeiro, e para no primeiro que responde", got)
	}
	or.full["openrouter/free"] = true
	or.seen = nil
	h = New(Config{Providers: []Provider{{Name: "openrouter", Upstream: or.start(t).URL, APIKey: "k", Models: []string{"openrouter/free"}, OnlyFree: true}}, Limits: DefaultLimits})
	ask(h, "a", personaBody("o Bento", true))
	if got := strings.Join(or.models_(), ","); got != "openrouter/free,meta/llama:free" {
		t.Errorf("tentou %s", got)
	}
}

func TestConfigFromEnvProviders(t *testing.T) {
	for _, k := range []string{"GROQ_API_KEY", "GEMINI_API_KEY", "CEREBRAS_API_KEY", "OPENROUTER_API_KEY", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID", "FINAN_PROVIDERS", "FINAN_PROXY_UPSTREAM", "FINAN_PROXY_MODELS"} {
		t.Setenv(k, "")
	}
	if _, err := ConfigFromEnv(); err == nil {
		t.Error("sem nenhuma chave deveria dar erro")
	}

	// Só o Groq: como antes.
	t.Setenv("GROQ_API_KEY", "g")
	cfg, err := ConfigFromEnv()
	if err != nil || len(cfg.Providers) != 1 {
		t.Fatalf("só groq: %v %+v", err, cfg.Providers)
	}
	if p := cfg.Providers[0]; p.Name != "groq" || p.Upstream != "https://api.groq.com/openai/v1" || p.APIKey != "g" || strings.Join(p.Models, ",") != defaultModels {
		t.Errorf("groq = %+v", p)
	}

	t.Setenv("GEMINI_API_KEY", "ge")
	t.Setenv("CEREBRAS_API_KEY", "c")
	t.Setenv("OPENROUTER_API_KEY", "o")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf") // sem o ACCOUNT_ID fica de fora
	cfg, _ = ConfigFromEnv()
	urls := map[string]string{
		"gemini":   "https://generativelanguage.googleapis.com/v1beta/openai",
		"cerebras": "https://api.cerebras.ai/v1",
	}
	var names []string
	for _, p := range cfg.Providers {
		names = append(names, p.Name)
		if u := urls[p.Name]; u != "" && p.Upstream != u {
			t.Errorf("%s = %s", p.Name, p.Upstream)
		}
	}
	if got := strings.Join(names, ","); got != "groq,gemini,cerebras,openrouter" {
		t.Errorf("ordem padrão = %s", got)
	}
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "abc123")
	t.Setenv("FINAN_PROVIDERS", "cloudflare, openrouter,inventado,groq")
	cfg, _ = ConfigFromEnv()
	want := map[string]string{
		"cloudflare": "https://api.cloudflare.com/client/v4/accounts/abc123/ai/v1",
		"openrouter": "https://openrouter.ai/api/v1",
		"groq":       "https://api.groq.com/openai/v1",
	}
	names = nil
	for _, p := range cfg.Providers {
		names = append(names, p.Name)
		if p.Upstream != want[p.Name] || len(p.Models) == 0 {
			t.Errorf("%s = %+v", p.Name, p)
		}
		if p.Name == "openrouter" && !p.OnlyFree {
			t.Error("openrouter só com modelos grátis")
		}
	}
	if got := strings.Join(names, ","); got != "cloudflare,openrouter,groq" {
		t.Errorf("ordem de FINAN_PROVIDERS = %s", got)
	}
}

// Um provedor que não conhece stream_options responde 400: a mesma pergunta
// tem que passar logo em seguida sem o campo, no mesmo modelo, e não gastar
// os outros modelos dele com o mesmo pedido que vai falhar de novo.
func TestProxyRetriesWithoutStreamOptions(t *testing.T) {
	captureLog(t)
	p := &fakeProvider{key: "k", models: []string{"m1", "m2", "m3"}, noStreamOpts: true}
	h := New(Config{Providers: []Provider{{Name: "groq", Upstream: p.start(t).URL, APIKey: "k", Models: []string{"m1", "m2", "m3"}, StreamUsage: true}}, Limits: DefaultLimits})
	rec := ask(h, "a", personaBody("o Bento", true))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"content":"ok"`) {
		t.Fatalf("resposta = %d %s", rec.Code, rec.Body.String())
	}
	if got := strings.Join(p.models_(), ","); got != "m1,m1" {
		t.Errorf("tentativas = %s; quer o m1 duas vezes (com e sem stream_options)", got)
	}
	if _, has := p.seen[1]["stream_options"]; has {
		t.Error("a segunda tentativa ainda leva stream_options")
	}
	// Na próxima pergunta o proxy já lembra e nem tenta o campo.
	p.mu.Lock()
	p.seen = nil
	p.mu.Unlock()
	ask(h, "a", personaBody("o Bento", true))
	if got := strings.Join(p.models_(), ","); got != "m1" {
		t.Errorf("segunda pergunta tentou %s; quer só m1, sem stream_options", got)
	}
}

// Tudo esgotado: o proxy desiste depois de maxAttempts tentativas (cada uma
// custa tempo de quem espera), em vez de passar por todos os modelos de todos os provedores.
func TestProxyCapsAttempts(t *testing.T) {
	captureLog(t)
	var provs []Provider
	var fakes []*fakeProvider
	for i := 0; i < 5; i++ {
		models := []string{"a", "b", "c", "d", "e", "f"}
		f := &fakeProvider{key: "k", models: models, status: http.StatusTooManyRequests}
		fakes = append(fakes, f)
		provs = append(provs, Provider{Name: fmt.Sprintf("p%d", i), Upstream: f.start(t).URL, APIKey: "k", Models: models})
	}
	h := New(Config{Providers: provs, Limits: DefaultLimits})
	rec := ask(h, "a", personaBody("a Luna", false))
	total := 0
	for _, f := range fakes {
		total += len(f.models_())
	}
	if total != maxAttempts || rec.Code != http.StatusTooManyRequests {
		t.Errorf("tentativas = %d (quer %d), resposta = %d", total, maxAttempts, rec.Code)
	}
}
