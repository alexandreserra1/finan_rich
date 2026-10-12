package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/auth"
)

// Configurações de acesso guardadas no banco.
const (
	accessPrefix    = "access_"
	keyPasswordHash = "password_hash"
	keyLAN          = "lan"

	sessionCookie   = "finan_sessao"
	sessionDuration = 30 * 24 * time.Hour
)

// publicPaths não exigem login (tela de login, arquivos estáticos e do PWA).
func isPublicPath(path string) bool {
	switch path {
	case "/entrar", "/healthz", "/manifest.webmanifest", "/sw.js", "/offline":
		return true
	}
	return strings.HasPrefix(path, "/static/")
}

// isLoopback diz se a requisição veio do próprio computador.
func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// passwordHash devolve o hash da senha de acesso, se houver.
func (s *Server) passwordHash(ctx context.Context) (string, error) {
	if s.envPasswordHash != "" {
		return s.envPasswordHash, nil
	}
	saved, err := s.store.Settings(ctx, accessPrefix)
	return saved[keyPasswordHash], err
}

// LANEnabled diz se o usuário liberou o acesso pelo celular (outros aparelhos da rede).
func (s *Server) LANEnabled(ctx context.Context) bool {
	saved, err := s.store.Settings(ctx, accessPrefix)
	return err == nil && saved[keyLAN] == "1"
}

// requireAuth exige login de quem acessa pela rede. Quem usa no próprio
// computador entra direto, a não ser que a senha venha de FINAN_PASSWORD
// (modo servidor, onde um proxy reverso faria tudo parecer local).
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) || (s.envPasswordHash == "" && isLoopback(r)) {
			next.ServeHTTP(w, r)
			return
		}
		hash, err := s.passwordHash(r.Context())
		if err != nil {
			serverError(w, err)
			return
		}
		if hash == "" || s.validSession(r) {
			// Sem senha só se chega aqui pela rede quando o app foi exposto de
			// propósito (flag -addr); o aviso aparece ao iniciar.
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			http.Redirect(w, r, "/entrar?volta="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		http.Error(w, "faça login para continuar", http.StatusUnauthorized)
	})
}

func (s *Server) validSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	ok, err := s.store.SessionValid(r.Context(), auth.HashToken(c.Value), time.Now())
	return err == nil && ok
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// safeReturn evita redirecionar para outro site depois do login.
func safeReturn(path string) string {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.HasPrefix(path, "/\\") {
		return "/"
	}
	return path
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login", map[string]any{"Return": safeReturn(r.URL.Query().Get("volta"))})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := safeReturn(r.FormValue("volta"))
	fail := func(msg string) {
		http.Redirect(w, r, "/entrar?"+url.Values{"volta": {back}, "erro": {msg}}.Encode(), http.StatusSeeOther)
	}
	key := clientKey(r)
	if !s.limiter.Allowed(key) {
		fail("Muitas tentativas erradas. Espere alguns minutos e tente de novo.")
		return
	}
	hash, err := s.passwordHash(ctx)
	if err != nil || hash == "" {
		fail("O acesso por senha não está configurado.")
		return
	}
	ok, err := auth.CheckPassword(r.FormValue("senha"), hash)
	if err != nil || !ok {
		s.limiter.Fail(key)
		fail("Senha incorreta.")
		return
	}
	s.limiter.Reset(key)
	if err := s.startSession(ctx, w, r); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	token, err := auth.NewToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionDuration)
	s.store.DeleteExpiredSessions(ctx, time.Now())
	if err := s.store.CreateSession(ctx, auth.HashToken(token), expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.store.DeleteSession(r.Context(), auth.HashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/entrar", http.StatusSeeOther)
}

// saveAccess liga/desliga o acesso pelo celular e define ou troca a senha.
func (s *Server) saveAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	action := r.FormValue("acao")
	values := map[string]string{}
	var msg string

	switch action {
	case "ligar", "senha":
		if s.envPasswordHash != "" {
			redirect(w, r, "/configuracoes", "", errors.New("a senha é definida pela variável FINAN_PASSWORD"))
			return
		}
		pw := r.FormValue("senha")
		if pw != r.FormValue("confirmar") {
			redirect(w, r, "/configuracoes", "", errors.New("as senhas não conferem"))
			return
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			redirect(w, r, "/configuracoes", "", err)
			return
		}
		values[keyPasswordHash] = hash
		msg = "Senha atualizada. Os aparelhos conectados precisam entrar de novo."
		if action == "ligar" {
			values[keyLAN] = "1"
			msg = "Acesso pelo celular ligado. Abra o endereço abaixo no celular, no mesmo Wi-Fi."
		}
		s.store.DeleteAllSessions(ctx)
	case "desligar":
		values[keyLAN] = "0"
		msg = "Acesso pelo celular desligado: o app voltou a aceitar só este computador."
		s.store.DeleteAllSessions(ctx)
	default:
		redirect(w, r, "/configuracoes", "", errors.New("ação inválida"))
		return
	}

	if err := s.store.SaveSettings(ctx, accessPrefix, values); err != nil {
		redirect(w, r, "/configuracoes", "", err)
		return
	}
	// Se quem trocou a senha está no celular, mantém essa pessoa conectada.
	if !isLoopback(r) && action == "senha" {
		s.startSession(ctx, w, r)
	}
	redirect(w, r, "/configuracoes#acesso", msg, nil)
	if _, changed := values[keyLAN]; changed && s.onNetworkChange != nil {
		// Reabre a porta (só local ou toda a rede) depois que a resposta sair.
		go func() {
			time.Sleep(300 * time.Millisecond)
			s.onNetworkChange()
		}()
	}
}
