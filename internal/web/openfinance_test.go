package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOpenFinance(t *testing.T) {
	pluggy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			w.Write([]byte(`{"apiKey":"k"}`))
		case "/accounts":
			w.Write([]byte(`{"results":[{"id":"a1","type":"BANK","name":"Conta","number":"1234"}]}`))
		case "/v2/transactions":
			w.Write([]byte(`{"results":[{"id":"t1","description":"Mercado","amount":-10,"date":"2026-10-06T00:00:00Z","type":"DEBIT","status":"POSTED"}],"next":null}`))
		}
	}))
	defer pluggy.Close()
	t.Setenv("FINAN_PLUGGY_URL", pluggy.URL)
	a := newApp(t, fakeOllama(t).URL)
	ask := func() (int, string) {
		resp, err := http.Post(a.srv.URL+"/api/openfinance", "application/json", strings.NewReader(`{"from":"2026-09-01"}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, body := ask(); code != http.StatusBadRequest || !strings.Contains(body, "Ajustes") {
		t.Errorf("sem credenciais = %d %s", code, body)
	}

	a.post("/configuracoes/openfinance", url.Values{"client_id": {"meu-id"}, "client_secret": {"segredo-123"}, "items": {"item-1\nitem-2"}})
	if _, page := a.get("/configuracoes"); strings.Contains(page, "segredo-123") || !strings.Contains(page, "meu-id") {
		t.Error("Ajustes não pode mostrar o segredo (mas mostra o Client ID)")
	}
	// Salvar de novo com o segredo em branco mantém o que estava.
	a.post("/configuracoes/openfinance", url.Values{"client_id": {"meu-id"}, "client_secret": {""}, "items": {"item-1"}})
	if s, _ := a.st.Settings(t.Context(), "pluggy_"); s["client_secret"] != "segredo-123" || s["items"] != "item-1" {
		t.Errorf("credenciais salvas = %v", s)
	}

	code, body := ask()
	var accs []struct {
		Name      string
		Statement struct {
			Account string
			Rows    []struct {
				Description string
				AmountCents int64
			}
		}
	}
	if code != http.StatusOK || json.Unmarshal([]byte(body), &accs) != nil || len(accs) != 1 || accs[0].Statement.Rows[0].AmountCents != -1000 {
		t.Errorf("busca = %d %s", code, body)
	}
}
