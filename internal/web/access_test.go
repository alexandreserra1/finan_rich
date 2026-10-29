package web

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// accessApp chama o handler direto, para poder simular o IP de origem.
type accessApp struct {
	t       *testing.T
	h       http.Handler
	st      *store.Store
	srv     *Server
	changes atomic.Int32
	quits   atomic.Int32
}

func newAccessApp(t *testing.T, envPassword string) *accessApp {
	t.Helper()
	st, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	a := &accessApp{t: t, st: st}
	s, err := New(st, Options{
		AIPreset: "ollama", AIDefaults: llm.Config{Kind: llm.KindOllama, BaseURL: "http://127.0.0.1:1", Model: "m"},
		Port: "8080", Password: envPassword, OnNetworkChange: func() { a.changes.Add(1) },
		OnQuit: func() { a.quits.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	a.srv = s
	s.tailscaleURL = func(context.Context) string { return "" }
	a.h = s.Routes()
	return a
}

const (
	local  = "127.0.0.1:50000"
	phone  = "192.168.0.50:50000"
	phone2 = "192.168.0.51:50000"
)

func (a *accessApp) do(method, path, from string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	req.Host = "127.0.0.1:8080" // como o navegador manda (o padrão "example.com" seria um site de fora)
	req.RemoteAddr = from
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func sessionFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	return nil
}

func (a *accessApp) enableLAN(password string) {
	a.t.Helper()
	rec := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"ligar"}, "senha": {password}, "confirmar": {password}})
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/configuracoes?msg=") || !strings.HasSuffix(loc, "#acesso") {
		a.t.Fatalf("ligar acesso: %d %s", rec.Code, loc)
	}
}

func TestLocalNeedsNoPassword(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	if rec := a.do("GET", "/", local, nil); rec.Code != http.StatusOK {
		t.Errorf("computador local deveria entrar direto: %d", rec.Code)
	}
}

func TestPhoneMustLogin(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")

	rec := a.do("GET", "/dividas", phone, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/entrar?volta=%2Fdividas" {
		t.Fatalf("celular sem login: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := a.do("POST", "/chat/limpar", phone, url.Values{}); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST sem login = %d", rec.Code)
	}
	for _, path := range []string{"/entrar", "/manifest.webmanifest", "/sw.js", "/offline", "/static/app.css", "/static/icons/icon-192.png"} {
		if rec := a.do("GET", path, phone, nil); rec.Code != http.StatusOK {
			t.Errorf("%s deveria ser público: %d", path, rec.Code)
		}
	}

	if rec := a.do("POST", "/entrar", phone, url.Values{"senha": {"errada-123"}, "volta": {"/dividas"}}); sessionFrom(rec) != nil || !strings.Contains(rec.Header().Get("Location"), "erro=") {
		t.Errorf("senha errada não pode criar sessão")
	}
	rec = a.do("POST", "/entrar", phone, url.Values{"senha": {"senha-forte-1"}, "volta": {"/dividas"}})
	cookie := sessionFrom(rec)
	if cookie == nil || rec.Header().Get("Location") != "/dividas" || !cookie.HttpOnly {
		t.Fatalf("login certo: %d %s %+v", rec.Code, rec.Header().Get("Location"), cookie)
	}
	if rec := a.do("GET", "/dividas", phone, nil, cookie); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Sair") {
		t.Errorf("com sessão: %d", rec.Code)
	}

	if rec := a.do("POST", "/sair", phone, url.Values{}, cookie); rec.Code != http.StatusSeeOther {
		t.Errorf("sair = %d", rec.Code)
	}
	if rec := a.do("GET", "/", phone, nil, cookie); rec.Code != http.StatusSeeOther {
		t.Error("sessão deveria acabar ao sair")
	}
}

