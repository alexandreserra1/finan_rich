// Package handler é o intermediário da IA grátis como função da Vercel
// (docs/IA-GRATIS.md): a mesma lógica do cmd/finanproxy (internal/proxy).
// O vercel.json manda todo caminho para cá em ?path=, porque a função fica
// em /api/index e o app chama <endereço>/chat/completions.
package handler

import (
	"log"
	"net/http"
	"sync"

	"github.com/alexandreserra1/finan_rich/internal/proxy"
)

var (
	once sync.Once
	h    http.Handler
)

// Handler é a entrada que a Vercel chama a cada pedido. A instância fica
// ligada entre pedidos, e o limitador de internal/proxy vive na memória dela.
// shortcut: o limite por aparelho zera quando a Vercel troca de instância; a cota grátis
// do Groq (conta sem cartão) continua sendo o teto, então não gera cobrança. Se o abuso
// virar problema, guardar a contagem no Vercel KV.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		cfg, err := proxy.ConfigFromEnv()
		if err != nil {
			log.Print(err)
			h = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "intermediário sem chave configurada", http.StatusServiceUnavailable)
			})
			return
		}
		h = proxy.New(cfg)
	})
	if p := r.URL.Query().Get("path"); p != "" {
		r.URL.Path = "/" + p
	}
	h.ServeHTTP(w, r)
}
