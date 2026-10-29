package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Login com Google (dados no Drive de cada pessoa; ver docs/GOOGLE.md). O
// Google exige o client secret para trocar o código e renovar o token de um
// cliente "Aplicativo da Web", e o secret não pode ir no app. Estas rotas só
// repassam ao Google com o secret: não guardam nada e não registram tokens.
// O código vem com PKCE: quem interceptar o código sem o verificador (que
// fica no aparelho) não consegue trocá-lo.

// oauthLimits: entrar e renovar são poucas chamadas por aparelho.
var oauthLimits = Limits{DeviceHour: 30, DeviceDay: 100, IPHour: 60, IPDay: 300}

const maxOAuthBody = 8 << 10

// No site (mesmo endereço do intermediário), o token de renovação fica neste
// cookie: HttpOnly (o JavaScript da página não lê, nem um script injetado),
// Secure e SameSite=Strict (não vai em pedido de outro site). A página pede
// esse modo com o cabeçalho X-Finan-Cookie: 1. O app do celular guarda no
// cofre do sistema e continua recebendo o token no corpo.
const rtCookie = "__Host-finan_rt"

const rtMaxAge = 180 * 24 * 60 * 60 // o Google desliga o token parado há 6 meses

func siteMode(r *http.Request) bool { return r.Header.Get("X-Finan-Cookie") == "1" }

func setRT(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: rtCookie, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

// oauthConfig entrega o ID do cliente (não é segredo) para o app montar o
// endereço do login. Sem as variáveis na Vercel, o botão do Google não aparece.
func (h *handler) oauthConfig(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GoogleClientID == "" || h.cfg.GoogleClientSecret == "" {
		fail(w, http.StatusNotFound, "o login com Google ainda não foi configurado")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"clientId": h.cfg.GoogleClientID})
}

// oauthToken troca o código do login (com o verificador PKCE) pelos tokens.
func (h *handler) oauthToken(w http.ResponseWriter, r *http.Request) {
	var in struct{ Code, Verifier, RedirectURI string }
	if !h.oauthStart(w, r, &in) {
		return
	}
	if in.Code == "" || in.Verifier == "" || !redirectOK(in.RedirectURI) {
		fail(w, http.StatusBadRequest, "faltam o código, o verificador ou o endereço de retorno")
		return
	}
	h.oauthRelay(w, r, url.Values{"grant_type": {"authorization_code"}, "code": {in.Code}, "code_verifier": {in.Verifier}, "redirect_uri": {in.RedirectURI}})
}

// oauthRefresh renova o token de acesso (vale 1 hora) com o token de renovação.
func (h *handler) oauthRefresh(w http.ResponseWriter, r *http.Request) {
	var in struct{ RefreshToken string }
	if !h.oauthStart(w, r, &in) {
		return
	}
	if in.RefreshToken != "" && siteMode(r) {
		setRT(w, in.RefreshToken, rtMaxAge) // quem entrou antes do cookie: o token sai do navegador e vem para cá
	}
	if in.RefreshToken == "" && siteMode(r) {
		if c, err := r.Cookie(rtCookie); err == nil {
			in.RefreshToken = c.Value
		}
		if in.RefreshToken == "" {
			fail(w, http.StatusUnauthorized, "entre de novo com Google")
			return
		}
	}
	if in.RefreshToken == "" {
		fail(w, http.StatusBadRequest, "falta o token de renovação")
		return
	}
	h.oauthRelay(w, r, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {in.RefreshToken}})
}

// redirectOK aceita retorno https ou na própria máquina (testes). Quem confere
// de verdade é o Google, pela lista de endereços cadastrada no cliente.
func redirectOK(u string) bool {
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://127.0.0.1:") || strings.HasPrefix(u, "http://localhost:")
}

func (h *handler) oauthStart(w http.ResponseWriter, r *http.Request, in any) bool {
	if h.cfg.GoogleClientID == "" || h.cfg.GoogleClientSecret == "" {
		fail(w, http.StatusNotFound, "o login com Google ainda não foi configurado")
		return false
	}
	if msg := h.oauthLimiter.allow("", clientIP(r), oauthLimits); msg != "" {
		fail(w, http.StatusTooManyRequests, "muitas tentativas de entrar; espere um pouco")
		return false
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, maxOAuthBody)).Decode(in) != nil {
		fail(w, http.StatusBadRequest, "pedido inválido")
		return false
	}
	return true
}

func (h *handler) oauthRelay(w http.ResponseWriter, r *http.Request, form url.Values) {
	form.Set("client_id", h.cfg.GoogleClientID)
	form.Set("client_secret", h.cfg.GoogleClientSecret)
	tokenURL := h.cfg.GoogleTokenURL
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		fail(w, http.StatusInternalServerError, "erro interno")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.cfg.Client.Do(req)
	if err != nil {
		fail(w, http.StatusBadGateway, "o Google não respondeu; tente de novo")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if siteMode(r) && resp.StatusCode == http.StatusOK {
		// Site: o token de renovação vai para o cookie e sai da resposta.
		var tok map[string]any
		if json.Unmarshal(body, &tok) == nil {
			if rt, _ := tok["refresh_token"].(string); rt != "" {
				setRT(w, rt, rtMaxAge)
			}
			delete(tok, "refresh_token")
			tok["cookie"] = true
			body, _ = json.Marshal(tok)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// oauthLogout (site): apaga o cookie e revoga o acesso no Google.
func (h *handler) oauthLogout(w http.ResponseWriter, r *http.Request) {
	if msg := h.oauthLimiter.allow("", clientIP(r), oauthLimits); msg != "" {
		fail(w, http.StatusTooManyRequests, "muitas tentativas; espere um pouco")
		return
	}
	if c, err := r.Cookie(rtCookie); err == nil && c.Value != "" {
		revokeURL := h.cfg.GoogleRevokeURL
		if revokeURL == "" {
			revokeURL = "https://oauth2.googleapis.com/revoke"
		}
		if req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, revokeURL, strings.NewReader(url.Values{"token": {c.Value}}.Encode())); err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if resp, err := h.cfg.Client.Do(req); err == nil {
				resp.Body.Close() // se o Google não responder, o cookie sai do mesmo jeito
			}
		}
	}
	setRT(w, "", -1)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(`{"ok":true}`))
}
