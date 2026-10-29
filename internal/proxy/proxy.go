// Package proxy é o intermediário do "finan_rich grátis": recebe as perguntas
// do consultor (já anonimizadas no aparelho) e repassa a provedores de IA
// grátis (Groq, Gemini, Cerebras, OpenRouter, Cloudflare) com as chaves que
// ficam só no servidor. Assim nenhuma chave vai dentro do app.
//
// Como o app é de código aberto, qualquer um acha o endereço. Por isso:
//   - só passa pedido com o formato do consultor (o prompt do finan_rich);
//   - limite de chamadas por aparelho e por IP, por hora e por dia;
//   - o modelo e o tamanho da resposta são escolhidos aqui, não por quem chama;
//   - quando um modelo esgota a cota grátis (429), cai ou foi desligado,
//     tenta o próximo da lista (no Groq cada modelo tem a sua cota); acabados
//     os modelos de um provedor, passa para o próximo provedor;
//   - cada resposta vira uma linha no log com tokens, modelo e personagem,
//     sem nada do texto da pessoa (usage.go).
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

// Config do intermediário.
type Config struct {
	Providers []Provider // em ordem de tentativa

	// Um provedor só, sem nome (o jeito antigo; vira "groq" se Providers está vazio).
	Upstream string   // ex.: https://api.groq.com/openai/v1
	APIKey   string   // chave do Groq (Secret Manager)
	Models   []string // em ordem de preferência
	Limits   Limits
	Client   *http.Client

	// Login com Google (oauth.go): sem os dois, as rotas respondem 404.
	GoogleClientID, GoogleClientSecret string
	GoogleTokenURL                     string // padrão: https://oauth2.googleapis.com/token (troca nos testes)
}

// Provider é uma API no formato OpenAI (/models e /chat/completions).
type Provider struct {
	Name        string   // aparece no log (groq, gemini...)
	Upstream    string   // endereço base, sem /chat/completions
	APIKey      string   // nunca vai para o log
	Models      []string // preferidos, em ordem
	OnlyFree    bool     // OpenRouter: só modelos ":free" (os outros cobram)
	StreamUsage bool     // aceita stream_options.include_usage (tokens no fim do streaming)
}

// Modelos abertos do Groq com ferramentas; cada um tem a sua cota grátis.
// O Groq desliga modelos com aviso de poucas semanas (console.groq.com/docs/deprecations):
// troque aqui ou em FINAN_PROXY_MODELS, sem publicar o app.
const defaultModels = "openai/gpt-oss-120b,qwen/qwen3.8-27b,openai/gpt-oss-20b"

// defaultProviders é a ordem padrão de FINAN_PROVIDERS.
const defaultProviders = "groq,gemini,cerebras,openrouter,cloudflare"

// presets: os provedores grátis no formato OpenAI. Os modelos preferidos vêm
// primeiro; depois, os outros de conversa que o /models do provedor listar.
var presets = map[string]Provider{
	"groq":   {Upstream: "https://api.groq.com/openai/v1", StreamUsage: true},
	"gemini": {Upstream: "https://generativelanguage.googleapis.com/v1beta/openai", StreamUsage: true, Models: []string{"gemini-3.8-flash", "gemini-flash-latest", "gemini-3.5-flash-lite"}},
	// Cerebras e Cloudflare: a documentação não fala de stream_options; o usage
	// é lido se vier, sem pedir (um campo desconhecido pode dar 400).
	"cerebras": {Upstream: "https://api.cerebras.ai/v1", Models: []string{"gpt-oss-120b", "qwen-3.8-27b"}},
	// O OpenRouter manda o usage no fim do streaming sozinho.
	"openrouter": {Upstream: "https://openrouter.ai/api/v1", OnlyFree: true, Models: []string{"openai/gpt-oss-120b:free", "openrouter/free"}},
	// Cloudflare não tem /models no formato OpenAI: fica a lista daqui (modelos
	// com ferramentas em developers.cloudflare.com/workers-ai/models).
	"cloudflare": {Upstream: "https://api.cloudflare.com/client/v4/accounts/%s/ai/v1", Models: []string{"@cf/zai-org/glm-4.7-flash"}},
}

