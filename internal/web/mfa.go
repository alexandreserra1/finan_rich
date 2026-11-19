package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/auth"
)

// Verificação em duas etapas no login pela rede: depois da senha certa, o código de 6
// dígitos do app autenticador (ou um código de recuperação). Neste computador não muda.

const (
	keyTOTP         = "totp_secret"   // ligada: o segredo compartilhado com o app
	keyTOTPPending  = "totp_pending"  // ligando: o segredo do QR, até a pessoa confirmar um código
	keyTOTPLast     = "totp_last"     // o último passo de 30 s usado (o mesmo código não vale duas vezes)
	keyTOTPRecovery = "totp_recovery" // hashes dos códigos de recuperação que ainda valem

	// codeKey: o código tem um limite só, para todos os endereços. Quem chega ao código já
	// acertou a senha; trocando de IP (fácil no Wi-Fi) não ganharia tentativas novas.
	codeKey = "codigo"

	mfaCookie     = "rich_mfa"
	mfaWait       = 5 * time.Minute // tempo para digitar o código depois de acertar a senha
	maxMFAPending = 64              // logins pela metade guardados ao mesmo tempo
)

// mfaPending guarda, só na memória, quem acertou a senha e ainda vai digitar o código.
// Reiniciar o programa apaga: a pessoa digita a senha de novo.
type mfaPending struct {
	mu sync.Mutex
	m  map[string]time.Time // hash do cookie → até quando vale
}

func (p *mfaPending) add(now time.Time) (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = map[string]time.Time{}
	}
	for k, until := range p.m {
		if now.After(until) {
			delete(p.m, k)
		}
	}
	if len(p.m) >= maxMFAPending {
		return "", errors.New("muitos logins esperando o código; tente de novo em alguns minutos")
	}
	p.m[auth.HashToken(token)] = now.Add(mfaWait)
	return token, nil
}

func (p *mfaPending) valid(token string, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	until, ok := p.m[auth.HashToken(token)]
	return ok && now.Before(until)
}

func (p *mfaPending) done(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, auth.HashToken(token))
}

// mfaOn diz se a verificação em duas etapas está ligada.
func (s *Server) mfaOn(ctx context.Context) bool {
	set, err := s.store.Settings(ctx, accessPrefix)
	return err == nil && set[keyTOTP] != ""
}

// secondFactor confere o código de 6 dígitos ou um código de recuperação (que é gasto).
// Um de cada vez: dois pedidos com o mesmo código não passam juntos.
func (s *Server) secondFactor(r *http.Request, code string) bool {
	s.mfaMu.Lock()
	defer s.mfaMu.Unlock()
	ctx := r.Context()
	set, err := s.store.Settings(ctx, accessPrefix)
	if err != nil || set[keyTOTP] == "" {
		return false
	}
	last, _ := strconv.ParseInt(set[keyTOTPLast], 10, 64)
	if step, ok := auth.CheckTOTP(set[keyTOTP], code, time.Now(), last); ok {
		return s.store.SaveSettings(ctx, accessPrefix, map[string]string{keyTOTPLast: strconv.FormatInt(step, 10)}) == nil
	}
	if rest, ok := auth.UseRecoveryCode(code, strings.Fields(set[keyTOTPRecovery])); ok {
		if s.store.SaveSettings(ctx, accessPrefix, map[string]string{keyTOTPRecovery: strings.Join(rest, " ")}) != nil {
			return false
		}
		s.logAccess(r, "Código de recuperação usado")
		return true
	}
	return false
}

// reauth: mudar a segurança pela rede pede a senha atual e, se ligado, o código. Quem
// pegar uma sessão aberta não troca a senha nem desliga o código ou o acesso. Neste
// computador não pede (sem FINAN_PASSWORD): quem perdeu o celular resolve aqui.
func (s *Server) reauth(r *http.Request) error {
	if s.envPasswordHash == "" && isLoopback(r) {
		return nil
	}
	key := clientKey(r)
	if !s.limiter.Take(key) {
		return errors.New("muitas tentativas erradas; espere alguns minutos")
	}
	hash, err := s.passwordHash(r.Context())
	if err != nil {
		return err
	}
	if ok, _ := auth.CheckPassword(r.FormValue("senha_atual"), hash); !ok {
		s.logAccess(r, "Senha atual errada ao mudar a segurança")
		return errors.New("senha atual incorreta")
	}
	if s.mfaOn(r.Context()) {
		if !s.codeLimiter.Take(codeKey) {
			return errors.New("muitas tentativas erradas do código; espere alguns minutos")
		}
		if !s.secondFactor(r, r.FormValue("codigo")) {
			s.logAccess(r, "Código de verificação errado")
			return errors.New("código de 6 dígitos incorreto")
		}
		s.codeLimiter.Reset(codeKey)
	}
	s.limiter.Reset(key)
	return nil
}

