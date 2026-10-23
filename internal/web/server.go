// Package web expõe a interface HTML (renderizada no servidor com html/template).
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/auth"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	store           *store.Store
	aiDefaults      AISettings
	pages           map[string]*template.Template
	port            string
	envPasswordHash string // senha de FINAN_PASSWORD (modo servidor), já com hash
	limiter         *auth.Limiter
	onNetworkChange func()
	onQuit          func()
	updates         *updateChecker
	assetsVersion   string // impressão digital de static/ e templates/
	pairing         pairing
}

// Options configura o servidor.
type Options struct {
	AIPreset   string     // preset de IA usado enquanto o usuário não salvar outro
	AIDefaults llm.Config // valores iniciais da IA
	Port       string     // porta HTTP, para montar o endereço do celular
	// Password, se definida (FINAN_PASSWORD), vale para todo acesso, inclusive
	// o local, e não pode ser trocada pela tela.
	Password string
	// OnNetworkChange é chamado quando o acesso pelo celular é ligado ou
	// desligado, para o programa reabrir a porta no endereço certo.
	OnNetworkChange func()
	// OnQuit fecha o programa (botão "Fechar o finan_rich" nos Ajustes).
	OnQuit func()
	// Version é a versão rodando, para avisar quando sair uma nova; vazia
	// desliga o aviso.
	Version string
}

var funcs = template.FuncMap{
	// jsonText é o JSON como texto comum (para atributos data-*).
	"jsonText": func(v any) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	},
	"json": func(v any) (template.JS, error) {
		b, err := json.Marshal(v)
		return template.JS(b), err
	},
}

func New(st *store.Store, opts Options) (*Server, error) {
	s := &Server{
		store:           st,
		aiDefaults:      AISettings{Preset: llm.PresetByID(opts.AIPreset), Config: opts.AIDefaults},
		pages:           map[string]*template.Template{},
		port:            opts.Port,
		limiter:         auth.NewLimiter(5, 10*time.Minute),
		onNetworkChange: opts.OnNetworkChange,
		onQuit:          opts.OnQuit,
		updates:         newUpdateCheckerIf(opts.Version),
		assetsVersion:   assetsFingerprint(),
	}
	if _, err := staticFS.ReadFile("static/finance.wasm"); err != nil {
		return nil, errors.New("static/finance.wasm não encontrado: rode 'go generate ./internal/web' (ou 'make wasm') antes de compilar")
	}
	if opts.Password != "" {
		hash, err := auth.HashPassword(opts.Password)
		if err != nil {
			return nil, fmt.Errorf("FINAN_PASSWORD: %w", err)
		}
		s.envPasswordHash = hash
	}
	pages, err := fs.Glob(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	// Arquivos que começam com "_" são parciais (layout e blocos reaproveitados)
	// e entram em todas as páginas; os demais são páginas.
	partials, err := fs.Glob(templatesFS, "templates/_*.html")
	if err != nil {
		return nil, err
	}
	for _, p := range pages {
		name := strings.TrimSuffix(strings.TrimPrefix(p, "templates/"), ".html")
		if strings.HasPrefix(name, "_") {
			continue
		}
		t, err := template.New("_layout.html").Funcs(funcs).ParseFS(templatesFS, append(partials, p)...)
		if err != nil {
			return nil, err
		}
		s.pages[name] = t
	}
	return s, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", &staticFiles{})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /manifest.webmanifest", serveStatic("manifest.webmanifest", "application/manifest+json"))
	mux.HandleFunc("GET /sw.js", s.serviceWorker)
	mux.HandleFunc("GET /offline", serveStatic("offline.html", "text/html; charset=utf-8"))

	mux.HandleFunc("GET /entrar", s.loginPage)
	mux.HandleFunc("POST /entrar", s.login)
	mux.HandleFunc("POST /sair", s.logout)

	// Telas de dados: o servidor entrega só o "casco"; o aparelho monta a tela
	// com os dados guardados nele (funciona offline) e sincroniza com /api/sync.
	for path, page := range map[string]string{"/{$}": "dashboard", "/lancamentos": "transactions", "/dividas": "debts", "/categorias": "categories", "/notas": "notes", "/exportar": "export", "/importar": "import", "/metas": "goals", "/analise": "insights"} {
		mux.HandleFunc("GET "+path, s.appShell(page))
	}
	mux.HandleFunc("GET /contas", s.appShell("accounts"))
	mux.HandleFunc("POST /api/sync", s.apiSync)
	mux.HandleFunc("POST /api/categorizar", s.apiCategorize)
	mux.HandleFunc("GET /api/chat", s.apiChatHistory)
	mux.HandleFunc("POST /api/chat", s.apiChatSend)
	mux.HandleFunc("POST /api/chat/limpar", s.apiChatClear)

	// App do celular: pareamento por código (veja pairing.go).
	mux.HandleFunc("GET /parear", s.pairPage)
	mux.HandleFunc("POST /api/parear", s.apiPair)
	mux.HandleFunc("POST /configuracoes/parear", s.startPairing)
	mux.HandleFunc("POST /configuracoes/aparelhos/remover", s.removeDevice)

	mux.HandleFunc("GET /chat", s.chat)
	mux.HandleFunc("POST /chat/enviar", s.chatSend)
	mux.HandleFunc("POST /chat/limpar", s.chatClear)

	mux.HandleFunc("GET /configuracoes", s.settings)
	mux.HandleFunc("POST /configuracoes", s.saveSettings)
	mux.HandleFunc("POST /configuracoes/testar", s.testAI)
	mux.HandleFunc("POST /configuracoes/baixar-modelo", s.pullModel)
	mux.HandleFunc("POST /configuracoes/acesso", s.saveAccess)
	mux.HandleFunc("POST /configuracoes/fechar", s.quit)
	mux.HandleFunc("GET /configuracoes/backup.db", s.backupDB)
	return logRequests(securityHeaders(apiCORS(sameOrigin(s.requireAuth(mux)))))
}

