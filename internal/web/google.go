package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/auth"
	"github.com/alexandreserra1/finan_rich/internal/drivesync"
	"github.com/alexandreserra1/finan_rich/internal/gdrive"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

// Entrar com Google no programa do computador (docs/GOOGLE.md): os dados vão também
// para a pasta de dados do app no Google Drive da pessoa, como no site. O login abre
// no navegador deste computador, e a página /oauth do site repassa a volta para
// 127.0.0.1 nesta porta (state "pc.<porta>.*"). O código vem com PKCE, e a troca
// passa pelo intermediário, que tem o client secret. O token de renovação fica na
// tabela settings (o backup do banco não leva; veja store.BackupTo); o de acesso,
// só na memória.

const (
	googlePrefix  = "google_"
	googleScope   = "openid email https://www.googleapis.com/auth/drive.appdata"
	googleEvery   = 2 * time.Minute  // como o site: a cada 2 min e logo depois de cada alteração
	googlePending = 15 * time.Minute // prazo para voltar do Google
)

var (
	errGoogleOff     = errors.New("entre com Google primeiro")
	errGoogleRevoked = errors.New("a conexão com o Google acabou (o acesso foi retirado na conta Google ou venceu). Entre com Google de novo; os dados continuam aqui")
	errGoogleLost    = errors.New("o login expirou ou não começou neste computador; tente de novo")
	portOK           = regexp.MustCompile(`^\d{2,5}$`) // o mesmo que a página /oauth do site aceita
)

// googleSync guarda o estado do Google no programa. Os endereços são os de verdade;
// os testes trocam por falsos.
type googleSync struct {
	server, drive, auth, revoke string
	client                      *http.Client
	soon                        chan struct{} // pede uma rodada logo (depois do /api/sync)

	mu sync.Mutex // uma rodada por vez; sair espera a rodada em andamento

	tmu       sync.Mutex // o resto: token, login pendente e o resultado da última rodada
	access    string
	exp       time.Time
	state     string
	verifier  string
	pendingAt time.Time
	lastErr   string
	relogin   bool
}

func newGoogleSync() *googleSync {
	return &googleSync{
		server: llm.FreeProxyURL, drive: gdrive.DefaultBase,
		auth: "https://accounts.google.com/o/oauth2/v2/auth", revoke: "https://oauth2.googleapis.com/revoke",
		client: &http.Client{Timeout: time.Minute}, soon: make(chan struct{}, 1),
	}
}

func (g *googleSync) setAccess(token string, seconds int) {
	if seconds <= 0 {
		seconds = 3600
	}
	g.tmu.Lock()
	g.access, g.exp = token, time.Now().Add(time.Duration(seconds)*time.Second)
	g.tmu.Unlock()
}

// take confere o state da volta (uma vez só, dentro do prazo) e devolve o verificador.
func (g *googleSync) take(state string) (string, bool) {
	g.tmu.Lock()
	defer g.tmu.Unlock()
	if g.state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(g.state)) != 1 || time.Since(g.pendingAt) > googlePending {
		return "", false
	}
	verifier := g.verifier
	g.state, g.verifier = "", ""
	return verifier, true
}

func (g *googleSync) setResult(err error) {
	g.tmu.Lock()
	defer g.tmu.Unlock()
	g.lastErr, g.relogin = "", false
	if err != nil && !errors.Is(err, errGoogleOff) {
		var de *gdrive.Error
		g.lastErr, g.relogin = err.Error(), errors.As(err, &de) && de.NeedsLogin()
	}
}

func (s *Server) googleSoon() {
	select {
	case s.google.soon <- struct{}{}:
	default: // já tem uma pedida
	}
}

// WatchGoogle sincroniza com o Drive até ctx acabar: a cada 2 minutos e logo depois
// que um aparelho manda alterações. Sem conta ligada, não faz nada.
func (s *Server) WatchGoogle(ctx context.Context) {
	tick := time.NewTicker(googleEvery)
	defer tick.Stop()
	for {
		if err := s.syncGoogle(ctx); err != nil && ctx.Err() == nil && !errors.Is(err, errGoogleOff) {
			log.Printf("Google Drive: %v", err) // mensagens feitas para a pessoa, sem token
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-s.google.soon:
		}
	}
}