// askCode guarda o login pela metade e manda para a tela do código.
func (s *Server) askCode(w http.ResponseWriter, r *http.Request, back string) error {
	token, err := s.mfa.add(time.Now())
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: mfaCookie, Value: token, Path: "/entrar", MaxAge: int(mfaWait.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	http.Redirect(w, r, "/entrar/codigo?"+url.Values{"volta": {back}}.Encode(), http.StatusSeeOther)
	return nil
}

func (s *Server) pendingLogin(r *http.Request) (string, bool) {
	c, err := r.Cookie(mfaCookie)
	if err != nil {
		return "", false
	}
	return c.Value, s.mfa.valid(c.Value, time.Now())
}

func timedOut(w http.ResponseWriter, r *http.Request, back string) {
	http.Redirect(w, r, "/entrar?"+url.Values{"volta": {back}, "erro": {"O tempo para digitar o código acabou. Digite a senha de novo."}}.Encode(), http.StatusSeeOther)
}

func (s *Server) loginCodePage(w http.ResponseWriter, r *http.Request) {
	back := safeReturn(r.URL.Query().Get("volta"))
	if _, ok := s.pendingLogin(r); !ok {
		timedOut(w, r, back)
		return
	}
	s.render(w, r, "login", map[string]any{"Return": back, "Code": true})
}

func (s *Server) loginCode(w http.ResponseWriter, r *http.Request) {
	back := safeReturn(r.FormValue("volta"))
	token, ok := s.pendingLogin(r)
	if !ok {
		timedOut(w, r, back)
		return
	}
	fail := func(msg string) {
		http.Redirect(w, r, "/entrar/codigo?"+url.Values{"volta": {back}, "erro": {msg}}.Encode(), http.StatusSeeOther)
	}
	if !s.codeLimiter.Take(codeKey) {
		fail("Muitas tentativas erradas. Espere alguns minutos e tente de novo.")
		return
	}
	if !s.secondFactor(r, r.FormValue("codigo")) {
		s.logAccess(r, "Código de verificação errado")
		fail("Código incorreto. Digite o código que aparece agora no app.")
		return
	}
	s.mfa.done(token)
	http.SetCookie(w, &http.Cookie{Name: mfaCookie, Value: "", Path: "/entrar", MaxAge: -1})
	s.codeLimiter.Reset(codeKey)
	if err := s.startSession(r.Context(), w, r); err != nil {
		serverError(w, err)
		return
	}
	s.logAccess(r, "Entrou pela rede")
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// startMFA guarda um segredo novo, à espera do primeiro código (o QR aparece nos Ajustes).
func (s *Server) startMFA(ctx context.Context) error {
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		return err
	}
	return s.store.SaveSettings(ctx, accessPrefix, map[string]string{keyTOTPPending: secret})
}

// saveMFA trata as ações da verificação em duas etapas nos Ajustes.
func (s *Server) saveMFA(w http.ResponseWriter, r *http.Request, action string) {
	ctx := r.Context()
	const back = "/configuracoes#mfa"
	switch action {
	case "mfa-iniciar":
		err := s.reauth(r)
		if err == nil {
			err = s.startMFA(ctx)
		}
		redirect(w, r, back, "Leia o QR code com o app autenticador e digite o código que ele mostra.", err)
	case "mfa-cancelar":
		err := s.store.SaveSettings(ctx, accessPrefix, map[string]string{keyTOTPPending: ""})
		redirect(w, r, back, "A verificação em duas etapas não foi ligada.", err)
	case "mfa-confirmar":
		set, err := s.store.Settings(ctx, accessPrefix)
		if err != nil || set[keyTOTPPending] == "" {
			redirect(w, r, back, "", errors.New("comece de novo: toque em \"Ligar o código de 6 dígitos\""))
			return
		}
		key := clientKey(r)
		if !s.limiter.Take(key) {
			redirect(w, r, back, "", errors.New("muitas tentativas erradas; espere alguns minutos"))
			return
		}
		step, ok := auth.CheckTOTP(set[keyTOTPPending], r.FormValue("codigo"), time.Now(), 0)
		if !ok {
			redirect(w, r, back, "", errors.New("código incorreto: confira se o relógio do celular está certo e digite o código que aparece agora"))
			return
		}
		s.limiter.Reset(key)
		codes, hashes, err := auth.NewRecoveryCodes(8)
		if err == nil {
			err = s.store.SaveSettings(ctx, accessPrefix, map[string]string{
				keyTOTP: set[keyTOTPPending], keyTOTPPending: "", keyTOTPLast: strconv.FormatInt(step, 10), keyTOTPRecovery: strings.Join(hashes, " "),
			})
		}
		if err != nil {
			redirect(w, r, back, "", err)
			return
		}
		// As sessões abertas pela rede passaram só pela senha: entram de novo, já com o código.
		s.store.DeleteAllSessions(ctx)
		if !isLoopback(r) {
			s.startSession(ctx, w, r)
		}
		s.logAccess(r, "Verificação em duas etapas ligada")
		// Os códigos de recuperação aparecem só nesta resposta: não vão na URL, no log nem no cache.
		w.Header().Set("Cache-Control", "no-store")
		s.settingsPage(w, r, map[string]any{"RecoveryCodes": codes})
	case "mfa-desligar":
		err := s.reauth(r)
		if err == nil {
			err = s.store.SaveSettings(ctx, accessPrefix, map[string]string{keyTOTP: "", keyTOTPPending: "", keyTOTPLast: "", keyTOTPRecovery: ""})
		}
		if err == nil {
			s.logAccess(r, "Verificação em duas etapas desligada")
		}
		redirect(w, r, back, "Verificação em duas etapas desligada: o login pela rede pede só a senha.", err)
	}
}

// mfaView monta a parte da verificação em duas etapas dos Ajustes.
func (s *Server) mfaView(ctx context.Context) (map[string]any, error) {
	set, err := s.store.Settings(ctx, accessPrefix)
	if err != nil {
		return nil, err
	}
	v := map[string]any{"On": set[keyTOTP] != "", "Left": len(strings.Fields(set[keyTOTPRecovery]))}
	if p := set[keyTOTPPending]; p != "" && set[keyTOTP] == "" {
		var groups []string
		for i := 0; i < len(p); i += 4 {
			groups = append(groups, p[i:min(i+4, len(p))])
		}
		v["Secret"], v["QR"] = strings.Join(groups, " "), qrDataURI(auth.TOTPURI(p, accountName()))
	}
	return v, nil
}

// accountName é o nome que aparece no app autenticador ("Rich:<nome do computador>").
func accountName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return strings.TrimSuffix(h, ".local")
	}
	return "computador"
}
