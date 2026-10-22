//go:build !js

package llm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// Anthropic usa os modelos Claude pelo SDK oficial (opcional, pago).
type Anthropic struct {
	client anthropic.Client
	model  string
}

// NewAnthropic cria o provedor. baseURL vazio usa a API oficial.
func NewAnthropic(apiKey, model, baseURL string) *Anthropic {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &Anthropic{client: anthropic.NewClient(opts...), model: model}
}

func (a *Anthropic) Chat(ctx context.Context, req Request, onText func(string)) (Reply, error) {
	var msgs []anthropic.BetaMessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case RoleAssistant:
			var blocks []anthropic.BetaContentBlockParamUnion
			if m.Content != "" {
				blocks = append(blocks, anthropic.NewBetaTextBlock(m.Content))
			}
			for _, c := range m.ToolCalls {
				blocks = append(blocks, anthropic.NewBetaToolUseBlock(c.ID, args(c.Args), c.Name))
			}
			msgs = append(msgs, anthropic.BetaMessageParam{Role: anthropic.BetaMessageParamRoleAssistant, Content: blocks})
		case RoleTool:
			// Resultados seguidos vão juntos numa só mensagem do usuário.
			block := anthropic.NewBetaToolResultBlock(m.ToolCallID, m.Content, false)
			if n := len(msgs); n > 0 && msgs[n-1].Role == anthropic.BetaMessageParamRoleUser && msgs[n-1].Content[0].OfToolResult != nil {
				msgs[n-1].Content = append(msgs[n-1].Content, block)
			} else {
				msgs = append(msgs, anthropic.NewBetaUserMessage(block))
			}
		default:
			msgs = append(msgs, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(m.Content)))
		}
	}
	var tools []anthropic.BetaToolUnionParam
	for _, t := range req.Tools {
		var schema struct {
			Properties any      `json:"properties"`
			Required   []string `json:"required"`
		}
		json.Unmarshal(t.Parameters, &schema)
		tool := anthropic.BetaToolUnionParamOfTool(anthropic.BetaToolInputSchemaParam{Properties: schema.Properties, Required: schema.Required}, t.Name)
		tool.OfTool.Description = anthropic.String(t.Description)
		tools = append(tools, tool)
	}
	stream := a.client.Beta.Messages.NewStreaming(ctx, anthropic.BetaMessageNewParams{
		Model:        anthropic.Model(a.model),
		MaxTokens:    16000,
		System:       []anthropic.BetaTextBlockParam{{Text: req.System}},
		Messages:     msgs,
		Tools:        tools,
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortMedium},
		// Se o filtro de segurança recusar por engano, a API tenta outro modelo sozinha.
		Fallbacks: anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()},
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
	})
	defer stream.Close()

	var msg anthropic.BetaMessage
	for stream.Next() {
		event := stream.Current()
		if err := msg.Accumulate(event); err != nil {
			return Reply{}, err
		}
		if delta, ok := event.AsAny().(anthropic.BetaRawContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.BetaTextDelta); ok {
				onText(text.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return Reply{}, &APIError{Status: apiErr.StatusCode, Message: apiErr.Error()}
		}
		return Reply{}, err
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return Reply{}, ErrRefused
	}
	var reply Reply
	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.BetaTextBlock:
			reply.Text += b.Text
		case anthropic.BetaToolUseBlock:
			input, _ := json.Marshal(b.Input)
			reply.ToolCalls = append(reply.ToolCalls, ToolCall{ID: b.ID, Name: b.Name, Args: args(input)})
		}
	}
	return reply, nil
}
