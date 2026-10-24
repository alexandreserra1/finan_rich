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
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, io.LimitReader(resp.Body, 64<<10))
}
