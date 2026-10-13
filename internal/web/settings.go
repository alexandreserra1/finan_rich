package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ai, err := s.aiSettings(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	data := map[string]any{
		"AI": ai, "Status": s.aiStatus(ctx, ai), "Presets": llm.Presets, "HasKey": ai.APIKey != "",
		"LAN": s.LANEnabled(ctx), "EnvPassword": s.envPasswordHash != "",
	}
	data["Port"] = s.port
	if ts := tailscaleURL(ctx); ts != "" {
		data["TailscaleURL"] = ts
		data["TailscaleQR"] = qrDataURI(ts)
	}
	if urls := lanURLs(s.port); len(urls) > 0 {
		data["LANURLs"] = urls
		data["LANQR"] = qrDataURI(urls[0])
	}
	// Endereço que o app do celular usa: o do Wi-Fi (se o acesso estiver
	// ligado) ou o do Tailscale.
	base, _ := data["TailscaleURL"].(string)
	if urls, _ := data["LANURLs"].([]string); len(urls) > 0 && data["LAN"] == true {
		base = urls[0]
	}
	app, err := s.pairingView(ctx, base)
	if err != nil {
		serverError(w, err)
		return
	}
	data["App"] = app
	s.render(w, r, "settings", data)
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current, err := s.aiSettings(ctx)
	if err != nil {
		redirect(w, r, "/configuracoes", "", err)
		return
	}
	preset := llm.PresetByID(r.FormValue("preset"))
	ai := AISettings{Preset: preset, Config: llm.Config{
		Kind:    preset.Kind,
		BaseURL: strings.TrimSpace(r.FormValue("base_url")),
		Model:   strings.TrimSpace(r.FormValue("model")),
		APIKey:  strings.TrimSpace(r.FormValue("api_key")),
	}}
	// Campo de chave em branco mantém a chave já salva do mesmo provedor.
	if ai.APIKey == "" && current.Preset.ID == preset.ID {
		ai.APIKey = current.APIKey
	}
	if _, err := llm.New(ai.Config); err != nil {
		redirect(w, r, "/configuracoes", "", err)
		return
	}
	err = s.saveAISettings(ctx, ai)
	redirect(w, r, "/configuracoes", "Configuração da IA salva.", err)
}

// testAI faz uma pergunta curta ao modelo configurado e devolve o resultado em texto.
func (s *Server) testAI(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	provider, _, err := s.provider(ctx)
	if err == nil {
		var answer string
		start := time.Now()
		answer, err = provider.Chat(ctx, llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Responda apenas com a frase: Tudo certo! Estou pronto para ajudar com suas finanças."},
		}}, func(string) {})
		if err == nil {
			w.Write([]byte("✅ Funcionou em " + time.Since(start).Round(100*time.Millisecond).String() + ". Resposta: " + answer))
			return
		}
	}
	w.WriteHeader(http.StatusBadGateway)
	w.Write([]byte("❌ " + err.Error()))
}

// pullModel baixa o modelo configurado no Ollama, enviando o progresso em SSE.
func (s *Server) pullModel(w http.ResponseWriter, r *http.Request) {
	sse, ok := newSSE(w)
	if !ok {
		return
	}
	provider, _, err := s.provider(r.Context())
	if err != nil {
		sse.Error(err)
		return
	}
	ollama, ok := provider.(*llm.Ollama)
	if !ok {
		sse.Error(errors.New("o download de modelos só existe para o Ollama"))
		return
	}
	err = ollama.Pull(r.Context(), func(p llm.PullProgress) { sse.Send(p) })
	if err != nil {
		sse.Error(err)
		return
	}
	sse.Done()
}
