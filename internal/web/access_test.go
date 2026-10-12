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
	changes atomic.Int32
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
	})
	if err != nil {
		t.Fatal(err)
	}
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
