package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

// Preferred são os modelos de nuvem bons com ferramentas, na ordem de escolha
// quando o modelo salvo não existe mais no provedor. Os provedores tiram
// modelos do ar com poucas semanas de aviso; em vez de depender só desta lista,
// o app pergunta ao provedor quais existem (ListModels) e fica com o melhor.
var Preferred = []string{
	"openai/gpt-oss-120b", "qwen/qwen3.8-27b", "openai/gpt-oss-20b", // Groq
	"gemini-3.8-flash", "gemini-flash-latest", "gemini-3.5-flash-lite", // Gemini
	"openrouter/free", "deepseek-chat", // OpenRouter, DeepSeek
}

// ListModels pergunta a uma API no formato OpenAI quais modelos ela tem.
func ListModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimSlash(baseURL)+"/models", nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lista de modelos: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// notChat marca modelos que não conversam (áudio, imagem, filtros, vetores).
var notChat = []string{"whisper", "guard", "orpheus", "tts", "embed", "image", "imagen", "veo", "transcribe",
	"live", "audio", "moderation", "aqa", "lyria", "robotics", "computer-use"}

// PickModels devolve os preferidos que o provedor tem (na ordem da lista) e,
// depois, os outros modelos de conversa dele. O Gemini lista "models/<id>".
func PickModels(available, preferred []string) []string {
	var have, rest []string
	for _, id := range available {
		id = strings.TrimPrefix(id, "models/")
		if !slices.ContainsFunc(notChat, func(w string) bool { return strings.Contains(strings.ToLower(id), w) }) {
			have = append(have, id)
		}
	}
	var out []string
	for _, p := range preferred {
		if slices.Contains(have, p) {
			out = append(out, p)
		}
	}
	for _, id := range have {
		if !slices.Contains(out, id) {
			rest = append(rest, id)
		}
	}
	return append(out, rest...)
}

// modelMissing: o provedor diz que o modelo não existe (ou foi desligado).
func modelMissing(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	msg := strings.ToLower(apiErr.Message)
	return apiErr.Status == http.StatusNotFound ||
		apiErr.Status == http.StatusBadRequest && strings.Contains(msg, "model") && !strings.Contains(msg, "tool") && !strings.Contains(msg, "function")
}
