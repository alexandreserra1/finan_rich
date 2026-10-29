package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
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
	paths := []string{"/", "/lancamentos", "/categorias", "/dividas", "/notas", "/exportar", "/chat", "/configuracoes",
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
	a.st.AppendChatTurn(context.Background(), "oi", "**olá**", "", nil)
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
	// A hora do computador vai junto: o aparelho acerta o carimbo das edições por ela.
	if now, _ := first["now"].(float64); math.Abs(now-float64(time.Now().UnixMilli())) > 5000 {
		t.Errorf("hora do computador na sincronização = %v", first["now"])
	}

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
	if len(msgs) != 2 || msgs[0].Content != "Qual dívida pago primeiro?" || msgs[1].Content != "Pague o cartão." || msgs[1].Agent != "jade" {
		t.Errorf("histórico = %+v", msgs)
	}
	if !strings.Contains(body, `"suggest":{"id":"bento"`) {
		t.Errorf("assunto de dívida com a Jade: deveria sugerir o Bento: %s", body)
	}

	// Com a Luna escolhida, a resposta fica dela; personagem desconhecido vira Jade.
	_, body = a.post("/chat/enviar", url.Values{"message": {"Qual dívida pago primeiro?"}, "agent": {"luna"}})
	a.post("/chat/enviar", url.Values{"message": {"oi"}, "agent": {"<script>"}})
	msgs, _ = a.st.ListChat(context.Background())
	if msgs[3].Agent != "luna" || msgs[5].Agent != "jade" {
		t.Errorf("quem respondeu = %q, %q", msgs[3].Agent, msgs[5].Agent)
	}
	if _, page := a.get("/chat"); !strings.Contains(page, "Luna · Alertas") || !strings.Contains(page, `data-agent="otavio"`) {
		t.Error("a página do chat deveria mostrar quem respondeu e o time para escolher")
	}
}

func TestChatWhenOllamaOffline(t *testing.T) {
	off := fakeOllama(t)
	off.Close()
	a := newApp(t, off.URL)
	if _, body := a.get("/chat"); !strings.Contains(body, "O Ollama não está rodando") {
		t.Error("página do chat deveria explicar como instalar o Ollama")
	}
	// Sem IA, o personagem responde com as contas do app e diz por que a IA falhou.
	_, body := a.post("/chat/enviar", url.Values{"message": {"oi"}})
	if !strings.Contains(body, `"app":{`) || !strings.Contains(body, "A IA não respondeu agora") || !strings.Contains(body, "ollama.com") || strings.Contains(body, `"limit":true`) {
		t.Errorf("SSE = %s", body)
	}
	if msgs, _ := a.st.ListChat(context.Background()); len(msgs) != 2 || !strings.HasPrefix(msgs[1].Content, advisor.OfflineNotice) {
		t.Errorf("a resposta sem IA fica na conversa: %+v", msgs)
	}
}

// Sugestão com resposta pronta: o Go responde sem chamar a IA.
func TestChatPreparedSkipsAI(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			t.Error("não devia chamar a IA")
		}
		fmt.Fprint(w, `{"models":[{"name":"qwen3:4b"}]}`)
	}))
	t.Cleanup(ollama.Close)
	a := newApp(t, ollama.URL)
	if _, page := a.get("/chat"); !strings.Contains(page, `data-for="bento"`) || !strings.Contains(page, "Quando eu fico livre das dívidas?") {
		t.Error("a página deveria ter as sugestões de cada personagem")
	}
	_, body := a.post("/chat/enviar", url.Values{"message": {"Qual dívida devo pagar primeiro?"}, "agent": {"bento"}})
	if !strings.Contains(body, `"app":{`) || strings.Contains(body, "A IA não respondeu") || !strings.Contains(body, "Nenhuma dívida") {
		t.Errorf("SSE = %s", body)
	}
}

// As sugestões do app (js/pages/advisor.js) são as mesmas do Go: as prontas precisam bater.
func TestSuggestionsMatchApp(t *testing.T) {
	js, err := os.ReadFile("static/js/pages/advisor.js")
	if err != nil {
		t.Fatal(err)
	}
	for agent, list := range advisor.Suggestions {
		for _, q := range list {
			if !strings.Contains(string(js), `"`+q+`"`) {
				t.Errorf("%s: %q não está em advisor.js", agent, q)
			}
		}
	}
}

