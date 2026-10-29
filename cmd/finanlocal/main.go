// finanlocal empresta a IA deste computador (Ollama) ao intermediário grátis do
// finan_rich, sem cota de tokens. Fica na frente do Ollama e só escuta neste
// computador; um túnel (Tailscale Funnel, por exemplo) leva o endereço para a
// internet. Passo a passo em docs/IA-GRATIS.md.
//
//	FINAN_LOCAL_KEY=<senha longa> OLLAMA_CONTEXT_LENGTH=8192 ollama serve   (em outro terminal: só o OLLAMA_...)
//	FINAN_LOCAL_KEY=<senha longa> go run ./cmd/finanlocal
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/localguard"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:11435", "onde escutar (só este computador; o túnel aponta para cá)")
	ollama := flag.String("ollama", "http://127.0.0.1:11434", "endereço do Ollama")
	model := flag.String("model", "qwen2.5:7b-instruct", "o único modelo que responde")
	busy := flag.Int("busy", 1, "perguntas ao mesmo tempo (as outras recebem 429 e vão para outro provedor)")
	flag.Parse()

	h, err := localguard.New(localguard.Config{Key: os.Getenv("FINAN_LOCAL_KEY"), Ollama: *ollama, Model: *model, MaxBusy: *busy})
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Addr: *addr, Handler: h, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second}
	log.Printf("finanlocal ouvindo em %s, repassando %s ao Ollama (%s)", *addr, *model, *ollama)
	log.Fatal(srv.ListenAndServe())
}
