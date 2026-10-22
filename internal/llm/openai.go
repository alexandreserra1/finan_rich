package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
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
		FinishReason string `json:"finish_reason"`
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
	finished := false    // [DONE] ou finish_reason: sem isso, a conexão caiu no meio
	emit, finish := streamText(onText)
	err = readLines(resp.Body, func(line []byte) error {
		data, ok := bytes.CutPrefix(line, []byte("data:"))
		if !ok {
			return nil // comentários e campos "event:" do SSE
		}
		data = bytes.TrimSpace(data)
		if string(data) == "[DONE]" {
			finished = true
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
			finished = finished || chunk.Choices[0].FinishReason != ""
			emit(chunk.Choices[0].Delta.Content)
			for _, d := range chunk.Choices[0].Delta.ToolCalls {
				// Um id novo é um pedido novo: o Gemini manda pedidos em paralelo
				// todos com index 0, cada um inteiro num pedaço. Sem id, é
				// continuação do pedido daquele index (formato da OpenAI).
				i := slices.IndexFunc(calls, func(c ToolCall) bool { return d.ID != "" && c.ID == d.ID })
				switch {
				case i >= 0:
				case d.ID != "" && (d.Index >= len(calls) || calls[d.Index].ID != ""):
					calls = append(calls, ToolCall{ID: d.ID})
					i = len(calls) - 1
				default:
					for len(calls) <= d.Index {
						calls = append(calls, ToolCall{})
					}
					i = d.Index
					if d.ID != "" {
						calls[i].ID = d.ID
					}
				}
				c := &calls[i]
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
	if !finished {
		return Reply{}, errors.New("a resposta da IA foi cortada no meio (a conexão caiu); tente de novo")
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
