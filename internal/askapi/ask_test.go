package askapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/proxy"
)

// IA na nuvem falsa (formato OpenAI): pede a busca, depois responde usando o
// apelido que recebeu. Confere que nenhum nome real saiu do aparelho.
func TestAskAnonymizedCloud(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		delete(body, "tools") // as descrições das ferramentas têm exemplos fixos, não dados da pessoa
		clean, _ := json.Marshal(body)
		bodies = append(bodies, string(clean))
		if r.Header.Get("Authorization") != "Bearer chave-secreta" {
			t.Errorf("chave = %q", r.Header.Get("Authorization"))
		}
		var delta string
		switch strings.Count(string(raw), `"role":"tool"`) {
		case 0:
			delta = `{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"buscar_lancamentos","arguments":"{\"texto\":\"Loja AB\"}"}}]}`
		case 1:
			delta = `{"tool_calls":[{"index":0,"id":"c2","type":"function","function":{"name":"alterar_lancamento","arguments":"{\"id\":\"b\",\"valor\":\"30\"}"}}]}`
		default:
			delta = `{"content":"Você gastou R$ 25,00 na Loja AB. Deixei a correção pronta."}`
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":%s}]}\n\ndata: [DONE]\n\n", delta)
	}))
	defer srv.Close()

	req := map[string]any{
		"ledger": json.RawMessage(`{
		  "categories": [{"id":"sal","updatedAt":1,"name":"Salário","bucket":"renda"},{"id":"tra","updatedAt":1,"name":"Transporte","bucket":"necessidades"}],
		  "transactions": [
		    {"id":"a","updatedAt":1,"kind":"entrada","description":"Salário ACME Ltda","amountCents":500000,"categoryId":"sal","date":"2026-10-05"},
		    {"id":"b","updatedAt":1,"kind":"saida","description":"Uber *Trip","amountCents":2500,"categoryId":"tra","date":"2026-10-06"}],
		  "debts": [{"id":"d","updatedAt":1,"name":"Cartão Nubank","creditor":"Nu","balanceCents":320000,"monthlyRateBp":1400,"minPaymentCents":32000}],
		  "notes": [{"id":"n","updatedAt":1,"title":"segredo","body":"PIN 1234"}]}`),
		"history":   []ChatTurn{{"user", "e o uber?"}, {"assistant", "O Uber custou pouco."}},
		"question":  "Quanto gastei com Uber? O valor certo era 30",
		"today":     "2026-10-20",
		"ai":        AIConfig{Preset: "groq", APIKey: "chave-secreta", BaseURL: srv.URL},
		"anonymize": true,
	}
	raw, _ := json.Marshal(req)
	var out struct {
		OK    AskReply
		Error string
	}
	json.Unmarshal([]byte(Ask(context.Background(), string(raw))), &out)
	if out.Error != "" {
		t.Fatal(out.Error)
	}
	for _, b := range bodies {
		for _, secret := range []string{"Uber", "uber", "ACME", "Nubank", "segredo", "PIN 1234"} {
			if i := strings.Index(b, secret); i >= 0 {
				t.Errorf("%q saiu do aparelho: …%s…", secret, b[max(0, i-120):min(len(b), i+60)])
			}
		}
	}
	if out.OK.Answer != "Você gastou R$ 25,00 na Uber. Deixei a correção pronta." {
		t.Errorf("resposta = %q", out.OK.Answer)
	}
	if len(out.OK.Proposals) != 1 || !strings.Contains(string(out.OK.Proposals[0].Record), `"description":"Uber *Trip"`) ||
		!strings.Contains(string(out.OK.Proposals[0].Record), `"amountCents":3000`) || strings.Contains(out.OK.Proposals[0].Label, "Loja") {
		t.Errorf("proposta = %+v", out.OK.Proposals)
	}
}

