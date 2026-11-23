package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/drivesync"
	"github.com/alexandreserra1/finan_rich/internal/gdrive/fakedrive"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// fakeGoogle faz o papel do intermediário (/oauth/*, que tem o client secret) e da
// revogação do Google.
type fakeGoogle struct {
	*httptest.Server
	mu            sync.Mutex
	verifier      string // o que chegou em /oauth/token
	redirect      string
	refreshes     []string
	revoked       []string
	refreshStatus int
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	g := &fakeGoogle{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oauth/config", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"clientId":"cliente-123"}`)
	})
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Code, Verifier, RedirectURI string }
		json.NewDecoder(r.Body).Decode(&in)
		if in.Code != "codigo-1" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"invalid_grant","error_description":"Bad Request"}`)
			return
		}
		g.mu.Lock()
		g.verifier, g.redirect = in.Verifier, in.RedirectURI
		g.mu.Unlock()
		claims := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"pessoa@exemplo.com"}`))
		json.NewEncoder(w).Encode(map[string]any{"access_token": "acesso-1", "refresh_token": "renova-1", "expires_in": 3600, "id_token": "cabecalho." + claims + ".assinatura"})
	})
	mux.HandleFunc("POST /oauth/refresh", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ RefreshToken string }
		json.NewDecoder(r.Body).Decode(&in)
		g.mu.Lock()
		g.refreshes = append(g.refreshes, in.RefreshToken)
		status := g.refreshStatus
		g.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			io.WriteString(w, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)
			return
		}
		io.WriteString(w, `{"access_token":"acesso-2","expires_in":3600}`)
	})
	mux.HandleFunc("POST /revoke", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		g.mu.Lock()
		g.revoked = append(g.revoked, r.Form.Get("token"))
		g.mu.Unlock()
	})
	g.Server = httptest.NewServer(mux)
	t.Cleanup(g.Close)
	return g
}

// seen devolve, sob a trava, o que o intermediário falso recebeu.
func (g *fakeGoogle) seen() (verifier, redirect string, refreshes, revoked []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.verifier, g.redirect, append([]string(nil), g.refreshes...), append([]string(nil), g.revoked...)
}

func googleApp(t *testing.T) (*accessApp, *fakeGoogle, *fakedrive.Drive) {
	t.Helper()
	a := newAccessApp(t, "")
	g := newFakeGoogle(t)
	d := fakedrive.New()
	t.Cleanup(d.Close)
	a.srv.google.server, a.srv.google.drive, a.srv.google.revoke = g.URL, d.URL, g.URL+"/revoke"
	return a, g, d
}

func txItem(id, desc string, at int64) drivesync.Item {
	b, _ := json.Marshal(domain.Transaction{Meta: domain.Meta{ID: id, UpdatedAt: at}, Kind: domain.KindSaida,
		Description: desc, AmountCents: 1000, CategoryID: "cat-3", Date: "2026-11-20"})
	return drivesync.Item{Type: store.TypeTransaction, Record: b}
}

func driveFile(items ...drivesync.Item) []byte {
	b, _ := json.Marshal(drivesync.File{Version: 1, Items: items})
	return b
}

func hasTx(st *store.Store, id, desc string) bool {
	l, _ := st.Ledger(context.Background())
	for _, tx := range l.Transactions {
		if tx.ID == id && tx.Description == desc {
			return true
		}
	}
	return false
}

// editInDrive troca um registro dentro do arquivo do Drive, como outro aparelho faria
// (ele regrava o arquivo inteiro, com tudo o que já estava lá).
func editInDrive(t *testing.T, d *fakedrive.Drive, edited drivesync.Item) {
	t.Helper()
	var f drivesync.File
	if err := json.Unmarshal(d.Content(drivesync.FileName), &f); err != nil {
		t.Fatal(err)
	}
	id := func(it drivesync.Item) string {
		var m struct{ ID string }
		json.Unmarshal(it.Record, &m)
		return it.Type + "/" + m.ID
	}
	for i := range f.Items {
		if id(f.Items[i]) == id(edited) {
			f.Items[i] = edited
		}
	}
	b, _ := json.Marshal(f)
	d.Put(drivesync.FileName, b)
}

func fileHas(d *fakedrive.Drive, id string) bool {
	return strings.Contains(string(d.Content(drivesync.FileName)), `"id":"`+id+`"`)
}