// Grátis do finan_rich no limite: além da resposta sem IA, o aviso para usar a própria chave.
func TestChatFreeLimit(t *testing.T) {
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"o consultor grátis chegou ao limite deste aparelho por hoje"}}`)
	}))
	t.Cleanup(free.Close)
	st, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s, err := New(st, Options{AIPreset: llm.FreeID, AIDefaults: llm.Config{Kind: llm.KindOpenAI, BaseURL: free.URL, Model: "auto", APIKey: "aparelho"}, Port: "8080"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Routes())
	t.Cleanup(srv.Close)
	a := &app{t: t, srv: srv, st: st}
	if _, body := a.post("/chat/enviar", url.Values{"message": {"e a moto?"}}); !strings.Contains(body, `"limit":true`) || !strings.Contains(body, "A IA não respondeu agora") {
		t.Errorf("SSE = %s", body)
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
	_, body := a.get("/configuracoes")
	if strings.Contains(body, "segredo") {
		t.Error("a chave não pode aparecer no HTML")
	}
	// Dois caminhos para a IA: a grátis do finan_rich ou a chave da pessoa
	// (com a chave salva, o segundo vem marcado).
	for _, want := range []string{`name="caminho" value="finan"`, `name="caminho" value="propria" checked`, "Usar a IA do finan_rich", "Usar a minha própria chave"} {
		if !strings.Contains(body, want) {
			t.Errorf("ajustes deveriam ter %q", want)
		}
	}
}

func TestPullModel(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	if _, body := a.post("/configuracoes/baixar-modelo", nil); !strings.Contains(body, `"status":"success"`) || !strings.Contains(body, `"done":true`) {
		t.Errorf("pull = %s", body)
	}
}

func TestCategorizeAPI(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"qwen3:4b"}]}`)
		case "/api/chat":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "IFOOD") || strings.Contains(string(body), "8750") {
				t.Errorf("a IA deve receber só as descrições: %s", body)
			}
			fmt.Fprintln(w, `{"message":{"content":"{\"1\": \"Restaurantes e delivery\", \"2\": \"?\"}"},"done":true}`)
		}
	}))
	defer ollama.Close()
	a := newApp(t, ollama.URL)
	code, res := a.postJSON("/api/categorizar", map[string]any{"items": []map[string]string{
		{"description": "IFOOD *RESTAURANTE", "kind": "saida"}, {"description": "XPTO 123", "kind": "saida"},
	}})
	cats, _ := res["categories"].([]any)
	if code != http.StatusOK || len(cats) != 2 || cats[0] != "cat-11" || cats[1] != "" {
		t.Fatalf("categorizar = %d %v", code, res)
	}
	if code, _ := a.postJSON("/api/categorizar", map[string]any{"items": []any{}}); code != http.StatusBadRequest {
		t.Errorf("lista vazia = %d", code)
	}
}

func TestChatAPI(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	getInfo := func() map[string]any {
		resp, err := http.Get(a.srv.URL + "/api/chat")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var v map[string]any
		json.NewDecoder(resp.Body).Decode(&v)
		return v
	}
	if info := getInfo(); info["ready"] != true || len(info["history"].([]any)) != 0 {
		t.Fatalf("info = %v", info)
	}
	b, _ := json.Marshal(map[string]string{"message": "Quanto gastei com iFood?"})
	resp, err := http.Post(a.srv.URL+"/api/chat", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), `"text":"Pague o cartão."`) {
		t.Fatalf("resposta = %s", body)
	}
	if h := getInfo()["history"].([]any); len(h) != 2 || h[1].(map[string]any)["agent"] != "jade" {
		t.Errorf("histórico = %v", h)
	}
	if code, _ := a.postJSON("/api/chat/limpar", map[string]any{}); code != http.StatusOK || len(getInfo()["history"].([]any)) != 0 {
		t.Error("limpar a conversa")
	}
}