// render executa a página; dados comuns (página ativa, avisos) vão no mapa.
func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data map[string]any) {
	if _, ok := data["Page"]; !ok {
		data["Page"] = page
	}
	data["Flash"] = r.URL.Query().Get("msg")
	data["Error"] = r.URL.Query().Get("erro")
	data["Remote"] = !isLoopback(r) // acesso pela rede: mostra o botão de sair
	// Aviso de versão nova: só no próprio computador, que é onde se atualiza.
	if v, ok := s.updates.available(); ok && isLoopback(r) && data["Mobile"] != true {
		data["Update"], data["UpdateCmd"], data["UpdateOS"] = v, updateCommand(), runtime.GOOS
	}
	// Renderiza num buffer: se o template falhar, devolve 500 em vez de meia página.
	var buf bytes.Buffer
	if err := s.pages[page].Execute(&buf, data); err != nil {
		serverError(w, fmt.Errorf("render %s: %w", page, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("erro: %v", err)
	http.Error(w, "Erro interno. Os detalhes estão na janela do finan_rich.", http.StatusInternalServerError)
}

// sameOrigin barra POSTs vindos de outros sites (CSRF): sem login, qualquer página
// aberta no navegador poderia enviar formulários para o app.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// O app do celular se identifica pelo token (ou pelo código de pareamento),
		// que o navegador nunca envia sozinho: aí não há risco de CSRF.
		_, hasToken := bearerToken(r)
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !hasToken && r.URL.Path != "/api/parear" && !isSameOrigin(r) {
			http.Error(w, "requisição de outra origem bloqueada", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	}
	return true
}

// serviceWorker entrega o sw.js com a impressão digital dos arquivos do app:
// mudou algum arquivo, muda o service worker, e o navegador renova o cache.
func (s *Server) serviceWorker(w http.ResponseWriter, r *http.Request) {
	b, err := staticFS.ReadFile("static/sw.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/javascript")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(bytes.ReplaceAll(b, []byte("__VERSION__"), []byte(s.assetsVersion)))
}

// appShell entrega a página que o JavaScript do aparelho preenche.
func (s *Server) appShell(page string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.render(w, r, "app", map[string]any{"Page": page, "AppPage": page})
	}
}

// browserCSP diz ao navegador o que as telas podem carregar: só arquivos do
// próprio finan_rich, nenhum script embutido na página (um texto malicioso que
// escapasse para o HTML não roda) e o WebAssembly das regras. Atributos style=""
// ficam liberados (as barras dos gráficos usam width: N%); blocos <style> não.
// O app do celular usa a mesma política, com a rede liberada (veja appCSP).
const browserCSP = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; style-src-attr 'unsafe-inline'; " +
	"img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// securityHeaders impede que o app seja aberto dentro de outro site (clickjacking),
// que o navegador "adivinhe" tipos de arquivo e que a página rode código de fora (CSP).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", browserCSP)
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// assetsFingerprint resume o conteúdo dos arquivos do app num código curto.
func assetsFingerprint() string {
	h := sha256.New()
	for _, fsys := range []fs.FS{staticFS, templatesFS} {
		fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				b, _ := fs.ReadFile(fsys, path)
				h.Write([]byte(path))
				h.Write(b)
			}
			return nil
		})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// serveStatic serve um arquivo de static/ num caminho da raiz. O manifest e o
// service worker precisam ficar na raiz para o PWA controlar o site inteiro.
func serveStatic(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/static/") && r.URL.Path != "/healthz" {
			log.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