// ConfigFromEnv lê a configuração das variáveis de ambiente (Cloud Run e Vercel).
// Cada provedor só entra se a chave dele existir; precisa de pelo menos um.
//
//	GROQ_API_KEY         chave do Groq
//	GEMINI_API_KEY       chave do Gemini (Google AI Studio)
//	CEREBRAS_API_KEY     chave da Cerebras
//	OPENROUTER_API_KEY   chave do OpenRouter (só modelos grátis)
//	CLOUDFLARE_API_TOKEN e CLOUDFLARE_ACCOUNT_ID: Workers AI
//	FINAN_PROVIDERS      ordem de tentativa (padrão: groq,gemini,cerebras,openrouter,cloudflare)
//	FINAN_PROXY_MODELS   modelos do Groq em ordem, separados por vírgula
//	FINAN_PROXY_UPSTREAM API do Groq (padrão: https://api.groq.com/openai/v1)
//	GOOGLE_CLIENT_ID     login com Google (ID do cliente "Aplicativo da Web")
//	GOOGLE_CLIENT_SECRET chave secreta desse cliente (só no servidor)
func ConfigFromEnv() (Config, error) {
	env := func(name, fallback string) string {
		if v := os.Getenv(name); v != "" {
			return v
		}
		return fallback
	}
	keys := map[string]string{
		"groq":       os.Getenv("GROQ_API_KEY"),
		"gemini":     os.Getenv("GEMINI_API_KEY"),
		"cerebras":   os.Getenv("CEREBRAS_API_KEY"),
		"openrouter": os.Getenv("OPENROUTER_API_KEY"),
		"cloudflare": os.Getenv("CLOUDFLARE_API_TOKEN"),
	}
	account := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	var providers []Provider
	for _, name := range strings.Split(env("FINAN_PROVIDERS", defaultProviders), ",") {
		name = strings.TrimSpace(name)
		p, ok := presets[name]
		if !ok || keys[name] == "" || name == "cloudflare" && account == "" ||
			slices.ContainsFunc(providers, func(q Provider) bool { return q.Name == name }) {
			continue
		}
		p.Name, p.APIKey = name, keys[name]
		switch name {
		case "groq":
			p.Upstream = env("FINAN_PROXY_UPSTREAM", p.Upstream)
			p.Models = strings.Split(env("FINAN_PROXY_MODELS", defaultModels), ",")
		case "cloudflare":
			p.Upstream = fmt.Sprintf(p.Upstream, url.PathEscape(account))
		}
		providers = append(providers, p)
	}
	if len(providers) == 0 {
		return Config{}, errors.New("falta a chave de algum provedor de IA (GROQ_API_KEY, GEMINI_API_KEY, CEREBRAS_API_KEY, OPENROUTER_API_KEY ou CLOUDFLARE_API_TOKEN com CLOUDFLARE_ACCOUNT_ID)")
	}
	return Config{
		Providers:          providers,
		Limits:             DefaultLimits,
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleTokenURL:     os.Getenv("GOOGLE_TOKEN_URL"),
	}, nil
}

// Limits são as chamadas permitidas (uma pergunta usa de 2 a 5).
type Limits struct{ DeviceHour, DeviceDay, IPHour, IPDay int }

// DefaultLimits dividem a cota grátis do Groq (uns 200 mil tokens por dia por
// modelo; cada chamada do consultor leva uns 4 mil): perto de 10 perguntas por
// aparelho por dia, para um aparelho só não gastar a cota de todo mundo.
var DefaultLimits = Limits{DeviceHour: 15, DeviceDay: 40, IPHour: 30, IPDay: 100}