func TestLoginRateLimitAndRedirect(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	for range 5 {
		a.do("POST", "/entrar", phone, url.Values{"senha": {"errada-123"}})
	}
	rec := a.do("POST", "/entrar", phone, url.Values{"senha": {"senha-forte-1"}})
	if sessionFrom(rec) != nil || !strings.Contains(rec.Header().Get("Location"), "Muitas+tentativas") {
		t.Errorf("deveria bloquear após 5 erros: %s", rec.Header().Get("Location"))
	}
	// Outro aparelho não é afetado; e "volta" não pode levar para outro site.
	rec = a.do("POST", "/entrar", phone2, url.Values{"senha": {"senha-forte-1"}, "volta": {"//evil.example/x"}})
	if sessionFrom(rec) == nil || rec.Header().Get("Location") != "/" {
		t.Errorf("outro aparelho: %s", rec.Header().Get("Location"))
	}
}

func TestAccessSettings(t *testing.T) {
	a := newAccessApp(t, "")
	ctx := context.Background()

	bad := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"ligar"}, "senha": {"curta"}, "confirmar": {"curta"}})
	diff := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"ligar"}, "senha": {"senha-forte-1"}, "confirmar": {"outra-senha-1"}})
	for _, rec := range []*httptest.ResponseRecorder{bad, diff} {
		if !strings.Contains(rec.Header().Get("Location"), "erro=") {
			t.Errorf("senha inválida aceita: %s", rec.Header().Get("Location"))
		}
	}

	a.enableLAN("senha-forte-1")
	for i := 0; a.changes.Load() == 0 && i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if a.changes.Load() != 1 {
		t.Error("ligar o acesso deveria pedir para reabrir a porta na rede")
	}
	saved, _ := a.st.Settings(ctx, accessPrefix)
	if saved[keyLAN] != "1" || !strings.HasPrefix(saved[keyPasswordHash], "pbkdf2-sha256$") || strings.Contains(saved[keyPasswordHash], "senha-forte-1") {
		t.Errorf("configuração salva = %v", saved)
	}
	page := a.do("GET", "/configuracoes", local, nil).Body.String()
	if !strings.Contains(page, "Ligado.") || strings.Contains(page, "pbkdf2") {
		t.Error("página de ajustes deveria mostrar o acesso ligado sem expor o hash")
	}

	// Trocar a senha desconecta os aparelhos.
	cookie := sessionFrom(a.do("POST", "/entrar", phone, url.Values{"senha": {"senha-forte-1"}}))
	a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"senha"}, "senha": {"senha-nova-22"}, "confirmar": {"senha-nova-22"}})
	if rec := a.do("GET", "/", phone, nil, cookie); rec.Code != http.StatusSeeOther {
		t.Error("sessão antiga deveria cair ao trocar a senha")
	}

	a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"desligar"}})
	if saved, _ := a.st.Settings(ctx, accessPrefix); saved[keyLAN] != "0" {
		t.Errorf("desligar: %v", saved)
	}
}

func TestEnvPasswordProtectsEverything(t *testing.T) {
	a := newAccessApp(t, "senha-do-servidor")
	// Atrás de um proxy reverso tudo parece local, então até o local pede senha.
	if rec := a.do("GET", "/", local, nil); rec.Code != http.StatusSeeOther {
		t.Errorf("com FINAN_PASSWORD o acesso local também pede senha: %d", rec.Code)
	}
	if cookie := sessionFrom(a.do("POST", "/entrar", local, url.Values{"senha": {"senha-do-servidor"}})); cookie == nil {
		t.Error("login com a senha do servidor falhou")
	}
	if rec := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"senha"}, "senha": {"x-123456789"}, "confirmar": {"x-123456789"}}); rec.Code != http.StatusUnauthorized {
		t.Errorf("sem login não pode trocar a senha: %d", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	a := newAccessApp(t, "")
	h := a.do("GET", "/", local, nil).Header()
	if h.Get("X-Frame-Options") != "DENY" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers = %v", h)
	}
	if ct := a.do("GET", "/manifest.webmanifest", local, nil).Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("manifest content-type = %s", ct)
	}
}

