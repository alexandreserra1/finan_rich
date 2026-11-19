package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Autorização: o mesmo pedido, de cada tipo de quem pede, contra cada nível de acesso.
// Nível que não está na lista (o zero) nega para todo mundo.
func TestAuthorizeMatrix(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	session := sessionFrom(a.do("POST", "/entrar", phone, url.Values{"senha": {"senha-forte-1"}}))
	token, _ := a.pair(a.newCode())
	if session == nil || token == "" {
		t.Fatal("sem sessão ou token para o teste")
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("liberado")) })
	call := func(who access, kind string) int {
		req := httptest.NewRequest("POST", "/qualquer", strings.NewReader(""))
		req.Host = "192.168.0.10:8080"
		switch kind {
		case "computador":
			req.Host, req.RemoteAddr = "127.0.0.1:8080", local
		case "rede sem login":
			req.RemoteAddr = phone
		case "rede com login":
			req.RemoteAddr = phone
			req.AddCookie(session)
		case "app pareado":
			req.RemoteAddr = phone
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		a.srv.authorize(who, ok).ServeHTTP(rec, req)
		if rec.Code == http.StatusOK && rec.Body.String() != "liberado" {
			t.Fatalf("200 sem passar pela rota")
		}
		return rec.Code
	}
	const yes, no = true, false
	for who, want := range map[access][4]bool{
		//               computador, rede sem login, rede com login, app pareado
		public:        {yes, yes, yes, yes},
		owner:         {yes, no, yes, no},
		ownerOrDevice: {yes, no, yes, yes},
		localOnly:     {yes, no, no, no},
		0:             {no, no, no, no},
	} {
		for i, kind := range []string{"computador", "rede sem login", "rede com login", "app pareado"} {
			if got := call(who, kind) == http.StatusOK; got != want[i] {
				t.Errorf("nível %d, %s: liberado = %v, quer %v", who, kind, got, want[i])
			}
		}
	}
}

// A tabela de rotas: toda rota tem nível; só as da lista abaixo são públicas; a API é da
// pessoa ou do app pareado; só fechar o programa é exclusivo deste computador.
func TestRouteTable(t *testing.T) {
	a := newAccessApp(t, "")
	publics := map[string]bool{
		"GET /static/": true, "GET /healthz": true, "GET /manifest.webmanifest": true, "GET /sw.js": true, "GET /offline": true,
		"GET /entrar": true, "POST /entrar": true, "GET /entrar/codigo": true, "POST /entrar/codigo": true,
		"GET /parear": true, "POST /api/parear": true,
	}
	seen := map[string]bool{}
	for _, rt := range a.srv.routes() {
		api := strings.Contains(rt.pattern, " /api/") && rt.pattern != "POST /api/parear"
		switch {
		case seen[rt.pattern]:
			t.Errorf("%s repetida", rt.pattern)
		case rt.who < public || rt.who > localOnly:
			t.Errorf("%s sem nível de acesso", rt.pattern)
		case publics[rt.pattern] != (rt.who == public):
			t.Errorf("%s: pública = %v", rt.pattern, rt.who == public)
		case api != (rt.who == ownerOrDevice):
			t.Errorf("%s: o token do app só vale para a API", rt.pattern)
		case (rt.pattern == "POST /configuracoes/fechar") != (rt.who == localOnly):
			t.Errorf("%s: só fechar o programa é exclusivo deste computador", rt.pattern)
		}
		seen[rt.pattern] = true
	}
	for p := range publics {
		if !seen[p] {
			t.Errorf("%s não está na tabela", p)
		}
	}
}
