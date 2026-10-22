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
