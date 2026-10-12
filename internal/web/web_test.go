package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// fakeOllama imita o Ollama com o modelo qwen3:4b instalado.
func fakeOllama(t *testing.T) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"qwen3:4b"}]}`)
		case "/api/chat":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "dados_financeiros") {
				t.Errorf("pergunta sem retrato financeiro: %s", body)
			}
			fmt.Fprintln(w, `{"message":{"content":"Pague o cartão."},"done":false}`)
			fmt.Fprintln(w, `{"message":{"content":""},"done":true}`)
		case "/api/pull":
			fmt.Fprintln(w, `{"status":"success"}`)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

type app struct {
	t   *testing.T
	srv *httptest.Server
	st  *store.Store
}

func newApp(t *testing.T, ollamaURL string) *app {
	t.Helper()
	st, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s, err := New(st, Options{AIPreset: "ollama", AIDefaults: llm.Config{Kind: llm.KindOllama, BaseURL: ollamaURL, Model: "qwen3:4b"}, Port: "8080"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Routes())
	t.Cleanup(srv.Close)
	return &app{t: t, srv: srv, st: st}
}

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func (a *app) get(path string) (int, string) {
	a.t.Helper()
	resp, err := http.Get(a.srv.URL + path)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// post envia um formulário e devolve o status e o Location (ou o corpo).
func (a *app) post(path string, form url.Values, header ...string) (int, string) {
	a.t.Helper()
	req, _ := http.NewRequest(http.MethodPost, a.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	if loc := resp.Header.Get("Location"); loc != "" {
		return resp.StatusCode, loc
	}
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// TestPagesRender abre todas as páginas com o banco vazio e depois com dados,
// para pegar erros de template que só aparecem num dos dois casos.
func TestPagesRender(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	paths := []string{"/", "/lancamentos", "/categorias", "/dividas", "/chat", "/configuracoes",
		"/static/app.css", "/static/app.js", "/static/favicon.svg", "/manifest.webmanifest", "/sw.js", "/offline"}
	check := func(state string) {
		for _, path := range paths {
			if code, body := a.get(path); code != http.StatusOK {
				t.Errorf("[%s] GET %s = %d: %.300s", state, path, code, body)
			}
		}
	}
	check("vazio")
	seed(t, a.st)
	a.st.AppendChatTurn(context.Background(), "oi", "**olá**")
	check("com dados")

	// O casco das telas de dados diz qual tela o JavaScript deve montar.
	if _, body := a.get("/dividas"); !strings.Contains(body, `data-page="debts"`) || !strings.Contains(body, "/static/js/main.js") {
		t.Errorf("casco da tela de dívidas: %.400s", body)
	}
}

// seed grava um lançamento e uma dívida pelo mesmo caminho da sincronização.
func seed(t *testing.T, st *store.Store) {
	t.Helper()
	rec := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	_, err := st.ApplyChanges(context.Background(), []store.Change{
		{Type: store.TypeTransaction, Record: rec(domain.Transaction{Meta: domain.Meta{ID: "t1", UpdatedAt: 1}, Kind: domain.KindSaida,
			Description: "Aluguel", AmountCents: 180000, CategoryID: "cat-3", Date: time.Now().Format(domain.DateLayout)})},
		{Type: store.TypeDebt, Record: rec(domain.Debt{Meta: domain.Meta{ID: "d1", UpdatedAt: 1}, Name: "Cartão",
			BalanceCents: 320000, MonthlyRateBP: 1400, MinPaymentCents: 32000})},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (a *app) postJSON(path string, body any) (int, map[string]any) {
	a.t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, a.srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestSyncAPI(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)

	// Aparelho novo: baixa tudo (categorias padrão e divisão do salário).
	code, first := a.postJSON("/api/sync", map[string]any{"cursor": 0})
	if code != http.StatusOK || len(first["changes"].([]any)) != 16 || first["more"] != false {
		t.Fatalf("primeira sincronização: %d %v", code, first)
	}
	cursor := first["cursor"].(float64)

	// Envia um lançamento feito offline e recebe de volta só o que mudou.
	tx := domain.Transaction{Meta: domain.Meta{ID: "uuid-1", UpdatedAt: 1000}, Kind: domain.KindSaida,
		Description: "Mercado", AmountCents: 9000, CategoryID: "cat-5", Date: "2026-10-12"}
	code, second := a.postJSON("/api/sync", map[string]any{"cursor": cursor, "changes": []any{map[string]any{"type": "transaction", "record": tx}}})
	changes := second["changes"].([]any)
	if code != http.StatusOK || len(changes) != 1 || second["cursor"].(float64) <= cursor {
		t.Fatalf("envio: %d %v", code, second)
	}
	l, _ := a.st.Ledger(context.Background())
	if txs := l.ActiveTransactions(); len(txs) != 1 || txs[0].Description != "Mercado" {
		t.Errorf("servidor não gravou: %+v", txs)
	}

	// Registro inválido é recusado com 422 e a mensagem do domínio.
	tx.Description = ""
	code, bad := a.postJSON("/api/sync", map[string]any{"cursor": cursor, "changes": []any{map[string]any{"type": "transaction", "record": tx}}})
	if code != http.StatusUnprocessableEntity || !strings.Contains(bad["error"].(string), "descrição") {
		t.Errorf("inválido: %d %v", code, bad)
	}
	if code, _ := a.postJSON("/api/sync", "lixo"); code != http.StatusBadRequest {
		t.Errorf("corpo inválido = %d", code)
	}
}

func TestBlocksCrossOriginPost(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	if code, _ := a.post("/chat/limpar", nil, "Origin", "http://evil.example"); code != http.StatusForbidden {
		t.Errorf("Origin estranho = %d", code)
	}
	if code, _ := a.post("/chat/limpar", nil, "Sec-Fetch-Site", "cross-site"); code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site cross-site = %d", code)
	}
	if code, _ := a.post("/chat/limpar", nil, "Origin", a.srv.URL); code != http.StatusSeeOther {
		t.Errorf("mesma origem = %d", code)
	}
}

func TestChatStreamsAndSaves(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	code, body := a.post("/chat/enviar", url.Values{"message": {"Qual dívida pago primeiro?"}})
	if code != http.StatusOK || !strings.Contains(body, `"text":"Pague o cartão."`) || !strings.Contains(body, `"done":true`) {
		t.Fatalf("SSE = %d %s", code, body)
	}
	msgs, _ := a.st.ListChat(context.Background())
	if len(msgs) != 2 || msgs[0].Content != "Qual dívida pago primeiro?" || msgs[1].Content != "Pague o cartão." {
		t.Errorf("histórico = %+v", msgs)
	}
}

func TestChatWhenOllamaOffline(t *testing.T) {
	off := fakeOllama(t)
	off.Close()
	a := newApp(t, off.URL)
	if _, body := a.get("/chat"); !strings.Contains(body, "O Ollama não está rodando") {
		t.Error("página do chat deveria explicar como instalar o Ollama")
	}
	_, body := a.post("/chat/enviar", url.Values{"message": {"oi"}})
	if !strings.Contains(body, `"error"`) || !strings.Contains(body, "ollama.com") {
		t.Errorf("SSE = %s", body)
	}
	if msgs, _ := a.st.ListChat(context.Background()); len(msgs) != 0 {
		t.Error("pergunta com erro não deve ser salva")
	}
}

func TestSettings(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	ctx := context.Background()

	// Provedor na nuvem sem chave: recusado.
	if _, loc := a.post("/configuracoes", url.Values{"preset": {"anthropic"}, "model": {"claude-opus-5-5"}}); !strings.Contains(loc, "erro=") {
		t.Errorf("sem chave deveria dar erro: %s", loc)
	}
	form := url.Values{"preset": {"groq"}, "model": {"qwen/qwen3-32b"}, "base_url": {"https://api.groq.com/openai/v1"}, "api_key": {"segredo"}}
	if _, loc := a.post("/configuracoes", form); !strings.Contains(loc, "msg=") {
		t.Fatalf("salvar: %s", loc)
	}
	// Salvar de novo com a chave em branco mantém a chave.
	form.Set("api_key", "")
	a.post("/configuracoes", form)
	saved, _ := a.st.Settings(ctx, aiPrefix)
	if saved["preset"] != "groq" || saved["api_key"] != "segredo" {
		t.Errorf("configuração salva = %v", saved)
	}
	if _, body := a.get("/configuracoes"); strings.Contains(body, "segredo") {
		t.Error("a chave não pode aparecer no HTML")
	}
}

func TestPullModel(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	if _, body := a.post("/configuracoes/baixar-modelo", nil); !strings.Contains(body, `"status":"success"`) || !strings.Contains(body, `"done":true`) {
		t.Errorf("pull = %s", body)
	}
}