// IA dentro do celular: as mensagens saem prontas daqui (formato OpenAI),
// com as instruções do consultor, os nomes verdadeiros e sem ferramentas.
func TestLocalPrompt(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"ledger":   json.RawMessage(`{"debts":[{"id":"d","updatedAt":1,"name":"Cartão Nubank","balanceCents":320000,"monthlyRateBp":1400,"minPaymentCents":32000}]}`),
		"history":  []ChatTurn{{"user", "p1"}, {"assistant", "r1"}, {"user", "p2"}, {"assistant", "r2"}},
		"question": "qual dívida pago primeiro?",
		"today":    "2026-10-20",
	})
	var out struct {
		OK    []ChatTurn
		Error string
	}
	if err := json.Unmarshal([]byte(LocalPrompt(string(raw))), &out); err != nil || out.Error != "" {
		t.Fatal(err, out.Error)
	}
	roles := ""
	for _, m := range out.OK {
		roles += m.Role + " "
	}
	if roles != "system user assistant user " || out.OK[1].Content != "p2" {
		t.Fatalf("mensagens = %+v", out.OK)
	}
	if !strings.HasPrefix(out.OK[0].Content, advisor.PromptMarker) || strings.Contains(out.OK[0].Content, "ferramenta") {
		t.Errorf("system = %q", out.OK[0].Content[:80])
	}
	if last := out.OK[3].Content; !strings.Contains(last, "Cartão Nubank") || !strings.HasSuffix(last, "qual dívida pago primeiro?") {
		t.Errorf("última = %q", last)
	}
}

func TestAskNeedsCloudPresetAndKey(t *testing.T) {
	for ai, want := range map[string]string{
		`{"preset":"ollama"}`:                 "escolha um provedor",
		`{"preset":"anthropic","apiKey":"x"}`: "escolha um provedor",
		`{"preset":"gemini"}`:                 "falta a chave",
	} {
		got := Ask(context.Background(), `{"ledger":{},"question":"oi","ai":`+ai+`}`)
		if !strings.Contains(got, want) {
			t.Errorf("%s = %s", ai, got)
		}
	}
}

func TestCheckKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer boa" {
			w.WriteHeader(http.StatusBadRequest) // como o Gemini responde a uma chave errada
			fmt.Fprint(w, `[{"error":{"code":400,"message":"API key not valid. Please pass a valid API key."}}]`)
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	check := func(key string) string {
		raw, _ := json.Marshal(AIConfig{Preset: "gemini", APIKey: key, BaseURL: srv.URL})
		return Check(context.Background(), string(raw))
	}
	if got := check("boa"); got != `{"ok":true}` {
		t.Errorf("chave boa: %s", got)
	}
	if got := check("ruim"); !strings.Contains(got, "a chave da IA foi recusada") || !strings.Contains(got, "API key not valid") {
		t.Errorf("chave ruim: %s", got)
	}
}

// Grátis do finan_rich: sem chave; vai a identificação do aparelho (para o limite).
func TestFreePreset(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	old := llm.FreeProxyURL
	t.Cleanup(func() { llm.FreeProxyURL = old })
	llm.FreeProxyURL = ""
	if strings.Contains(CloudPresets(), `"finan"`) {
		t.Error("sem endereço, a opção grátis não aparece")
	}
	llm.FreeProxyURL = srv.URL
	if !strings.Contains(CloudPresets(), `"id":"finan"`) || !strings.Contains(CloudPresets(), `"free":true`) {
		t.Errorf("presets = %s", CloudPresets())
	}
	raw, _ := json.Marshal(AIConfig{Preset: "finan", APIKey: "aparelho-123"})
	if got := Check(context.Background(), string(raw)); got != `{"ok":true}` || auth != "Bearer aparelho-123" {
		t.Errorf("check = %s auth = %q", got, auth)
	}
}

// "IA sugere" sem computador: vai só a assinatura da descrição (sem CPF).
func TestCategorizeCloud(t *testing.T) {
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sent = string(b)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"1\\\": \\\"Restaurantes e delivery\\\"}\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	raw, _ := json.Marshal(map[string]any{
		"categories": []map[string]any{{"id": "res", "updatedAt": 1, "name": "Restaurantes e delivery", "bucket": "desejos"}},
		"items":      []map[string]string{{"description": "PIX ENVIADO 123.456.789-00 IFOOD", "kind": "saida"}},
		"ai":         AIConfig{Preset: "groq", APIKey: "k", BaseURL: srv.URL},
	})
	got := Categorize(context.Background(), string(raw))
	if got != `{"ok":["res"]}` || strings.Contains(sent, "123.456") || strings.Contains(sent, "PIX ENVIADO") {
		t.Errorf("resposta %s; enviado %s", got, sent)
	}
}

