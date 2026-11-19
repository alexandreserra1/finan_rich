package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/auth"
)

func (a *accessApp) totp(at time.Time) string {
	a.t.Helper()
	set, _ := a.st.Settings(context.Background(), accessPrefix)
	secret := set[keyTOTP]
	if secret == "" {
		secret = set[keyTOTPPending]
	}
	code, err := auth.TOTPCode(secret, at)
	if err != nil {
		a.t.Fatal(err)
	}
	return code
}

var recoveryRe = regexp.MustCompile(`<code>([a-z2-9]{4}-[a-z2-9]{4}-[a-z2-9]{4})</code>`)

// enableMFA liga a verificação em duas etapas neste computador e devolve os códigos de recuperação.
func (a *accessApp) enableMFA() []string {
	a.t.Helper()
	if loc := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"mfa-iniciar"}}).Header().Get("Location"); !strings.Contains(loc, "msg=") {
		a.t.Fatalf("iniciar: %s", loc)
	}
	page := a.do("GET", "/configuracoes", local, nil).Body.String()
	if !strings.Contains(page, `alt="QR code para o app autenticador"`) || !strings.Contains(page, "data:image/png;base64,") {
		a.t.Fatal("sem o QR code do app autenticador nos Ajustes")
	}
	if loc := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"mfa-confirmar"}, "codigo": {"000000"}}).Header().Get("Location"); !strings.Contains(loc, "erro=") {
		a.t.Fatalf("código errado ligou: %s", loc)
	}
	rec := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"mfa-confirmar"}, "codigo": {a.totp(time.Now())}})
	var codes []string
	for _, m := range recoveryRe.FindAllStringSubmatch(rec.Body.String(), -1) {
		codes = append(codes, m[1])
	}
	if rec.Code != http.StatusOK || len(codes) != 8 || rec.Header().Get("Cache-Control") != "no-store" {
		a.t.Fatalf("confirmar: %d, %d códigos, cache %q", rec.Code, len(codes), rec.Header().Get("Cache-Control"))
	}
	return codes
}

// login pela rede com senha e, se pedido, código; devolve a sessão (nil se não entrou).
func (a *accessApp) loginWithCode(password, code string) (*http.Cookie, string) {
	a.t.Helper()
	rec := a.do("POST", "/entrar", phone, url.Values{"senha": {password}, "volta": {"/dividas"}})
	if s := sessionFrom(rec); s != nil {
		return s, rec.Header().Get("Location")
	}
	var mfa *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == mfaCookie && c.Value != "" {
			mfa = c
		}
	}
	if mfa == nil {
		return nil, rec.Header().Get("Location")
	}
	if !mfa.HttpOnly || !strings.HasPrefix(rec.Header().Get("Location"), "/entrar/codigo") {
		a.t.Fatalf("depois da senha: %s, cookie %+v", rec.Header().Get("Location"), mfa)
	}
	if page := a.do("GET", "/entrar/codigo?volta=%2Fdividas", phone, nil, mfa).Body.String(); !strings.Contains(page, `name="codigo"`) {
		a.t.Fatal("sem o campo do código")
	}
	rec = a.do("POST", "/entrar/codigo", phone, url.Values{"codigo": {code}, "volta": {"/dividas"}}, mfa)
	return sessionFrom(rec), rec.Header().Get("Location")
}

func TestMFALogin(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	before := sessionFrom(a.do("POST", "/entrar", phone, url.Values{"senha": {"senha-forte-1"}}))
	codes := a.enableMFA()
	if rec := a.do("GET", "/", phone, nil, before); rec.Code != http.StatusSeeOther {
		t.Error("a sessão aberta só com a senha continuou valendo depois de ligar o código")
	}

	// Senha certa não basta: sem o código, não há sessão.
	if s, loc := a.loginWithCode("senha-forte-1", "123456"); s != nil || !strings.Contains(loc, "erro=") {
		t.Fatalf("código errado entrou: %s", loc)
	}
	// O código usado para confirmar já foi gasto: vale o próximo (o app troca a cada 30 s).
	if s, _ := a.loginWithCode("senha-forte-1", a.totp(time.Now())); s != nil {
		t.Error("o código usado para ligar entrou de novo")
	}
	code := a.totp(time.Now().Add(30 * time.Second))
	s, loc := a.loginWithCode("senha-forte-1", code)
	if s == nil || loc != "/dividas" {
		t.Fatalf("senha e código certos: %s", loc)
	}
	if rec := a.do("GET", "/dividas", phone, nil, s); rec.Code != http.StatusOK {
		t.Errorf("com sessão: %d", rec.Code)
	}
	// O mesmo código não vale duas vezes (quem espiou não entra depois).
	if s, _ := a.loginWithCode("senha-forte-1", code); s != nil {
		t.Error("código reaproveitado entrou")
	}
	// Código de recuperação: vale uma vez.
	if s, _ := a.loginWithCode("senha-forte-1", codes[0]); s == nil {
		t.Error("código de recuperação recusado")
	}
	if s, _ := a.loginWithCode("senha-forte-1", codes[0]); s != nil {
		t.Error("código de recuperação usado duas vezes")
	}
	// Sem ter acertado a senha antes, o passo do código não abre nada.
	if rec := a.do("POST", "/entrar/codigo", phone2, url.Values{"codigo": {a.totp(time.Now().Add(30 * time.Second))}}); sessionFrom(rec) != nil || !strings.HasPrefix(rec.Header().Get("Location"), "/entrar?") {
		t.Errorf("código sem senha: %s", rec.Header().Get("Location"))
	}
	// O código tem um limite só, somando todos os endereços: trocar de IP não dá tentativas novas.
	for i := range 12 {
		from := fmt.Sprintf("192.168.0.%d:50000", 100+i)
		rec := a.do("POST", "/entrar", from, url.Values{"senha": {"senha-forte-1"}})
		var mfa *http.Cookie
		for _, c := range rec.Result().Cookies() {
			if c.Name == mfaCookie {
				mfa = c
			}
		}
		if mfa == nil {
			t.Fatalf("tentativa %d: a senha certa não levou ao código", i)
		}
		loc := a.do("POST", "/entrar/codigo", from, url.Values{"codigo": {"999999"}}, mfa).Header().Get("Location")
		if blocked := strings.Contains(loc, "Muitas+tentativas"); blocked != (i >= 9) {
			t.Fatalf("tentativa %d de outro endereço: %s", i, loc)
		}
	}
	events, _ := a.st.AccessLog(context.Background(), 50)
	var log []string
	for _, e := range events {
		log = append(log, e.Event)
	}
	for _, want := range []string{"Verificação em duas etapas ligada", "Código de verificação errado", "Código de recuperação usado", "Entrou pela rede"} {
		if !strings.Contains(strings.Join(log, "|"), want) {
			t.Errorf("registro sem %q: %v", want, log)
		}
	}
}

