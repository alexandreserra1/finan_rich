package llm

import (
	"context"
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

// ErrRefused indica que o modelo recusou responder.
var ErrRefused = errors.New("o assistente não pôde responder essa pergunta; tente reformular")

func (a *Anthropic) Chat(ctx context.Context, req Request, onText func(string)) (string, error) {
	msgs := make([]anthropic.BetaMessageParam, len(req.Messages))
	for i, m := range req.Messages {
		block := anthropic.NewBetaTextBlock(m.Content)
		if m.Role == RoleAssistant {
			msgs[i] = anthropic.BetaMessageParam{Role: anthropic.BetaMessageParamRoleAssistant, Content: []anthropic.BetaContentBlockParamUnion{block}}
		} else {
			msgs[i] = anthropic.NewBetaUserMessage(block)
		}
	}
	stream := a.client.Beta.Messages.NewStreaming(ctx, anthropic.BetaMessageNewParams{
		Model:        anthropic.Model(a.model),
		MaxTokens:    16000,
		System:       []anthropic.BetaTextBlockParam{{Text: req.System}},
		Messages:     msgs,
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
			return "", err
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
			return "", &APIError{Status: apiErr.StatusCode, Message: apiErr.Error()}
		}
		return "", err
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return "", ErrRefused
	}
	answer := ""
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			answer += t.Text
		}
	}
	return answer, nil
}