// Consultor grátis de ponta a ponta: o pedido verdadeiro do app (com a busca
// por ferramenta), a categorização e o teste de conexão passam pelo
// intermediário real (internal/proxy) até um Groq falso. Se o prompt do app
// mudar e o filtro do intermediário deixar de reconhecê-lo, este teste falha.
func TestFreePresetThroughProxy(t *testing.T) {
	var models []string
	groq := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			http.NotFound(w, r) // sem a lista, o intermediário usa a configuração
			return
		}
		if r.Header.Get("Authorization") != "Bearer chave-do-servidor" {
			t.Errorf("chave = %q", r.Header.Get("Authorization"))
		}
		raw, _ := io.ReadAll(r.Body)
		var body struct{ Model string }
		json.Unmarshal(raw, &body)
		models = append(models, body.Model)
		delta := `{"content":"Você gastou R$ 25,00."}`
		switch {
		case strings.Contains(string(raw), advisor.CategorizeMarker):
			delta = `{"content":"{\"1\": \"Transporte\"}"}`
		case !strings.Contains(string(raw), `"role":"tool"`) && strings.Contains(string(raw), advisor.PromptMarker):
			delta = `{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"buscar_lancamentos","arguments":"{\"texto\":\"Loja AB\"}"}}]}`
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":%s}]}\n\ndata: [DONE]\n\n", delta)
	}))
	defer groq.Close()
	free := httptest.NewServer(proxy.New(proxy.Config{Upstream: groq.URL, APIKey: "chave-do-servidor", Models: []string{"modelo-do-servidor"}, Limits: proxy.DefaultLimits}))
	defer free.Close()
	old := llm.FreeProxyURL
	t.Cleanup(func() { llm.FreeProxyURL = old })
	llm.FreeProxyURL = free.URL
	ai := AIConfig{Preset: llm.FreeID, APIKey: "aparelho-123"}

	aiRaw, _ := json.Marshal(ai)
	if got := Check(context.Background(), string(aiRaw)); got != `{"ok":true}` {
		t.Errorf("teste de conexão = %s", got)
	}
	cats := json.RawMessage(`[{"id":"tra","updatedAt":1,"name":"Transporte","bucket":"necessidades"}]`)
	raw, _ := json.Marshal(map[string]any{
		"ledger": json.RawMessage(`{"categories":` + string(cats) + `,
		  "transactions":[{"id":"b","updatedAt":1,"kind":"saida","description":"Uber *Trip","amountCents":2500,"categoryId":"tra","date":"2026-10-06"}]}`),
		"question": "Quanto gastei com Uber?", "today": "2026-10-20", "ai": ai, "anonymize": true,
	})
	var out struct {
		OK    AskReply
		Error string
	}
	json.Unmarshal([]byte(Ask(context.Background(), string(raw))), &out)
	if out.Error != "" || out.OK.Answer != "Você gastou R$ 25,00." {
		t.Errorf("consultor pelo intermediário: resposta %q, erro %q", out.OK.Answer, out.Error)
	}
	raw, _ = json.Marshal(map[string]any{"categories": cats, "items": []map[string]string{{"description": "UBER TRIP", "kind": "saida"}}, "ai": ai})
	if got := Categorize(context.Background(), string(raw)); got != `{"ok":["tra"]}` {
		t.Errorf("categorização pelo intermediário = %s", got)
	}
	for _, m := range models {
		if m != "modelo-do-servidor" {
			t.Errorf("o modelo é escolhido no intermediário, chegou %q", m)
		}
	}
	if len(models) < 4 {
		t.Errorf("chamadas ao Groq = %d (teste, busca, resposta, categorização)", len(models))
	}
}
