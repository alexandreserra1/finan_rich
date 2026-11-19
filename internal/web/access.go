package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
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

const untrustedHostMsg = "endereço desconhecido: abra pelo IP ou por localhost (ou acrescente o nome em FINAN_HOSTS)"

// trustedHost barra o "DNS rebinding": um site de fora cujo nome aponta para
// 127.0.0.1 chega aqui como se fosse local, mas com o nome dele no Host. Sem
// senha, só valem IPs, localhost, nomes .local (rede de casa), .ts.net
// (Tailscale) e os nomes listados em FINAN_HOSTS, separados por vírgula.
func trustedHost(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	if net.ParseIP(host) != nil || host == "localhost" {
		return true
	}
	for _, suffix := range []string{".localhost", ".local", ".ts.net"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	for _, h := range strings.Split(os.Getenv("FINAN_HOSTS"), ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" && h == host {
			return true
		}
	}
	return false
}

// isLoopback diz se a requisição veio do próprio computador. Requisições
// repassadas por um proxy local (Tailscale Serve, nginx...) vêm de outro
// aparelho e não contam como locais.
func isLoopback(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("Tailscale-User-Login") != "" {
		return false
	}
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

// access diz quem pode usar uma rota (autorização). Cada rota da tabela (routes, em
// server.go) declara o seu; o zero não é nível nenhum e nega tudo (negado por padrão).
type access int

const (
	public        access = iota + 1 // qualquer um: login, arquivos do app, pareamento por código
	owner                           // a pessoa: neste computador ou com login pela rede
	ownerOrDevice                   // a pessoa ou o app do celular pareado (token do aparelho)
	localOnly                       // a pessoa, e só neste computador (fechar o programa)
)

// authorize deixa passar só quem pode usar a rota. Quem usa no próprio computador entra
// direto, a não ser que a senha venha de FINAN_PASSWORD (modo servidor, onde um proxy
// reverso faria tudo parecer local); pela rede, só com login (senha e, se ligado, o
// código de 6 dígitos) ou com o token de um aparelho pareado, nas rotas do app.
func (s *Server) authorize(who access, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case who < public || who > localOnly:
			http.Error(w, "rota sem regra de acesso", http.StatusForbidden)
			return
		case who == public:
			next.ServeHTTP(w, r)
			return
		}
		// App pareado: vale só o token, mesmo vindo deste computador (assim uma
		// página de outro site não usa o acesso local para ler os dados).
		if token, ok := bearerToken(r); ok {
			// O token do celular vale só para as rotas do app (sincronizar, consultor):
			// quem roubar um token não troca a IA nem baixa o banco inteiro.
			if who != ownerOrDevice {
				http.Error(w, "o token do aparelho só vale para a sincronização", http.StatusForbidden)
				return
			}
			if !s.validDevice(r.Context(), token) {
				writeJSONError(w, http.StatusUnauthorized, "aparelho não pareado; pareie de novo em Ajustes → App do celular")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if !s.isOwner(w, r) {
			return
		}
		if who == localOnly && !isLoopback(r) {
			http.Error(w, "só dá para fazer isso no próprio computador", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isOwner diz se o pedido é da pessoa: neste computador ou com login pela rede. Se não
// for, já responde (recusa, ou o caminho para entrar) e devolve false.
func (s *Server) isOwner(w http.ResponseWriter, r *http.Request) bool {
	// Sem token, um pedido feito por outro site (fetch, imagem...) não usa o
	// acesso local nem o login: senão qualquer página aberta no computador
	// leria a conversa e os dados. Abrir um link (navegação) continua valendo.
	if !isSameOrigin(r) && r.Header.Get("Sec-Fetch-Mode") != "navigate" {
		http.Error(w, "requisição de outra origem bloqueada", http.StatusForbidden)
		return false
	}
	if s.envPasswordHash == "" && isLoopback(r) {
		if !trustedHost(r) {
			http.Error(w, untrustedHostMsg, http.StatusForbidden)
			return false
		}
		return true
	}
	hash, err := s.passwordHash(r.Context())
	if err != nil {
		serverError(w, err)
		return false
	}
	if hash == "" && !trustedHost(r) {
		http.Error(w, untrustedHostMsg, http.StatusForbidden)
		return false
	}
	if hash == "" || s.validSession(r) {
		// Sem senha só se chega aqui pela rede quando o app foi exposto de
		// propósito (flag -addr); o aviso aparece ao iniciar.
		return true
	}
	if r.Method == http.MethodGet {
		http.Redirect(w, r, "/entrar?volta="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return false
	}
	http.Error(w, "faça login para continuar", http.StatusUnauthorized)
	return false
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
	// IPv6: cada casa tem um /64 inteiro; trocar de endereço dentro dele não zera o limite.
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return host
}

// safeReturn evita redirecionar para outro site depois do login: só um caminho deste site.
// Caractere de controle e barra invertida não passam, em lugar nenhum: o navegador apaga
// tab e quebra de linha da URL ("/\t/site" viraria "//site") e trata "\\" como "/".
func safeReturn(path string) string {
	u, err := url.Parse(path)
	// "/\\" já cai na regra da barra invertida; fica explícito porque é o padrão que o CodeQL
	// reconhece como checagem completa de redirecionamento (go/bad-redirect-check).
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") ||
		strings.HasPrefix(path, "/\\") || strings.ContainsFunc(path, func(r rune) bool { return r < 0x20 || r == 0x7f || r == '\\' }) {
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
	if !s.limiter.Take(key) {
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
		s.logAccess(r, "Senha errada")
		fail("Senha incorreta.")
		return
	}
	s.limiter.Reset(key)
	// Com a verificação em duas etapas, a senha certa leva ao passo do código, que tem o
	// limite de tentativas dele (codeLimiter).
	if s.mfaOn(ctx) {
		if err := s.askCode(w, r, back); err != nil {
			fail(err.Error())
		}
		return
	}
	if err := s.startSession(ctx, w, r); err != nil {
		serverError(w, err)
		return
	}
	s.logAccess(r, "Entrou pela rede")
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
		s.logAccess(r, "Saiu")
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	http.Redirect(w, r, "/entrar", http.StatusSeeOther)
}

// saveAccess liga/desliga o acesso pelo celular e define ou troca a senha.
func (s *Server) saveAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	action := r.FormValue("acao")
	values := map[string]string{}
	var msg string
	if strings.HasPrefix(action, "mfa-") {
		s.saveMFA(w, r, action)
		return
	}
	// Pela rede, trocar a senha ou desligar o acesso pede a senha atual (e o código).
	if action == "senha" || action == "desligar" {
		if err := s.reauth(r); err != nil {
			redirect(w, r, "/configuracoes#acesso", "", err)
			return
		}
	}

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
		msg = "Senha atualizada. Os aparelhos conectados precisam entrar (ou parear o app) de novo."
		if action == "ligar" {
			values[keyLAN] = "1"
			msg = "Acesso pelo celular ligado. Abra o endereço abaixo no celular, no mesmo Wi-Fi."
		}
		s.store.DeleteAllSessions(ctx)
		s.store.DeleteAllDevices(ctx)
	case "desligar":
		values[keyLAN] = "0"
		msg = "Acesso pelo celular desligado: o app voltou a aceitar só este computador."
		s.store.DeleteAllSessions(ctx)
		s.store.DeleteAllDevices(ctx)
		s.pairing.cancel()
	default:
		redirect(w, r, "/configuracoes", "", errors.New("ação inválida"))
		return
	}

	if err := s.store.SaveSettings(ctx, accessPrefix, values); err != nil {
		redirect(w, r, "/configuracoes", "", err)
		return
	}
	s.logAccess(r, map[string]string{"ligar": "Acesso pelo celular ligado", "senha": "Senha trocada", "desligar": "Acesso pelo celular desligado"}[action])
	// Ligar o acesso já oferece o código de 6 dígitos (marcado por padrão): o QR aparece em seguida.
	back := "/configuracoes#acesso"
	if action == "ligar" && r.FormValue("mfa") == "1" && !s.mfaOn(ctx) {
		if err := s.startMFA(ctx); err != nil {
			redirect(w, r, "/configuracoes", "", err)
			return
		}
		msg += " Agora leia o QR code com o app autenticador para ligar o código de 6 dígitos."
		back = "/configuracoes#mfa"
	}
	// Se quem trocou a senha está no celular, mantém essa pessoa conectada.
	if !isLoopback(r) && action == "senha" {
		s.startSession(ctx, w, r)
	}
	redirect(w, r, back, msg, nil)
	if _, changed := values[keyLAN]; changed && s.onNetworkChange != nil {
		// Reabre a porta (só local ou toda a rede) depois que a resposta sair.
		go func() {
			time.Sleep(300 * time.Millisecond)
			s.onNetworkChange()
		}()
	}
}
