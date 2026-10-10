package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAI conversa com qualquer API compatível com /v1/chat/completions da OpenAI.
// É o formato mais comum: Groq, DeepSeek, OpenRouter, LM Studio, vLLM, etc.
type OpenAI struct {
	baseURL string // inclui o /v1, ex: https://api.groq.com/openai/v1
	apiKey  string
	model   string
}

func NewOpenAI(baseURL, apiKey, model string) *OpenAI {
	return &OpenAI{baseURL: trimSlash(baseURL), apiKey: apiKey, model: model}
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAI) Chat(ctx context.Context, req Request, onText func(string)) (string, error) {
	msgs := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, openAIMessage{Role: string(m.Role), Content: m.Content})
	}
	payload, err := json.Marshal(map[string]any{"model": o.model, "messages": msgs, "stream": true})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("não foi possível conectar em %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", apiError(resp)
	}

	emit, finish := streamText(onText)
	err = readLines(resp.Body, func(line []byte) error {
		data, ok := bytes.CutPrefix(line, []byte("data:"))
		if !ok {
			return nil // comentários e campos "event:" do SSE
		}
		data = bytes.TrimSpace(data)
		if string(data) == "[DONE]" {
			return io.EOF
		}
		var chunk openAIChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return fmt.Errorf("resposta inválida do provedor: %w", err)
		}
		if chunk.Error != nil {
			return &APIError{Status: http.StatusInternalServerError, Message: chunk.Error.Message}
		}
		if len(chunk.Choices) > 0 {
			emit(chunk.Choices[0].Delta.Content)
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	return finish(), nil
}

// apiError lê a mensagem de erro do corpo, aceitando os formatos mais comuns:
// {"error": "texto"} (Ollama) e {"error": {"message": "texto"}} (OpenAI e afins).
func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var parsed struct {
		Error json.RawMessage `json:"error"`
	}
	msg := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &parsed) == nil && len(parsed.Error) > 0 {
		var text string
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(parsed.Error, &text) == nil {
			msg = text
		} else if json.Unmarshal(parsed.Error, &obj) == nil && obj.Message != "" {
			msg = obj.Message
		}
	}
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return &APIError{Status: resp.StatusCode, Message: msg}
}
