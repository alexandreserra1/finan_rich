// Package localguard é o porteiro entre a internet e o Ollama de um computador
// que empresta a IA para o intermediário grátis (cmd/finanlocal). O Ollama não
// tem senha: exposto direto, qualquer um usaria o computador e chegaria em
// /api/pull, /api/delete etc. Aqui só passa a conversa no formato OpenAI, só
// com a chave certa, sempre com o modelo escolhido e uma pergunta por vez.
package localguard

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Config do porteiro.
type Config struct {
	Key     string // a mesma de FINAN_LOCAL_KEY na Vercel (24+ caracteres)
	Ollama  string // endereço do Ollama, ex.: http://127.0.0.1:11434
	Model   string // único modelo que responde, ex.: qwen2.5:7b-instruct
	MaxBusy int    // perguntas ao mesmo tempo (padrão 1: o resto recebe 429 e vai para outro provedor)
}

const maxBody = 256 << 10 // o pedido do consultor tem uns 20 KB; o intermediário aceita até 128 KB

// New devolve o porteiro.
func New(cfg Config) (http.Handler, error) {
	if len(cfg.Key) < 24 {
		return nil, errors.New("FINAN_LOCAL_KEY precisa de pelo menos 24 caracteres (gere uma senha longa e aleatória)")
	}
	target, err := url.Parse(cfg.Ollama)
	if err != nil || target.Host == "" {
		return nil, errors.New("endereço do Ollama inválido")
	}
	if cfg.Model == "" {
		return nil, errors.New("falta o modelo")
	}
	if cfg.MaxBusy < 1 {
		cfg.MaxBusy = 1
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = -1 // streaming: cada pedaço segue na hora
	slots := make(chan struct{}, cfg.MaxBusy)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []map[string]string{{"id": cfg.Model, "object": "model"}}})
	})
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		var body map[string]json.RawMessage
		if err != nil || json.Unmarshal(raw, &body) != nil {
			http.Error(w, `{"error":{"message":"pedido inválido"}}`, http.StatusBadRequest)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":{"message":"o computador está ocupado com outra pergunta"}}`)
			return
		}
		body["model"], _ = json.Marshal(cfg.Model)
		delete(body, "reasoning_effort") // só para os gpt-oss da nuvem
		out, _ := json.Marshal(body)
		r.Body, r.ContentLength = io.NopCloser(bytes.NewReader(out)), int64(len(out))
		r.Header.Del("Authorization") // a chave para aqui; o Ollama não precisa dela
		rp.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(cfg.Key)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"error":{"message":"chave inválida"}}`)
			return
		}
		mux.ServeHTTP(w, r)
	}), nil
}