func TestLANURLRanking(t *testing.T) {
	ips := map[string]int{"192.168.0.10": 0, "10.0.0.5": 1, "172.17.0.1": 2, "100.101.1.2": 3}
	for ip, want := range ips {
		if got := ipRank(net.ParseIP(ip).To4()); got != want {
			t.Errorf("ipRank(%s) = %d, want %d", ip, got, want)
		}
	}
	for name, want := range map[string]bool{"docker0": true, "br-1a2b": true, "veth9": true, "en0": false, "wlan0": false, "tailscale0": false} {
		if isVirtualInterface(name) != want {
			t.Errorf("isVirtualInterface(%s) != %v", name, want)
		}
	}
}

func TestProxiedRequestIsNotLocal(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	// Tailscale Serve e proxies reversos conectam de 127.0.0.1, mas o pedido vem
	// de outro aparelho: precisa de login.
	for _, header := range []string{"X-Forwarded-For", "Tailscale-User-Login"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "127.0.0.1:8080"
		req.RemoteAddr = local
		req.Header.Set(header, "100.64.0.2")
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: pedido repassado entrou sem login (%d)", header, rec.Code)
		}
	}
}

func TestBackupDB(t *testing.T) {
	a := newAccessApp(t, "")
	rec := a.do("GET", "/configuracoes/backup.db", local, nil)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "SQLite format 3\x00") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), `attachment; filename="finan_rich_`) {
		t.Fatalf("backup do banco: %d %q %.20q", rec.Code, rec.Header().Get("Content-Disposition"), rec.Body.String())
	}
	a.enableLAN("senha-forte-1")
	if rec := a.do("GET", "/configuracoes/backup.db", phone, nil); rec.Code != http.StatusSeeOther {
		t.Errorf("backup pela rede sem login = %d", rec.Code)
	}
}

func TestQuitOnlyFromThisComputer(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, "/configuracoes/fechar") {
		t.Error("Ajustes no computador deveria mostrar o botão de fechar")
	}
	if rec := a.do("POST", "/configuracoes/fechar", phone, nil); rec.Code == http.StatusOK {
		t.Errorf("fechar pelo celular = %d", rec.Code)
	}
	if rec := a.do("POST", "/configuracoes/fechar", local, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "fechado") {
		t.Fatalf("fechar no computador = %d", rec.Code)
	}
	for i := 0; i < 50 && a.quits.Load() == 0; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if a.quits.Load() != 1 {
		t.Errorf("OnQuit chamado %d vezes", a.quits.Load())
	}
}

// Outro site aberto no computador não lê os dados pelo acesso local: nem por
// fetch (com ou sem cabeçalhos Sec-Fetch), nem com um nome que aponta para
// 127.0.0.1 (DNS rebinding). Abrir um link para o app continua valendo.
func TestOtherSitesCannotUseLocalAccess(t *testing.T) {
	a := newAccessApp(t, "")
	get := func(path string, set func(*http.Request)) int {
		req := httptest.NewRequest("GET", path, nil)
		req.Host, req.RemoteAddr = "127.0.0.1:8080", local
		set(req)
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := get("/api/chat", func(r *http.Request) {
		r.Header.Set("Origin", "https://site-malicioso.example")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Sec-Fetch-Mode", "cors")
	}); code != http.StatusForbidden {
		t.Errorf("fetch de outro site leu a conversa: %d", code)
	}
	if code := get("/api/chat", func(r *http.Request) { r.Header.Set("Origin", "https://site-malicioso.example") }); code != http.StatusForbidden {
		t.Errorf("navegador sem Sec-Fetch leu a conversa: %d", code)
	}
	if code := get("/", func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Sec-Fetch-Mode", "navigate")
	}); code != http.StatusOK {
		t.Errorf("link de outro site para o app deveria abrir: %d", code)
	}
	if code := get("/configuracoes/backup.db", func(r *http.Request) { r.Host = "site-malicioso.example:8080" }); code != http.StatusForbidden {
		t.Errorf("DNS rebinding baixou o banco: %d", code)
	}
	for _, host := range []string{"localhost:8080", "[::1]:8080", "meupc.local:8080", "pc.tail1234.ts.net"} {
		if code := get("/", func(r *http.Request) { r.Host = host }); code != http.StatusOK {
			t.Errorf("%s deveria abrir: %d", host, code)
		}
	}
	t.Setenv("FINAN_HOSTS", "financas.casa, outro.nome")
	if code := get("/", func(r *http.Request) { r.Host = "financas.casa:8080" }); code != http.StatusOK {
		t.Errorf("nome em FINAN_HOSTS deveria abrir: %d", code)
	}
}