const (
	maxModels = 6         // além disso, o pedido demora demais pulando modelos
	maxBody   = 128 << 10 // cabe a conversa longa com ferramentas; a cota grátis por minuto do Groq é menor ainda
	maxTokens = 2048
)

// Paths são os caminhos que o intermediário atende; no mesmo programa, o resto
// é do site (cmd/api).
var Paths = []string{"/healthz", "/chat/completions", "/v1/chat/completions", "/oauth/config", "/oauth/token", "/oauth/refresh"}

// New devolve o handler HTTP do intermediário.
func New(cfg Config) http.Handler {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 3 * time.Minute}
	}
	if len(cfg.Providers) == 0 && cfg.Upstream != "" {
		cfg.Providers = []Provider{{Name: "groq", Upstream: cfg.Upstream, APIKey: cfg.APIKey, Models: cfg.Models, StreamUsage: true}}
	}
	h := &handler{cfg: cfg, limiter: newLimiter(time.Now), oauthLimiter: newLimiter(time.Now), models: map[string]cachedModels{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })
	mux.HandleFunc("POST /chat/completions", h.chat)
	mux.HandleFunc("POST /v1/chat/completions", h.chat)
	mux.HandleFunc("GET /oauth/config", h.oauthConfig)
	mux.HandleFunc("POST /oauth/token", h.oauthToken)
	mux.HandleFunc("POST /oauth/refresh", h.oauthRefresh)
	return cors(mux)
}

type handler struct {
	cfg          Config
	limiter      *limiter
	oauthLimiter *limiter // separado: entrar com Google não gasta a cota do consultor

	mu     sync.Mutex
	models map[string]cachedModels // por provedor: os modelos que ele ainda tem
}

type cachedModels struct {
	ids []string
	at  time.Time
}

// modelsTTL é quanto tempo vale a lista de modelos que o Groq diz ter.
const modelsTTL = time.Hour

// available devolve os modelos a tentar no provedor, na ordem: os preferidos
// que ele ainda tem e, depois, os outros de conversa dele (cada modelo tem a
// sua cota grátis). Se a lista falhar, usa os preferidos como estão.
func (h *handler) available(p Provider) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.models[p.Name]; ok && time.Since(c.at) < modelsTTL {
		return c.ids
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // a trava segura as outras perguntas
	defer cancel()
	ids, err := llm.ListModels(ctx, p.Upstream, p.APIKey)
	picked := llm.PickModels(ids, p.Models)
	if p.OnlyFree {
		picked = slices.DeleteFunc(picked, func(id string) bool { return !free(id) })
	}
	if err != nil || len(picked) == 0 {
		// Tenta a lista de novo em 5 minutos, sem uma chamada a mais em cada pergunta.
		h.models[p.Name] = cachedModels{p.Models, time.Now().Add(5*time.Minute - modelsTTL)}
		return p.Models
	}
	picked = picked[:min(len(picked), maxModels)]
	h.models[p.Name] = cachedModels{picked, time.Now()}
	return picked
}

// free: no OpenRouter, os modelos grátis terminam em ":free"; openrouter/free
// escolhe um deles sozinho.
func free(id string) bool { return strings.HasSuffix(id, ":free") || id == "openrouter/free" }

