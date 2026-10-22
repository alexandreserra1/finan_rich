// Comando finanproxy: o intermediário do "finan_rich grátis" (internal/proxy),
// feito para o Google Cloud Run (escala a zero; só liga quando alguém pergunta).
//
// Variáveis:
//
//	GROQ_API_KEY         chave do Groq (no Cloud Run, vem do Secret Manager)
//	PORT                 porta (o Cloud Run define; padrão 8080)
//	FINAN_PROXY_MODELS   modelos em ordem, separados por vírgula
//	FINAN_PROXY_UPSTREAM API no formato OpenAI (padrão: Groq)
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
)

// Modelos abertos do Groq com ferramentas; cada um tem a sua cota grátis.
const defaultModels = "llama-3.3-70b-versatile,openai/gpt-oss-120b,qwen/qwen3-32b,meta-llama/llama-4-scout-17b-16e-instruct"

func main() {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		log.Fatal("falta GROQ_API_KEY")
	}
	cfg := proxy.Config{
		Upstream: env("FINAN_PROXY_UPSTREAM", "https://api.groq.com/openai/v1"),
		APIKey:   key,
		Models:   strings.Split(env("FINAN_PROXY_MODELS", defaultModels), ","),
		Limits:   proxy.DefaultLimits,
	}
	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           proxy.New(cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second, // corpo mandado devagar não segura uma vaga (a resposta em streaming não é afetada)
	}
	log.Printf("finanproxy ouvindo em %s (modelos: %s)", srv.Addr, strings.Join(cfg.Models, ", "))
	log.Fatal(srv.ListenAndServe())
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
