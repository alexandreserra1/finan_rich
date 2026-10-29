// Comando api (finanproxy): o intermediário do "finan_rich grátis" (internal/proxy)
// e, compilado com -tags sitebundle, também o site (package site). Roda na Vercel
// (que procura o programa em cmd/api, daí o nome) ou no Google Cloud Run.
//
// Variáveis: proxy.ConfigFromEnv; PORT é a porta (o Cloud Run define; padrão 8080).
//
// Como publicar: docs/IA-GRATIS.md.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/proxy"
	"github.com/alexandreserra1/finan_rich/site"
)

func main() {
	cfg, err := proxy.ConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	handler := proxy.New(cfg)
	if pages := site.Handler(); pages != nil {
		mux := http.NewServeMux()
		for _, p := range proxy.Paths {
			mux.Handle(p, handler)
		}
		mux.Handle("/", pages)
		handler = mux
	}
	srv := &http.Server{
		Addr:              ":" + port(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second, // corpo mandado devagar não segura uma vaga (a resposta em streaming não é afetada)
	}
	log.Printf("finanproxy ouvindo em %s (provedores: %s)", srv.Addr, strings.Join(names(cfg.Providers), ", "))
	log.Fatal(srv.ListenAndServe())
}

func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

// names lista os provedores ligados (só os nomes: a chave nunca vai para o log).
func names(ps []proxy.Provider) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}