func (h *handler) chat(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		fail(w, http.StatusRequestEntityTooLarge, "pedido grande demais")
		return
	}
	var body map[string]json.RawMessage
	var msgs []chatMessage
	if json.Unmarshal(raw, &body) != nil || json.Unmarshal(body["messages"], &msgs) != nil || !fromApp(body, msgs) {
		fail(w, http.StatusForbidden, "este endereço só atende o consultor do app finan_rich")
		return
	}
	device, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(device) > 64 {
		device = device[:64]
	}
	if msg := h.limiter.allow(device, clientIP(r), h.cfg.Limits); msg != "" {
		fail(w, http.StatusTooManyRequests, msg)
		return
	}
	body["max_tokens"] = json.RawMessage(capTokens(body["max_tokens"]))
	delete(body, "max_completion_tokens")
	var stream bool
	json.Unmarshal(body["stream"], &stream)
	rec := usageRecord{Persona: personaOf(msgs), Device: hashDevice(device)}
	for pi, p := range h.cfg.Providers {
		models := h.available(p)
		for i, model := range models {
			last := pi == len(h.cfg.Providers)-1 && i == len(models)-1
			body["model"], _ = json.Marshal(model)
			delete(body, "reasoning_effort")
			if strings.Contains(model, "gpt-oss") {
				body["reasoning_effort"] = json.RawMessage(`"low"`) // menos tokens de raciocínio: sobra cota grátis
			}
			delete(body, "stream_options")
			if stream && p.StreamUsage {
				body["stream_options"] = json.RawMessage(`{"include_usage":true}`) // tokens no último evento
			}
			payload, _ := json.Marshal(body)
			req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(p.Upstream, "/")+"/chat/completions", bytes.NewReader(payload))
			if err != nil {
				fail(w, http.StatusInternalServerError, err.Error())
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+p.APIKey)
			rec.Provider, rec.Model, rec.start = p.Name, model, time.Now()
			resp, err := h.cfg.Client.Do(req)
			if err != nil {
				rec.log(0, -1, -1)
				if last || r.Context().Err() != nil {
					fail(w, http.StatusBadGateway, "o provedor de IA não respondeu")
					return
				}
				break // provedor fora do ar: o próximo
			}
			if !last && (retryable(resp.StatusCode) || badKey(resp.StatusCode)) {
				resp.Body.Close()
				rec.log(resp.StatusCode, -1, -1)
				if badKey(resp.StatusCode) {
					break // chave errada ou sem permissão: os outros modelos dele também falham
				}
				// Cota grátis deste modelo acabou, ele está fora ou foi desligado
				// (400/404 de modelo aposentado): tenta o próximo.
				continue
			}
			in, out := relay(w, resp)
			rec.log(resp.StatusCode, in, out)
			return
		}
	}
	fail(w, http.StatusBadGateway, "nenhum provedor de IA respondeu")
}

func retryable(status int) bool {
	return status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusTooManyRequests || status >= 500
}