// googleSignIn abre o login do Google. É um link (GET), não um formulário: um POST que
// redireciona para o Google bateria no form-action 'self' da CSP.
func (s *Server) googleSignIn(w http.ResponseWriter, r *http.Request) {
	const back = "/configuracoes#google"
	if s.envPasswordHash != "" {
		redirect(w, r, back, "", errors.New("no modo servidor a volta do login não chega a este programa; entre com Google pelo site ou pelo app"))
		return
	}
	if !portOK.MatchString(s.port) {
		redirect(w, r, back, "", errors.New("a volta do login precisa da porta do programa em números (ex.: 8080)"))
		return
	}
	var cfg struct {
		ClientID string `json:"clientId"`
	}
	status, err := s.googleCall(r.Context(), http.MethodGet, "/oauth/config", nil, &cfg)
	if err == nil && (status != http.StatusOK || cfg.ClientID == "") {
		err = errors.New("o login com Google não está disponível agora; tente mais tarde")
	}
	verifier, err2 := auth.NewToken()
	nonce, err3 := auth.NewToken()
	if err = errors.Join(err, err2, err3); err != nil {
		redirect(w, r, back, "", err)
		return
	}
	g := s.google
	state := "pc." + s.port + "." + nonce
	g.tmu.Lock()
	g.state, g.verifier, g.pendingAt = state, verifier, time.Now()
	g.tmu.Unlock()
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"client_id": {cfg.ClientID}, "redirect_uri": {g.server + "/oauth"}, "response_type": {"code"}, "scope": {googleScope},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}, "state": {state},
		// consent garante o token de renovação; select_account faz escolher a conta.
		"access_type": {"offline"}, "prompt": {"select_account consent"},
	}
	http.Redirect(w, r, g.auth+"?"+q.Encode(), http.StatusSeeOther)
}

// googleTokens é a resposta do intermediário (a do Google, repassada).
type googleTokens struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	IDToken      string          `json:"id_token"`
	ExpiresIn    int             `json:"expires_in"`
	Err          json.RawMessage `json:"error"` // texto do Google ("invalid_grant") ou {message} do intermediário
	Description  string          `json:"error_description"`
}

func (t googleTokens) code() string {
	var s string
	json.Unmarshal(t.Err, &s)
	return s
}

func (t googleTokens) message() string {
	var obj struct{ Message string }
	json.Unmarshal(t.Err, &obj)
	for _, m := range []string{t.Description, obj.Message, t.code()} {
		if m != "" {
			return m
		}
	}
	return "tente de novo"
}

// googleReturn recebe a volta do login (repassada pela página /oauth do site).
func (s *Server) googleReturn(w http.ResponseWriter, r *http.Request) {
	const back = "/configuracoes#google"
	q := r.URL.Query()
	verifier, ok := s.google.take(q.Get("state"))
	if !ok || q.Get("code") == "" {
		redirect(w, r, back, "", errGoogleLost)
		return
	}
	var tok googleTokens
	status, err := s.googleCall(r.Context(), http.MethodPost, "/oauth/token", map[string]string{"code": q.Get("code"), "verifier": verifier, "redirectUri": s.google.server + "/oauth"}, &tok)
	switch {
	case err != nil:
	case status != http.StatusOK || tok.AccessToken == "":
		err = fmt.Errorf("o Google recusou o login (%s)", tok.message())
	case tok.RefreshToken == "":
		err = errors.New("o Google não mandou a permissão para manter você conectado; tente de novo")
	}
	if err == nil {
		// Conta nova (ou a mesma de novo): sem versão nem revisão guardadas, a primeira rodada
		// mescla tudo. Espera a rodada em andamento, que senão gravaria a versão da conta anterior.
		s.google.mu.Lock()
		err = s.store.SaveSettings(r.Context(), googlePrefix, map[string]string{"refresh": tok.RefreshToken, "email": jwtEmail(tok.IDToken), "version": "", "rev": "", "synced": ""})
		if err == nil {
			s.google.setAccess(tok.AccessToken, tok.ExpiresIn)
		}
		s.google.mu.Unlock()
	}
	if err != nil {
		redirect(w, r, back, "", err)
		return
	}
	s.google.setResult(nil)
	s.logAccess(r, "Conta Google ligada")
	s.googleSoon()
	redirect(w, r, back, "Conectado ao Google. Os dados vão para o seu Drive em instantes.", nil)
}

