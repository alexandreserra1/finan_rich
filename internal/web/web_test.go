package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
	paths := []string{"/", "/?mes=2026-09", "/lancamentos?mes=2026-09", "/categorias", "/dividas", "/dividas?editar=1",
		"/chat", "/configuracoes", "/static/app.css", "/static/app.js", "/static/favicon.svg"}
	check := func(state string) {
		for _, path := range paths {
			if path == "/dividas?editar=1" && state == "vazio" {
				continue
			}
			if code, body := a.get(path); code != http.StatusOK {
				t.Errorf("[%s] GET %s = %d: %.300s", state, path, code, body)
			}
		}
	}
	check("vazio")

	ctx := context.Background()
	cats, _ := a.st.ListCategories(ctx)
	day := 10
	for _, tx := range []store.Transaction{
		{Kind: store.KindEntrada, Description: "Salário", AmountCents: 500000, CategoryID: cats[0].ID, OccurredOn: store.NewMonth(2026, 9).Day(5)},
		{Kind: store.KindSaida, Description: "Aluguel", AmountCents: 180000, CategoryID: cats[2].ID, OccurredOn: store.NewMonth(2026, 9).Day(10)},
	} {
		if err := a.st.CreateTransaction(ctx, tx); err != nil {
			t.Fatal(err)
		}
	}
	a.st.SaveDebt(ctx, store.Debt{Name: "Cartão", BalanceCents: 400000, MonthlyRateBP: 1250, MinPaymentCents: 40000, DueDay: &day})
	a.st.SaveDebt(ctx, store.Debt{Name: "Empréstimo", BalanceCents: 200000, MonthlyRateBP: 210, MinPaymentCents: 30000})
	a.st.AppendChatTurn(ctx, "oi", "**olá**")
	check("com dados")
}

func TestTransactionFlow(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	cats, _ := a.st.ListCategories(context.Background())
	form := url.Values{"kind": {"saida"}, "description": {"Aluguel"}, "amount": {"1.800,00"},
		"category_id": {fmt.Sprint(cats[2].ID)}, "occurred_on": {"2026-09-10"}}
	code, loc := a.post("/lancamentos", form)
	if code != http.StatusSeeOther || !strings.Contains(loc, "mes=2026-09") || !strings.Contains(loc, "msg=") {
		t.Fatalf("POST = %d %s", code, loc)
	}
	if _, body := a.get("/lancamentos?mes=2026-09"); !strings.Contains(body, "Aluguel") || !strings.Contains(body, "R$ 1.800,00") {
		t.Error("lançamento não aparece na lista")
	}
	// Com dados, o painel monta o resumo do mês e a tabela anual.
	if code, body := a.get("/?mes=2026-09"); code != http.StatusOK || !strings.Contains(body, "setembro de 2026") || !strings.Contains(body, "-R$ 1.800,00") {
		t.Errorf("painel com dados = %d: %.300s", code, body)
	}

	form.Set("amount", "abc")
	if _, loc := a.post("/lancamentos", form); !strings.Contains(loc, "erro=") {
		t.Errorf("valor inválido deveria dar erro: %s", loc)
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