// O caminho inteiro no computador: entrar (só daqui, com PKCE e o state do programa),
// sincronizar com o que o site gravou no Drive, regravar só quando há algo novo,
// renovar o token e sair sem perder dados.
func TestGoogleDesktop(t *testing.T) {
	a, g, d := googleApp(t)
	ctx := context.Background()
	d.RequireToken("acesso-1")
	d.Put(drivesync.FileName, driveFile(txItem("do-site", "Feira", 100))) // o site já gravou
	if _, err := a.st.ApplyChanges(ctx, []store.Change{store.Change(txItem("do-pc", "Padaria", 100))}); err != nil {
		t.Fatal(err)
	}

	rec := a.do("GET", "/google/entrar", local, nil)
	loc, _ := url.Parse(rec.Header().Get("Location"))
	q := loc.Query()
	state := q.Get("state")
	if rec.Code != http.StatusSeeOther || loc.Host != "accounts.google.com" || q.Get("client_id") != "cliente-123" ||
		q.Get("redirect_uri") != g.URL+"/oauth" || q.Get("code_challenge_method") != "S256" || q.Get("access_type") != "offline" ||
		!strings.Contains(q.Get("scope"), "drive.appdata") || !regexp.MustCompile(`^pc\.8080\.[A-Za-z0-9_-]{20,}$`).MatchString(state) {
		t.Fatalf("entrar: %d %s", rec.Code, loc)
	}
	if rec := a.do("GET", "/google/entrar", phone, nil); strings.Contains(rec.Header().Get("Location"), "accounts.google.com") {
		t.Errorf("entrar de outro aparelho começou o login: %s", rec.Header().Get("Location"))
	}
	if rec := a.do("GET", "/google/retorno?code=codigo-1&state=pc.8080.forjado", local, nil); !strings.Contains(rec.Header().Get("Location"), "erro=") {
		t.Errorf("volta com state forjado: %s", rec.Header().Get("Location"))
	}
	if acct, _ := a.st.Settings(ctx, googlePrefix); acct["refresh"] != "" {
		t.Fatalf("state forjado gravou a conta: %v", acct)
	}
	rec = a.do("GET", "/google/retorno?code=codigo-1&state="+url.QueryEscape(state), local, nil)
	verifier, redirect, _, _ := g.seen()
	sum := sha256.Sum256([]byte(verifier))
	if !strings.Contains(rec.Header().Get("Location"), "msg=") || base64.RawURLEncoding.EncodeToString(sum[:]) != q.Get("code_challenge") || redirect != g.URL+"/oauth" {
		t.Fatalf("volta: %s (verificador %q, retorno %q)", rec.Header().Get("Location"), verifier, redirect)
	}
	if acct, _ := a.st.Settings(ctx, googlePrefix); acct["refresh"] != "renova-1" || acct["email"] != "pessoa@exemplo.com" {
		t.Fatalf("conta: %v", acct)
	}
	if rec := a.do("GET", "/google/retorno?code=codigo-1&state="+url.QueryEscape(state), local, nil); !strings.Contains(rec.Header().Get("Location"), "erro=") {
		t.Errorf("o mesmo state valeu duas vezes: %s", rec.Header().Get("Location"))
	}

	// Primeira rodada: o lançamento do site chega; o do computador vai para o Drive.
	if err := a.srv.syncGoogle(ctx); err != nil {
		t.Fatal(err)
	}
	if !hasTx(a.st, "do-site", "Feira") || !fileHas(d, "do-site") || !fileHas(d, "do-pc") || d.Writes() != 1 {
		t.Fatalf("primeira rodada: gravações %d, arquivo %s", d.Writes(), d.Content(drivesync.FileName))
	}
	if err := a.srv.syncGoogle(ctx); err != nil || d.Writes() != 1 {
		t.Errorf("sem mudança regravou: %v, gravações %d", err, d.Writes())
	}
	// Um aparelho sincroniza com o computador: pede uma rodada logo, e ela regrava.
	body, _ := json.Marshal(map[string]any{"cursor": 0, "changes": []drivesync.Item{txItem("do-celular", "Uber", 200)}})
	req := httptest.NewRequest("POST", "/api/sync", bytes.NewReader(body))
	req.Host, req.RemoteAddr = "127.0.0.1:8080", local
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	a.h.ServeHTTP(res, req)
	if res.Code != http.StatusOK || len(a.srv.google.soon) != 1 {
		t.Fatalf("/api/sync: %d, rodadas pedidas %d", res.Code, len(a.srv.google.soon))
	}
	if err := a.srv.syncGoogle(ctx); err != nil || !fileHas(d, "do-celular") || d.Writes() != 2 {
		t.Errorf("depois do celular: %v, gravações %d", err, d.Writes())
	}
	// Outro aparelho edita no Drive: a edição chega aqui, sem regravar.
	editInDrive(t, d, txItem("do-site", "Feira do sábado", 300))
	if err := a.srv.syncGoogle(ctx); err != nil || !hasTx(a.st, "do-site", "Feira do sábado") || d.Writes() != 2 {
		t.Errorf("edição de outro aparelho: %v, gravações %d", err, d.Writes())
	}
	// O Drive recusa o token de acesso: renova com o de renovação e tenta de novo.
	d.RequireToken("acesso-2")
	if _, err := a.st.ApplyChanges(ctx, []store.Change{store.Change(txItem("do-pc-2", "Café", 400))}); err != nil {
		t.Fatal(err)
	}
	if err := a.srv.syncGoogle(ctx); err != nil || !fileHas(d, "do-pc-2") {
		t.Errorf("renovação: %v", err)
	}
	if _, _, refreshes, _ := g.seen(); len(refreshes) != 1 || refreshes[0] != "renova-1" {
		t.Errorf("renovações: %v", refreshes)
	}

	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, "pessoa@exemplo.com") || !strings.Contains(body, `action="/google/sincronizar"`) {
		t.Errorf("Ajustes sem a conta ligada")
	}
	rec = a.do("POST", "/google/sair", local, url.Values{})
	_, _, _, revoked := g.seen()
	if acct, _ := a.st.Settings(ctx, googlePrefix); !strings.Contains(rec.Header().Get("Location"), "msg=") || acct["refresh"] != "" ||
		len(revoked) != 1 || revoked[0] != "renova-1" || !hasTx(a.st, "do-pc", "Padaria") {
		t.Errorf("sair: %s, conta %v, revogados %v", rec.Header().Get("Location"), acct, revoked)
	}
	if err := a.srv.syncGoogle(ctx); !errors.Is(err, errGoogleOff) || d.Writes() != 3 {
		t.Errorf("rodada depois de sair: %v, gravações %d", err, d.Writes())
	}
	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, `href="/google/entrar"`) {
		t.Errorf("Ajustes sem o botão de entrar depois de sair")
	}
}