// jwtEmail lê o e-mail do id_token só para mostrar qual conta está ligada. O token
// veio do Google pelo intermediário, numa conexão https, e não autoriza nada aqui.
func jwtEmail(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	json.Unmarshal(b, &claims)
	return claims.Email
}

func (s *Server) googleSyncNow(w http.ResponseWriter, r *http.Request) {
	redirect(w, r, "/configuracoes#google", "Sincronizado com o Google Drive.", s.syncGoogle(r.Context()))
}

// googleSignOut revoga o acesso no Google e apaga o token daqui; os dados ficam.
func (s *Server) googleSignOut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g := s.google
	g.mu.Lock() // espera a rodada em andamento
	defer g.mu.Unlock()
	acct, err := s.store.Settings(ctx, googlePrefix)
	if err == nil {
		if rt := acct["refresh"]; rt != "" {
			s.googleRevoke(ctx, rt) // se o Google não responder, o token sai daqui do mesmo jeito
		}
		err = s.googleForget(ctx)
	}
	if err == nil {
		g.setResult(nil)
		s.logAccess(r, "Conta Google desligada")
	}
	redirect(w, r, "/configuracoes#google", "Conta Google desligada neste computador. Os dados continuam aqui e no seu Drive.", err)
}

func (s *Server) googleForget(ctx context.Context) error {
	g := s.google
	g.tmu.Lock()
	g.access = ""
	g.tmu.Unlock()
	return s.store.SaveSettings(ctx, googlePrefix, map[string]string{"refresh": "", "email": "", "version": "", "rev": "", "synced": ""})
}

func (s *Server) googleRevoke(ctx context.Context, token string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.google.revoke, strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := s.google.client.Do(req); err == nil {
		resp.Body.Close()
	}
}

// googleCall fala com o intermediário (o mesmo do site e do app): devolve o status e
// lê o JSON da resposta em out. Só dá erro quando não deu para falar com ele.
func (s *Server) googleCall(ctx context.Context, method, path string, in, out any) (int, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.google.server+path, body)
	if err != nil {
		return 0, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.google.client.Do(req)
	if err != nil {
		return 0, errors.New("não foi possível falar com o Google agora; confira a internet e tente de novo")
	}
	defer resp.Body.Close()
	json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(out)
	return resp.StatusCode, nil
}

// googleToken devolve um token de acesso válido, renovando quando falta menos de 1 min.
func (s *Server) googleToken(ctx context.Context) (string, error) {
	g := s.google
	g.tmu.Lock()
	if g.access != "" && time.Until(g.exp) > time.Minute {
		token := g.access
		g.tmu.Unlock()
		return token, nil
	}
	g.tmu.Unlock()
	acct, err := s.store.Settings(ctx, googlePrefix)
	if err != nil {
		return "", err
	}
	if acct["refresh"] == "" {
		return "", errGoogleOff
	}
	var tok googleTokens
	status, err := s.googleCall(ctx, http.MethodPost, "/oauth/refresh", map[string]string{"refreshToken": acct["refresh"]}, &tok)
	if err != nil {
		return "", err
	}
	if status == http.StatusUnauthorized || (status == http.StatusBadRequest && tok.code() == "invalid_grant") {
		// A pessoa tirou o acesso na conta Google (ou ele venceu): a conta sai daqui; os dados ficam.
		if err := s.googleForget(ctx); err != nil {
			return "", err
		}
		return "", errGoogleRevoked
	}
	if status != http.StatusOK || tok.AccessToken == "" {
		return "", fmt.Errorf("não foi possível renovar o acesso ao Google agora (%s)", tok.message())
	}
	g.setAccess(tok.AccessToken, tok.ExpiresIn)
	return tok.AccessToken, nil
}

