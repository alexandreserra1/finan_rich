// Package web expõe a interface HTML (server-side, html/template) e o chat.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/money"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	store   *store.Store
	advisor *advisor.Advisor // nil quando não há ANTHROPIC_API_KEY
	pages   map[string]*template.Template
}

var bucketLabels = map[string]string{
	store.BucketNecessidades:  "Necessidades",
	store.BucketDesejos:       "Desejos",
	store.BucketInvestimentos: "Investimentos",
	store.BucketRenda:         "Renda",
}

var funcs = template.FuncMap{
	"brl":         money.Format,
	"moneyInput":  money.Input,
	"rate":        advisor.RatePct,
	"rateInput":   func(bp int) string { return money.Input(int64(bp)) },
	"bucketLabel": func(b string) string { return bucketLabels[b] },
	"neg":         func(c int64) bool { return c < 0 },
	"abs": func(c int64) int64 {
		if c < 0 {
			return -c
		}
		return c
	},
	"min100": func(p int) int { return min(p, 100) },
	"barPct": func(v, max int64) int {
		if max <= 0 {
			return 0
		}
		return int(v * 100 / max)
	},
}

func New(st *store.Store, adv *advisor.Advisor) (*Server, error) {
	s := &Server{store: st, advisor: adv, pages: map[string]*template.Template{}}
	pages, err := fs.Glob(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	for _, p := range pages {
		name := strings.TrimSuffix(strings.TrimPrefix(p, "templates/"), ".html")
		if name == "layout" {
			continue
		}
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(templatesFS, "templates/layout.html", p)
		if err != nil {
			return nil, err
		}
		s.pages[name] = t
	}
	return s, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServerFS(staticFS))

	mux.HandleFunc("GET /{$}", s.dashboard)

	mux.HandleFunc("GET /lancamentos", s.transactions)
	mux.HandleFunc("POST /lancamentos", s.createTransaction)
	mux.HandleFunc("POST /lancamentos/{id}/apagar", s.deleteTransaction)
	mux.HandleFunc("POST /lancamentos/copiar-mes-anterior", s.copyPreviousMonth)

	mux.HandleFunc("GET /categorias", s.categories)
	mux.HandleFunc("POST /categorias", s.createCategory)
	mux.HandleFunc("POST /categorias/{id}/apagar", s.deleteCategory)
	mux.HandleFunc("POST /divisao", s.saveSplit)

	mux.HandleFunc("GET /dividas", s.debts)
	mux.HandleFunc("POST /dividas", s.saveDebt)
	mux.HandleFunc("POST /dividas/{id}/apagar", s.deleteDebt)

	mux.HandleFunc("GET /chat", s.chat)
	mux.HandleFunc("POST /chat/enviar", s.chatSend)
	mux.HandleFunc("POST /chat/limpar", s.chatClear)
	return logRequests(sameOrigin(mux))
}

// render executa a página; dados comuns (página ativa, flash) vão no mapa.
func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data map[string]any) {
	data["Page"] = page
	data["Flash"] = r.URL.Query().Get("msg")
	data["Error"] = r.URL.Query().Get("erro")
	data["ChatEnabled"] = s.advisor != nil
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[page].Execute(w, data); err != nil {
		log.Printf("render %s: %v", page, err)
	}
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("erro: %v", err)
	http.Error(w, "Erro interno: "+err.Error(), http.StatusInternalServerError)
}

// sameOrigin barra POSTs vindos de outros sites (CSRF): sem login, qualquer página
// aberta no navegador poderia enviar formulários para o app.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
				http.Error(w, "requisição de outra origem bloqueada", http.StatusForbidden)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
					http.Error(w, "requisição de outra origem bloqueada", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			log.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