// A pessoa tirou o acesso na conta Google: a renovação é recusada, a conta sai deste
// computador (os dados ficam) e os Ajustes pedem para entrar de novo.
func TestGoogleRefreshRevoked(t *testing.T) {
	a, g, d := googleApp(t)
	ctx := context.Background()
	a.st.SaveSettings(ctx, googlePrefix, map[string]string{"refresh": "renova-velho", "email": "pessoa@exemplo.com"})
	g.mu.Lock()
	g.refreshStatus = http.StatusBadRequest
	g.mu.Unlock()
	err := a.srv.syncGoogle(ctx)
	if acct, _ := a.st.Settings(ctx, googlePrefix); err == nil || !strings.Contains(err.Error(), "Entre com Google de novo") || acct["refresh"] != "" || d.Writes() != 0 {
		t.Fatalf("renovação recusada: %v, conta %v", err, acct)
	}
	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, "Entre com Google de novo") || !strings.Contains(body, `href="/google/entrar"`) {
		t.Errorf("Ajustes sem o aviso ou sem o botão de entrar")
	}
}

// Erro do Drive (aqui, a API desligada no projeto) aparece nos Ajustes com a
// mensagem do site, e a conta continua ligada.
func TestGoogleDriveError(t *testing.T) {
	a, _, d := googleApp(t)
	ctx := context.Background()
	a.st.SaveSettings(ctx, googlePrefix, map[string]string{"refresh": "renova-1", "email": "pessoa@exemplo.com"})
	d.Fail(func(*http.Request) (int, string) {
		return http.StatusForbidden, `{"error":{"errors":[{"reason":"accessNotConfigured"}]}}`
	})
	if err := a.srv.syncGoogle(ctx); err == nil || !strings.Contains(err.Error(), "não está ativada") {
		t.Fatalf("Drive desligado: %v", err)
	}
	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, "não está ativada") || !strings.Contains(body, "pessoa@exemplo.com") {
		t.Errorf("Ajustes sem o erro do Drive")
	}
}