// Mudar a segurança pela rede pede a senha atual e o código: quem pegar uma sessão
// aberta não troca a senha, não desliga o código nem o acesso. Neste computador, não pede.
func TestSecurityChangesNeedReauth(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	codes := a.enableMFA()
	s, _ := a.loginWithCode("senha-forte-1", codes[0]) // o próximo código de 6 dígitos fica para a reautenticação
	if s == nil {
		t.Fatal("sem sessão")
	}
	ctx := context.Background()
	hash := func() string { set, _ := a.st.Settings(ctx, accessPrefix); return set[keyPasswordHash] }
	old := hash()
	for _, form := range []url.Values{
		{"acao": {"senha"}, "senha": {"nova-senha-123"}, "confirmar": {"nova-senha-123"}},
		{"acao": {"senha"}, "senha": {"nova-senha-123"}, "confirmar": {"nova-senha-123"}, "senha_atual": {"errada-000"}, "codigo": {a.totp(time.Now().Add(30 * time.Second))}},
		{"acao": {"senha"}, "senha": {"nova-senha-123"}, "confirmar": {"nova-senha-123"}, "senha_atual": {"senha-forte-1"}, "codigo": {"000000"}},
		{"acao": {"mfa-desligar"}, "senha_atual": {"senha-forte-1"}},
		{"acao": {"desligar"}},
	} {
		if loc := a.do("POST", "/configuracoes/acesso", phone, form, s).Header().Get("Location"); !strings.Contains(loc, "erro=") {
			t.Errorf("%v sem reautenticar: %s", form, loc)
		}
	}
	set, _ := a.st.Settings(ctx, accessPrefix)
	if hash() != old || set[keyTOTP] == "" || set[keyLAN] != "1" {
		t.Fatal("algo mudou sem a senha atual e o código")
	}
	// As cinco tentativas erradas esgotaram o limite deste endereço; de outro, com a senha e o código certos, vale.
	form := url.Values{"acao": {"senha"}, "senha": {"nova-senha-123"}, "confirmar": {"nova-senha-123"}, "senha_atual": {"senha-forte-1"}, "codigo": {a.totp(time.Now().Add(30 * time.Second))}}
	if loc := a.do("POST", "/configuracoes/acesso", phone2, form, s).Header().Get("Location"); !strings.Contains(loc, "msg=") || hash() == old {
		t.Fatalf("com a senha atual e o código: %s", loc)
	}
	// Neste computador, desligar o código não pede nada (quem perdeu o celular resolve aqui).
	if loc := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"mfa-desligar"}}).Header().Get("Location"); !strings.Contains(loc, "msg=") {
		t.Fatalf("desligar no computador: %s", loc)
	}
	if set, _ := a.st.Settings(ctx, accessPrefix); set[keyTOTP] != "" || set[keyTOTPRecovery] != "" {
		t.Error("o código continuou ligado")
	}
}

// Ligar o acesso pelo celular já oferece o código de 6 dígitos (marcado por padrão).
func TestEnableLANOffersMFA(t *testing.T) {
	a := newAccessApp(t, "")
	if page := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(page, `name="mfa" value="1" checked`) {
		t.Error("o formulário de ligar o acesso não oferece o código")
	}
	rec := a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"ligar"}, "senha": {"senha-forte-1"}, "confirmar": {"senha-forte-1"}, "mfa": {"1"}})
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "#mfa") {
		t.Errorf("depois de ligar: %s", loc)
	}
	if set, _ := a.st.Settings(context.Background(), accessPrefix); set[keyTOTPPending] == "" || set[keyLAN] != "1" {
		t.Errorf("acesso e código: %v", set)
	}
}
