package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOAuthRelay(t *testing.T) {
	var forms []map[string]string
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		f := map[string]string{}
		for k := range r.PostForm {
			f[k] = r.PostForm.Get(k)
		}
		forms = append(forms, f)
		if f["code"] == "errado" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Write([]byte(`{"access_token":"acesso","expires_in":3599,"refresh_token":"renova"}`))
	}))
	defer google.Close()
	h := New(Config{Upstream: "http://127.0.0.1:9", Models: []string{"m"}, Limits: DefaultLimits,
		GoogleClientID: "id-123.apps.googleusercontent.com", GoogleClientSecret: "segredo-do-cliente", GoogleTokenURL: google.URL})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Forwarded-For", "200.1.2.3")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := call("GET", "/oauth/config", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "id-123") || strings.Contains(rec.Body.String(), "segredo") {
		t.Errorf("config = %d %s", rec.Code, rec.Body.String())
	}
	rec := call("POST", "/oauth/token", `{"code":"c1","verifier":"v1","redirectUri":"https://site/oauth"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"refresh_token":"renova"`) || strings.Contains(rec.Body.String(), "segredo") {
		t.Fatalf("troca = %d %s", rec.Code, rec.Body.String())
	}
	f := forms[0]
	if f["grant_type"] != "authorization_code" || f["code_verifier"] != "v1" || f["client_secret"] != "segredo-do-cliente" || f["redirect_uri"] != "https://site/oauth" {
		t.Errorf("enviado ao Google = %v", f)
	}
	if rec := call("POST", "/oauth/refresh", `{"refreshToken":"renova"}`); rec.Code != http.StatusOK || forms[1]["grant_type"] != "refresh_token" || forms[1]["refresh_token"] != "renova" {
		t.Errorf("renovar = %d %v", rec.Code, forms[1])
	}
	if rec := call("POST", "/oauth/token", `{"code":"errado","verifier":"v","redirectUri":"https://site/oauth"}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_grant") {
		t.Errorf("código errado = %d %s (o erro do Google chega ao app)", rec.Code, rec.Body.String())
	}
	for _, body := range []string{`{"code":"c","verifier":"","redirectUri":"https://site/oauth"}`, `{"code":"c","verifier":"v","redirectUri":"http://site/oauth"}`, `{"code":"c","verifier":"v","redirectUri":"http://127.0.0.1.evil.com/oauth"}`, `não é json`} {
		if rec := call("POST", "/oauth/token", body); rec.Code != http.StatusBadRequest {
			t.Errorf("pedido inválido %s = %d", body, rec.Code)
		}
	}
	if len(forms) != 3 {
		t.Errorf("pedidos inválidos não devem chegar ao Google: %d chamadas", len(forms))
	}

	// Sem as variáveis na Vercel: tudo 404 (o app não mostra o botão).
	off := New(Config{Upstream: "http://127.0.0.1:9", Models: []string{"m"}, Limits: DefaultLimits})
	for _, path := range []string{"/oauth/config", "/oauth/token"} {
		rec := httptest.NewRecorder()
		method := "GET"
		if path == "/oauth/token" {
			method = "POST"
		}
		off.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s sem configuração = %d", path, rec.Code)
		}
	}
}

func TestOAuthRateLimit(t *testing.T) {
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	defer google.Close()
	h := New(Config{Upstream: "http://127.0.0.1:9", Models: []string{"m"}, Limits: DefaultLimits, GoogleClientID: "id", GoogleClientSecret: "s", GoogleTokenURL: google.URL})
	last := 0
	for i := 0; i < oauthLimits.DeviceHour+1; i++ {
		req := httptest.NewRequest("POST", "/oauth/refresh", strings.NewReader(`{"refreshToken":"r"}`))
		req.Header.Set("X-Forwarded-For", "200.9.9.9")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("depois do limite = %d, quer 429", last)
	}
}

// No site, o token de renovação fica num cookie HttpOnly (o JavaScript da
// página não lê): entrar guarda no cookie e não devolve no corpo; renovar lê
// do cookie; sair apaga o cookie e revoga o acesso no Google.
func TestOAuthSiteCookie(t *testing.T) {
	var forms []map[string]string
	var revoked []string
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.URL.Path == "/revoke" {
			revoked = append(revoked, r.PostForm.Get("token"))
			return
		}
		f := map[string]string{}
		for k := range r.PostForm {
			f[k] = r.PostForm.Get(k)
		}
		forms = append(forms, f)
		w.Write([]byte(`{"access_token":"acesso","expires_in":3599,"refresh_token":"renova","id_token":"x.e30.x"}`))
	}))
	defer google.Close()
	h := New(Config{Upstream: "http://127.0.0.1:9", Models: []string{"m"}, Limits: DefaultLimits,
		GoogleClientID: "id", GoogleClientSecret: "segredo", GoogleTokenURL: google.URL + "/token", GoogleRevokeURL: google.URL + "/revoke"})
	call := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("X-Forwarded-For", "200.1.2.4")
		req.Header.Set("X-Finan-Cookie", "1")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	rec := call("/oauth/token", `{"code":"c1","verifier":"v1","redirectUri":"https://site/oauth"}`, nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "renova") || !strings.Contains(rec.Body.String(), `"access_token":"acesso"`) || !strings.Contains(rec.Body.String(), `"cookie":true`) {
		t.Fatalf("troca no site = %d %s", rec.Code, rec.Body.String())
	}
	var rt *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "__Host-finan_rt" {
			rt = c
		}
	}
	if rt == nil || rt.Value != "renova" || !rt.HttpOnly || !rt.Secure || rt.SameSite != http.SameSiteStrictMode || rt.Path != "/" || rt.MaxAge <= 0 {
		t.Fatalf("cookie = %+v", rt)
	}
	if rec := call("/oauth/refresh", `{}`, rt); rec.Code != http.StatusOK || forms[1]["refresh_token"] != "renova" {
		t.Errorf("renovar pelo cookie = %d %v", rec.Code, forms[1])
	}
	if rec := call("/oauth/refresh", `{}`, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("renovar sem cookie = %d (quer 401: entre de novo)", rec.Code)
	}
	// Quem entrou antes do cookie manda o token no corpo uma vez: ele vira cookie.
	rec = call("/oauth/refresh", `{"refreshToken":"antigo"}`, nil)
	migrated := false
	for _, c := range rec.Result().Cookies() {
		migrated = migrated || c.Name == "__Host-finan_rt" && c.Value == "antigo" && c.HttpOnly
	}
	if rec.Code != http.StatusOK || !migrated {
		t.Errorf("migração do token antigo para o cookie = %d %v", rec.Code, migrated)
	}
	rec = call("/oauth/logout", `{}`, rt)
	gone := false
	for _, c := range rec.Result().Cookies() {
		gone = gone || c.Name == "__Host-finan_rt" && c.MaxAge < 0
	}
	if rec.Code != http.StatusOK || !gone || len(revoked) != 1 || revoked[0] != "renova" {
		t.Errorf("sair = %d, cookie apagado %v, revogados %v", rec.Code, gone, revoked)
	}
}