// O consultor propõe um lançamento: o cartão vai no SSE, fica salvo na
// conversa e aparece como feito depois que a pessoa confirma.
func TestChatProposalsAreSaved(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			fmt.Fprint(w, `{"models":[{"name":"qwen3:4b"}]}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"role":"tool"`) {
			fmt.Fprintln(w, `{"message":{"content":"Deixei pronto para você confirmar."},"done":true}`)
			return
		}
		fmt.Fprintln(w, `{"message":{"content":"","tool_calls":[{"function":{"name":"propor_lancamento","arguments":{"descricao":"Cinema","valor":"30","categoria":"Lazer"}}}]},"done":true}`)
	}))
	t.Cleanup(ollama.Close)
	a := newApp(t, ollama.URL)
	_, body := a.post("/chat/enviar", url.Values{"message": {"lança 30 do cinema"}})
	if !strings.Contains(body, `"proposal":{"action":"criar","type":"transaction"`) || !strings.Contains(body, "Deixei pronto") {
		t.Fatalf("SSE = %s", body)
	}
	if _, page := a.get("/chat"); !strings.Contains(page, `data-proposals="[{&#34;action&#34;:&#34;criar&#34;`) || strings.Contains(page, "&#34;applied&#34;:true") {
		t.Fatalf("a proposta deveria ficar na conversa, ainda não feita: %s", page)
	}
	// A pessoa confirma: o aparelho grava o registro proposto e sincroniza.
	msgs, _ := a.st.ListChat(context.Background())
	var props []struct {
		Type   string
		Record json.RawMessage
	}
	json.Unmarshal(msgs[1].Proposals, &props)
	if _, err := a.st.ApplyChanges(context.Background(), []store.Change{{Type: props[0].Type, Record: props[0].Record}}); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(a.srv.URL + "/api/chat")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	api, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(api), `"applied":true`) || !strings.Contains(string(api), `"label":"Novo gasto:`) {
		t.Errorf("API do app: %s", api)
	}
}

// Com IA na nuvem, a categorização não manda CPF, conta nem "PIX ENVIADO".
func TestCategorizeCloudSendsOnlyKeys(t *testing.T) {
	var sent string
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			body, _ := io.ReadAll(r.Body)
			sent = string(body)
			fmt.Fprintln(w, `{"message":{"content":"{\"1\": \"?\"}"},"done":true}`)
		}
	}))
	defer ollama.Close()
	a := newApp(t, ollama.URL)
	resp, err := noRedirect.PostForm(a.srv.URL+"/configuracoes", url.Values{"preset": {"ollama-cloud"}, "model": {"qwen3.5:cloud"}, "base_url": {ollama.URL}})
	if err != nil || resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("salvar IA: %v %v", err, resp)
	}
	if code, res := a.postJSON("/api/categorizar", map[string]any{"items": []map[string]string{
		{"description": "PIX ENVIADO 123.456.789-00 FARMACIA SAO JOAO", "kind": "saida"},
	}}); code != http.StatusOK {
		t.Fatalf("categorizar = %d %v", code, res)
	}
	if strings.Contains(sent, "123.456") || strings.Contains(sent, "PIX ENVIADO") || !strings.Contains(sent, "farmacia sao joao") {
		t.Errorf("para a nuvem deveria ir só a assinatura da descrição: %s", sent)
	}
}

// Envio acima do limite recebe 413 com mensagem clara (o app manda em lotes).
func TestSyncTooBig(t *testing.T) {
	a := newApp(t, "")
	big := `{"cursor":0,"changes":[` + strings.Repeat(`{"type":"transaction","record":{"id":"x","description":"`+strings.Repeat("a", 1000)+`"}},`, 11000) + `{}]}`
	resp, err := http.Post(a.srv.URL+"/api/sync", "application/json", strings.NewReader(big))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), "grande demais") {
		t.Errorf("status = %d %s", resp.StatusCode, body)
	}
}

// Arquivos estáticos: gzip quando o navegador aceita, ETag para revalidar.
func TestStaticGzipAndETag(t *testing.T) {
	a := newApp(t, "")
	req, _ := http.NewRequest("GET", a.srv.URL+"/static/app.css", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultTransport.RoundTrip(req) // sem descompactar sozinho
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Encoding") != "gzip" || etag == "" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/css") {
		t.Fatalf("status=%d headers=%v", resp.StatusCode, resp.Header)
	}
	req.Header.Set("If-None-Match", etag)
	// Os corpos são fechados: um corpo grande não lido (o wasm) prende o
	// servidor de teste no Close.
	if resp, _ := http.DefaultTransport.RoundTrip(req); resp.StatusCode != http.StatusNotModified {
		t.Errorf("revalidação = %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp, _ := http.Get(a.srv.URL + "/static/nao-existe.js"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("inexistente = %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	resp, err = http.Get(a.srv.URL + "/static/finance.wasm")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 && resp.Header.Get("Content-Type") != "application/wasm" {
		t.Errorf("wasm com tipo %q", resp.Header.Get("Content-Type"))
	}
}