func badKey(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

// fromApp aceita só o que o app manda: o consultor e a categorização de
// extrato (pelas frases fixas das instruções) ou o teste de conexão. Quem copiar o prompt passa, mas cai nos
// limites; a ideia é não virar uma IA grátis genérica.
func fromApp(body map[string]json.RawMessage, msgs []chatMessage) bool {
	if len(msgs) == 0 {
		return false
	}
	if msgs[0].Role == "system" && (strings.HasPrefix(msgs[0].Content, advisor.PromptMarker) || strings.HasPrefix(msgs[0].Content, advisor.CategorizeMarker)) {
		return true
	}
	return len(msgs) == 1 && msgs[0].Role == "user" && msgs[0].Content == llm.PingMessage && body["tools"] == nil
}

func capTokens(raw json.RawMessage) string {
	var n int
	if json.Unmarshal(raw, &n) == nil && n > 0 && n < maxTokens {
		return string(raw)
	}
	b, _ := json.Marshal(maxTokens)
	return string(b)
}

// relay copia a resposta (streaming SSE) pedaço por pedaço e devolve os
// tokens de entrada e de saída do "usage" (-1 se o provedor não mandou).
func relay(w http.ResponseWriter, resp *http.Response) (in, out int) {
	defer resp.Body.Close()
	var tail tailBuffer
	defer func() { in, out = tail.usage() }()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			tail.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

// clientIP: o Google (Cloud Run) acrescenta o IP de quem chamou no FIM do X-Forwarded-For
// (o começo é o que o cliente mandou, e pode ser inventado para burlar o
// limite). Lê da direita para a esquerda pulando os IPs do próprio Google e
// os internos; sem nenhum IP público, vale o endereço da conexão. Na Vercel o
// X-Forwarded-For traz só o IP de quem chamou (ela descarta o que o cliente manda).
func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		parts := strings.Split(f, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := net.ParseIP(strings.TrimSpace(parts[i]))
			if ip != nil && !googleFrontEnd(ip) && !ip.IsPrivate() && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Faixas do front-end e dos balanceadores do Google (documentadas pelo Google Cloud).
var googleRanges = []*net.IPNet{mustCIDR("35.191.0.0/16"), mustCIDR("130.211.0.0/22")}

func googleFrontEnd(ip net.IP) bool {
	for _, n := range googleRanges {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": msg}})
}

// cors libera o navegador (PWA); o app do celular usa HTTP nativo.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		h.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limiter conta chamadas em janelas fixas de uma hora e de um dia. Fica na
// memória: com o Cloud Run em no máximo 1 instância, vale para todos; se a
// instância dormir, a contagem recomeça (aceitável: a cota do Groq é o teto).
// maxKeys limita a memória do limitador (cada aparelho e cada IP é uma chave).
const maxKeys = 100000

// maxDevicesPerIP: aparelhos novos por rede no dia; passou disso, os novos
// dividem o balde da rede (senão alguém lota o limitador inventando aparelhos).
const maxDevicesPerIP = 5

type limiter struct {
	mu     sync.Mutex
	now    func() time.Time
	counts map[string]*count
}

type count struct {
	hour, day     int64 // início da janela (Unix)
	inHour, inDay int
	devices       int // aparelhos novos vistos desta rede hoje (só nas chaves "ip:")
}

func newLimiter(now func() time.Time) *limiter {
	return &limiter{now: now, counts: map[string]*count{}}
}

// allow devolve "" se pode, ou a mensagem para a pessoa.
func (l *limiter) allow(device, ip string, lim Limits) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	hour, day := now.Truncate(time.Hour).Unix(), now.UTC().Truncate(24*time.Hour).Unix()
	if len(l.counts) >= maxKeys { // tira só quem é de outro dia (zerar tudo soltaria quem está barrado)
		for k, c := range l.counts {
			if c.day != day {
				delete(l.counts, k)
			}
		}
	}
	get := func(key string) *count {
		c := l.counts[key]
		if c == nil {
			if len(l.counts) >= maxKeys {
				return nil // memória cheia hoje: não aceita gente nova até amanhã
			}
			c = &count{}
			l.counts[key] = c
		}
		if c.hour != hour {
			c.hour, c.inHour = hour, 0
		}
		if c.day != day {
			c.day, c.inDay, c.devices = day, 0, 0
		}
		return c
	}
	if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() == nil {
		ip = parsed.Mask(net.CIDRMask(64, 128)).String() + "/64" // IPv6: a casa tem o /64 inteiro
	}
	ipc := get("ip:" + ip)
	if ipc == nil {
		return "o consultor grátis está com movimento demais agora; tente mais tarde ou use sua própria chave grátis em Ajustes"
	}
	if _, seen := l.counts["dev:"+device]; device != "" && !seen {
		if ipc.devices >= maxDevicesPerIP {
			device = ""
		} else {
			ipc.devices++
		}
	}
	if device == "" { // sem identificação (ex.: computador): conta como um aparelho daquela rede
		device = "anonimo:" + ip
	}
	dev := get("dev:" + device)
	if dev == nil {
		return "o consultor grátis está com movimento demais agora; tente mais tarde ou use sua própria chave grátis em Ajustes"
	}
	if ipc.inHour >= lim.IPHour || ipc.inDay >= lim.IPDay {
		return "muitas perguntas desta rede ao consultor grátis; tente mais tarde ou use sua própria chave grátis em Ajustes"
	}
	if dev.inHour >= lim.DeviceHour || dev.inDay >= lim.DeviceDay {
		return "o consultor grátis chegou ao limite deste aparelho por hoje; tente amanhã ou use sua própria chave grátis em Ajustes"
	}
	dev.inHour++
	dev.inDay++
	ipc.inHour++
	ipc.inDay++
	return ""
}