// syncGoogle faz uma rodada com o Drive. Sem conta ligada, devolve errGoogleOff.
func (s *Server) syncGoogle(ctx context.Context) error {
	g := s.google
	g.mu.Lock()
	defer g.mu.Unlock()
	acct, err := s.store.Settings(ctx, googlePrefix)
	if err != nil {
		return err
	}
	if acct["refresh"] == "" {
		return errGoogleOff
	}
	saved, err := s.googleRound(ctx, acct)
	var de *gdrive.Error
	if errors.As(err, &de) && de.Status == http.StatusUnauthorized {
		// O Drive recusou o token de acesso antes da hora: renova e tenta mais uma vez.
		g.tmu.Lock()
		g.access = ""
		g.tmu.Unlock()
		saved, err = s.googleRound(ctx, acct)
	}
	if err == nil {
		err = s.store.SaveSettings(ctx, googlePrefix, saved)
	}
	g.setResult(err)
	return err
}

// googleRound é a rodada de js/gdrive.js (syncDrive): baixa o arquivo se ele mudou,
// aplica o que for mais novo e regrava o arquivo inteiro se este computador tiver algo
// que falta lá. Devolve o que guardar para a próxima rodada.
func (s *Server) googleRound(ctx context.Context, acct map[string]string) (map[string]string, error) {
	c := &gdrive.Client{HTTP: s.google.client, Base: s.google.drive, Token: s.googleToken}
	f, err := c.Find(ctx)
	if err != nil {
		return nil, err
	}
	var remote []drivesync.Item
	downloaded := f != nil && f.Version != acct["version"]
	if downloaded {
		file, err := c.Download(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		remote = file.Items
		changes := make([]store.Change, len(remote))
		for i, it := range remote {
			changes[i] = store.Change(it)
		}
		if _, err := s.store.ApplyChanges(ctx, changes); err != nil {
			if errors.Is(err, store.ErrInvalidChange) {
				return nil, fmt.Errorf("o arquivo do Drive tem um registro que este computador não aceita (%v); atualize a Rich", err)
			}
			return nil, err
		}
	}
	// A revisão é lida antes dos registros: o que chegar no meio fica para a próxima rodada.
	rev, err := s.store.CurrentRev(ctx)
	if err != nil {
		return nil, err
	}
	last, _ := strconv.ParseInt(acct["rev"], 10, 64)
	now := time.Now().Format(time.RFC3339)
	if f != nil && !downloaded && rev <= last { // nada mudou lá nem aqui: nem lê os registros
		return map[string]string{"version": f.Version, "rev": strconv.FormatInt(rev, 10), "synced": now}, nil
	}
	local, err := s.allItems(ctx)
	if err != nil {
		return nil, err
	}
	upload := false
	switch {
	case downloaded:
		upload = drivesync.NeedsUpload(local, remote)
	case f == nil:
		upload = len(local) > 0
	default: // arquivo igual ao da última rodada: regrava se o banco mudou desde então
		upload = rev > last
	}
	version := ""
	if f != nil {
		version = f.Version
	}
	if upload {
		id := ""
		if f != nil {
			id = f.ID
		}
		written, err := c.Write(ctx, id, drivesync.File{Version: 1, Items: local})
		if err != nil {
			return nil, err
		}
		version = written.Version
	}
	return map[string]string{"version": version, "rev": strconv.FormatInt(rev, 10), "synced": now}, nil
}

// allItems lê todos os registros do banco, inclusive os apagados.
func (s *Server) allItems(ctx context.Context) ([]drivesync.Item, error) {
	var items []drivesync.Item
	for cursor := int64(0); ; {
		changes, next, err := s.store.ChangesSince(ctx, cursor, syncPageSize)
		if err != nil {
			return nil, err
		}
		for _, c := range changes {
			items = append(items, drivesync.Item(c))
		}
		if len(changes) < syncPageSize {
			return items, nil
		}
		cursor = next
	}
}

// googleView é a seção "Conta Google" dos Ajustes.
func (s *Server) googleView(ctx context.Context, r *http.Request) map[string]any {
	acct, _ := s.store.Settings(ctx, googlePrefix)
	g := s.google
	g.tmu.Lock()
	lastErr, relogin := g.lastErr, g.relogin
	g.tmu.Unlock()
	v := map[string]any{
		"Linked": acct["refresh"] != "", "Email": acct["email"], "Error": lastErr, "Relogin": relogin,
		"CanSignIn": isLoopback(r) && s.envPasswordHash == "",
	}
	if t, err := time.Parse(time.RFC3339, acct["synced"]); err == nil {
		v["Synced"] = t.Local().Format("02/01 às 15:04")
	}
	return v
}