// O token do celular vale só para a API: com ele não se baixa o banco nem se
// troca a IA (o que mandaria a chave para outro servidor).
func TestDeviceTokenOnlyForAPI(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	token, status := a.pair(a.newCode())
	if status != http.StatusOK {
		t.Fatalf("parear = %d", status)
	}
	if rec := a.api("POST", "/api/sync", phone, token, emptySync); rec.Code != http.StatusOK {
		t.Fatalf("sync com token = %d", rec.Code)
	}
	for _, path := range []string{"/configuracoes/backup.db", "/configuracoes"} {
		if rec := a.api("GET", path, phone, token, ""); rec.Code != http.StatusForbidden {
			t.Errorf("token do celular abriu %s: %d", path, rec.Code)
		}
	}
}

func TestAISettingsKeyAndChatSafety(t *testing.T) {
	a := newAccessApp(t, "")
	ctx := context.Background()
	save := func(v url.Values) {
		if rec := a.do("POST", "/configuracoes", local, v); rec.Code != http.StatusSeeOther {
			t.Fatalf("salvar IA = %d %s", rec.Code, rec.Body.String())
		}
	}
	save(url.Values{"preset": {"ollama"}, "model": {"qwen3.5:9b"}})
	if err := a.srv.store.AppendChatTurn(ctx, "minha nota diz consulta com Dr. X", "ok", "", nil, ""); err != nil {
		t.Fatal(err)
	}
	groq := llm.PresetByID("groq")
	save(url.Values{"preset": {"groq"}, "model": {groq.Model}, "base_url": {groq.BaseURL}, "api_key": {"gsk_segredo"}})
	if turns, _ := a.srv.store.ListChat(ctx); len(turns) != 0 {
		t.Errorf("ao passar da IA local para a nuvem, a conversa antiga deveria sumir: %v", turns)
	}
	save(url.Values{"preset": {"groq"}, "model": {groq.Model}, "base_url": {groq.BaseURL}})
	if ai, _ := a.srv.aiSettings(ctx); ai.APIKey != "gsk_segredo" {
		t.Error("campo em branco deveria manter a chave")
	}
	save(url.Values{"preset": {"groq"}, "model": {groq.Model}, "base_url": {"https://atacante.example/v1"}})
	if ai, _ := a.srv.aiSettings(ctx); ai.APIKey != "" {
		t.Error("trocar o endereço sem colar a chave de novo não pode levar a chave junto")
	}
}

func TestClientKeyGroupsIPv6(t *testing.T) {
	key := func(addr string) string {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addr
		return clientKey(r)
	}
	if a, b := key("[2001:db8:1:2::10]:5000"), key("[2001:db8:1:2:ffff::1]:6000"); a != b || a != "2001:db8:1:2::/64" {
		t.Errorf("mesmo /64 deveria dar a mesma chave: %q e %q", a, b)
	}
	if k := key("192.168.0.7:5000"); k != "192.168.0.7" {
		t.Errorf("IPv4 = %q", k)
	}
}
