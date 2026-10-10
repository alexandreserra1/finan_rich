package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// DefaultOllamaURL é onde o Ollama escuta após a instalação.
const DefaultOllamaURL = "http://localhost:11434"

// Ollama conversa com um servidor Ollama local pela API nativa (/api/chat), que
// permite ajustar o tamanho do contexto e desligar o "modo raciocínio".
type Ollama struct {
	baseURL string
	model   string
}

func NewOllama(baseURL, model string) *Ollama {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	return &Ollama{baseURL: trimSlash(baseURL), model: model}
}

// contextTokens é a janela de contexto pedida ao Ollama. O padrão dele (4096)
// é pequeno para o retrato financeiro + histórico.
const contextTokens = 8192

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"` // false: responde direto, sem raciocínio longo
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaChatChunk struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
	Error   string        `json:"error"`
}

func (o *Ollama) Chat(ctx context.Context, req Request, onText func(string)) (string, error) {
	msgs := make([]ollamaMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, ollamaMessage{Role: string(m.Role), Content: m.Content})
	}
	resp, err := o.post(ctx, "/api/chat", ollamaChatRequest{
		Model: o.model, Messages: msgs, Stream: true,
		Options: map[string]any{"num_ctx": contextTokens},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	emit, finish := streamText(onText)
	err = readLines(resp.Body, func(line []byte) error {
		var chunk ollamaChatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			return fmt.Errorf("resposta inválida do Ollama: %w", err)
		}
		if chunk.Error != "" {
			return &APIError{Status: http.StatusInternalServerError, Message: chunk.Error}
		}
		emit(chunk.Message.Content)
		return nil
	})
	if err != nil {
		return "", err
	}
	return finish(), nil
}

// ---- Gerenciamento de modelos (usado pela tela de configurações) ----

// ErrOllamaOffline indica que o Ollama não está instalado ou não está rodando.
var ErrOllamaOffline = errors.New("Ollama não encontrado. Instale em https://ollama.com e deixe-o aberto")

// Models lista os modelos já baixados no Ollama.
func (o *Ollama) Models(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, ErrOllamaOffline
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	names := make([]string, len(body.Models))
	for i, m := range body.Models {
		names[i] = m.Name
	}
	slices.Sort(names)
	return names, nil
}

// HasModel diz se o modelo configurado já foi baixado ("qwen3" casa com "qwen3:latest").
func (o *Ollama) HasModel(ctx context.Context) (bool, error) {
	models, err := o.Models(ctx)
	if err != nil {
		return false, err
	}
	want := o.model
	if !strings.Contains(want, ":") {
		want += ":latest"
	}
	return slices.Contains(models, want), nil
}

// PullProgress é o andamento do download de um modelo.
type PullProgress struct {
	Status    string `json:"status"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Error     string `json:"error"`
}

// Pull baixa o modelo configurado, chamando onProgress a cada atualização.
func (o *Ollama) Pull(ctx context.Context, onProgress func(PullProgress)) error {
	resp, err := o.post(ctx, "/api/pull", map[string]any{"model": o.model, "stream": true})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return readLines(resp.Body, func(line []byte) error {
		var p PullProgress
		if err := json.Unmarshal(line, &p); err != nil {
			return err
		}
		if p.Error != "" {
			return &APIError{Status: http.StatusInternalServerError, Message: p.Error}
		}
		onProgress(p)
		return nil
	})
}

func (o *Ollama) post(ctx context.Context, path string, body any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrOllamaOffline
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		err := apiError(resp)
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("o modelo %q ainda não foi baixado; baixe-o em Configurações (%w)", o.model, err)
		}
		return nil, err
	}
	return resp, nil
}
