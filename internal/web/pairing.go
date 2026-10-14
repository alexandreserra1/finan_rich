package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/alexandreserra1/finan_rich/internal/auth"
)

// Pareamento do app do celular (Capacitor).
//
// No computador, o usuário gera um código curto (Ajustes → App do celular).
// O app troca esse código, uma única vez, por um token próprio do aparelho
// (POST /api/parear) e passa a sincronizar com "Authorization: Bearer <token>".
// O código vale poucos minutos; o token vale até o aparelho ser removido.

const (
	pairingTTL      = 10 * time.Minute
	pairingAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // sem 0/O e 1/I, fáceis de confundir
	pairingLen      = 8
	appScheme       = "finanrich" // links finanrich://parear abrem o app
	appDownloadURL  = "https://github.com/alexandreserra1/finan_rich/releases/latest"
)

// pairing guarda o código ativo (só um por vez, só em memória).
type pairing struct {
	mu      sync.Mutex
	code    string
	expires time.Time
}

func newPairingCode() (string, error) {
	b := make([]byte, pairingLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = pairingAlphabet[int(b[i])%len(pairingAlphabet)] // 256 é múltiplo de 32: sem viés
	}
	return string(b), nil
}

// normalizeCode aceita o código digitado com espaços, hífen ou minúsculas.
func normalizeCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatCode mostra o código em dois blocos (ABCD-EFGH), mais fácil de digitar.
func formatCode(code string) string {
	if len(code) != pairingLen {
		return code
	}
	return code[:4] + "-" + code[4:]
}

func (p *pairing) start(now time.Time) (string, error) {
	code, err := newPairingCode()
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.code, p.expires = code, now.Add(pairingTTL)
	return code, nil
}

func (p *pairing) active(now time.Time) (string, time.Time, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.code == "" || now.After(p.expires) {
		return "", time.Time{}, false
	}
	return p.code, p.expires, true
}

// consume confere o código e, se certo, o invalida (uso único).
func (p *pairing) consume(code string, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.code == "" || now.After(p.expires) || subtle.ConstantTimeCompare([]byte(p.code), []byte(normalizeCode(code))) != 1 {
		return false
	}
	p.code = ""
	return true
}

func (p *pairing) cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.code = ""
}

// pairURL é o endereço do QR code: abre a página /parear no celular, que leva
// ao app (ou explica como instalá-lo).
func pairURL(base, code string) string {
	return base + "/parear?" + url.Values{"codigo": {code}}.Encode()
}

// appLink abre o app já com o endereço do computador e o código preenchidos.
func appLink(base, code string) string {
	return appScheme + "://parear?" + url.Values{"url": {base}, "codigo": {code}}.Encode()
}

// requestBase é o endereço pelo qual o aparelho chegou até aqui.
func requestBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// startPairing gera um código novo (Ajustes → App do celular).
func (s *Server) startPairing(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("acao") == "cancelar" {
		s.pairing.cancel()
		redirect(w, r, "/configuracoes#app", "", nil)
		return
	}
	_, err := s.pairing.start(time.Now())
	redirect(w, r, "/configuracoes#app", "", err)
}

// pairPage é aberta pelo QR code no celular.
func (s *Server) pairPage(w http.ResponseWriter, r *http.Request) {
	code := normalizeCode(r.URL.Query().Get("codigo"))
	base := requestBase(r)
	s.render(w, r, "pair", map[string]any{
		// template.URL: o html/template bloquearia o esquema finanrich:// (montado aqui, é seguro).
		"Code": formatCode(code), "Base": base, "AppLink": template.URL(appLink(base, code)), "Download": appDownloadURL,
		"IOS": isIOS(r),
	})
}

// isIOS: iPhone ou iPad. Lá o app ainda não está na App Store, então a página
// de pareamento ensina a usar pelo Safari (Adicionar à Tela de Início).
func isIOS(r *http.Request) bool {
	ua := r.UserAgent()
	// O iPad se apresenta como Mac ("Macintosh"), mas com "Mobile/".
	return strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") ||
		strings.Contains(ua, "Macintosh") && strings.Contains(ua, "Mobile/")
}

type pairRequest struct {
	Code string `json:"code"`
	Name string `json:"name"` // ex.: "Android", "iPhone"
}

type pairResponse struct {
	Token  string `json:"token"`
	Device string `json:"device"`
}

// apiPair troca o código de pareamento por um token do aparelho.
func (s *Server) apiPair(w http.ResponseWriter, r *http.Request) {
	key := clientKey(r)
	if !s.limiter.Allowed(key) {
		writeJSONError(w, http.StatusTooManyRequests, "muitas tentativas erradas; espere alguns minutos")
		return
	}
	var req pairRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "pedido de pareamento inválido")
		return
	}
	if !s.pairing.consume(req.Code, time.Now()) {
		s.limiter.Fail(key)
		writeJSONError(w, http.StatusForbidden, "código errado ou vencido; gere outro no computador (Ajustes → App do celular)")
		return
	}
	s.limiter.Reset(key)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Celular"
	}
	if len([]rune(name)) > 60 {
		name = string([]rune(name)[:60])
	}
	token, err := auth.NewToken()
	if err == nil {
		_, err = s.store.CreateDevice(r.Context(), name, auth.HashToken(token))
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pairResponse{Token: token, Device: name})
}

// bearerToken lê o token do app ("Authorization: Bearer <token>").
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func (s *Server) validDevice(ctx context.Context, token string) bool {
	_, ok, err := s.store.DeviceByToken(ctx, auth.HashToken(token), time.Now())
	return err == nil && ok
}

// removeDevice desconecta um app pareado.
func (s *Server) removeDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		redirect(w, r, "/configuracoes#app", "", errors.New("aparelho inválido"))
		return
	}
	err = s.store.DeleteDevice(r.Context(), id)
	redirect(w, r, "/configuracoes#app", "Aparelho desconectado. Para usar o app nele de novo, pareie outra vez.", err)
}

// pairingView monta os dados da seção "App do celular" dos Ajustes.
func (s *Server) pairingView(ctx context.Context, base string) (map[string]any, error) {
	devices, err := s.store.Devices(ctx)
	if err != nil {
		return nil, err
	}
	view := map[string]any{"Devices": devices, "Download": appDownloadURL, "Base": base}
	if code, expires, ok := s.pairing.active(time.Now()); ok && base != "" {
		link := pairURL(base, code)
		view["Code"] = formatCode(code)
		view["Expires"] = expires.Local().Format("15:04")
		view["QR"] = qrDataURI(link)
	}
	return view, nil
}

// apiCORS libera /api/ para o app do celular, que roda em outra origem
// (capacitor://localhost). É seguro porque essas rotas, vindas de outra
// origem, só valem com o token do aparelho ou o código de pareamento: o
// navegador não envia esses cabeçalhos sozinho, como faria com um cookie.
func apiCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
