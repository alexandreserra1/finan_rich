package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakePluggy liga uma Pluggy falsa com uma conta e um lançamento e devolve
// quantas buscas chegaram a ela.
func fakePluggy(t *testing.T) *atomic.Int32 {
	var auths atomic.Int32
	pluggy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			auths.Add(1)
			w.Write([]byte(`{"apiKey":"k"}`))
		case "/accounts":
			w.Write([]byte(`{"results":[{"id":"a1","type":"BANK","name":"Conta","number":"1234"}]}`))
		case "/v2/transactions":
			w.Write([]byte(`{"results":[{"id":"t1","description":"Mercado","amount":-10,"date":"2026-10-06T00:00:00Z","type":"DEBIT","status":"POSTED"}],"next":null}`))
		}
	}))
	t.Cleanup(pluggy.Close)
	t.Setenv("FINAN_PLUGGY_URL", pluggy.URL)
	return &auths
}

func TestOpenFinance(t *testing.T) {
	fakePluggy(t)
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

// O computador busca sozinho e guarda o resultado: a busca automática dos
// aparelhos ("auto") usa o guardado por até 6 horas; "Buscar do banco" vai
// sempre à Pluggy; credenciais trocadas não usam o guardado.
func TestOpenFinanceBackground(t *testing.T) {
	auths := fakePluggy(t)
	a := newApp(t, fakeOllama(t).URL)
	ask := func(auto bool) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"from": bankFrom(time.Now()), "auto": auto})
		resp, err := http.Post(a.srv.URL+"/api/openfinance", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("busca = %d", resp.StatusCode)
		}
	}
	a.s.refreshBank(t.Context())
	if n := auths.Load(); n != 0 {
		t.Fatalf("sem credenciais buscou %d vezes", n)
	}
	a.post("/configuracoes/openfinance", url.Values{"client_id": {"meu-id"}, "client_secret": {"segredo"}, "items": {"item-1"}})
	a.s.refreshBank(t.Context())
	a.s.refreshBank(t.Context())
	ask(true)
	if n := auths.Load(); n != 1 {
		t.Errorf("segundo plano duas vezes + busca automática da tela = %d buscas na Pluggy, quero 1", n)
	}
	ask(false)
	if n := auths.Load(); n != 2 {
		t.Errorf("Buscar do banco precisa ir à Pluggy (%d buscas)", n)
	}
	a.post("/configuracoes/openfinance", url.Values{"client_id": {"meu-id"}, "client_secret": {""}, "items": {"item-1 item-2"}})
	ask(true)
	if n := auths.Load(); n != 3 {
		t.Errorf("com outro banco conectado, o guardado não serve (%d buscas)", n)
	}
	a.s.bank.at = a.s.bank.at.Add(-bankEvery)
	a.s.refreshBank(t.Context())
	if n := auths.Load(); n != 4 {
		t.Errorf("depois de 6 horas o segundo plano busca de novo (%d buscas)", n)
	}
}

func TestBankFrom(t *testing.T) {
	for now, want := range map[time.Time]string{
		time.Date(2026, 11, 7, 23, 0, 0, 0, time.Local): "2026-10-01",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local):   "2025-12-01",
		time.Date(2026, 3, 31, 12, 0, 0, 0, time.Local): "2026-02-01",
	} {
		if got := bankFrom(now); got != want {
			t.Errorf("bankFrom(%s) = %s, quero %s", now, got, want)
		}
	}
}
