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
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments"` // texto JSON (vem em pedaços no streaming)
	} `json:"function"`
	ExtraContent json.RawMessage `json:"extra_content,omitempty"` // Gemini: thought_signature
}

type openAIChunk struct {
	Choices []struct {
		Delta struct {
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAI) Chat(ctx context.Context, req Request, onText func(string)) (Reply, error) {
	msgs := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		om := openAIMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for i, c := range m.ToolCalls {
			tc := openAIToolCall{Index: i, ID: c.ID, Type: "function", ExtraContent: c.Extra}
			tc.Function.Name, tc.Function.Arguments = c.Name, string(args(c.Args))
			om.ToolCalls = append(om.ToolCalls, tc)
		}
		msgs = append(msgs, om)
	}
	body := map[string]any{"model": o.model, "messages": msgs, "stream": true}
	if len(req.Tools) > 0 {
		body["tools"] = openAITools(req.Tools)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Reply{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Reply{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return Reply{}, fmt.Errorf("não foi possível conectar em %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Reply{}, apiError(resp)
	}

	var calls []ToolCall // montados aos pedaços, pelo índice
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
			for _, d := range chunk.Choices[0].Delta.ToolCalls {
				for len(calls) <= d.Index {
					calls = append(calls, ToolCall{})
				}
				c := &calls[d.Index]
				if d.ID != "" {
					c.ID = d.ID
				}
				if len(d.ExtraContent) > 0 {
					c.Extra = d.ExtraContent
				}
				c.Name += d.Function.Name
				c.Args = append(c.Args, d.Function.Arguments...)
			}
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return Reply{}, err
	}
	for i := range calls {
		calls[i].Args = args(calls[i].Args)
		if calls[i].ID == "" {
			calls[i].ID = fmt.Sprintf("call_%d", i+1)
		}
	}
	return Reply{Text: finish(), ToolCalls: calls}, nil
}

// apiError lê a mensagem de erro do corpo, aceitando os formatos mais comuns:
// {"error": "texto"} (Ollama) e {"error": {"message": "texto"}} (OpenAI e afins).
func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var parsed struct {
		Error json.RawMessage `json:"error"`
	}
	msg := strings.TrimSpace(string(body))
	var list []json.RawMessage // o Gemini manda o erro dentro de uma lista
	if json.Unmarshal(body, &list) == nil && len(list) > 0 {
		body = list[0]
	}
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
