//go:build js

package llm

import (
	"context"
	"errors"
)

// No celular (WebAssembly) o SDK da Anthropic fica de fora para o app não
// crescer; o Claude continua disponível pelo computador.
type Anthropic struct{}

func NewAnthropic(apiKey, model, baseURL string) *Anthropic { return &Anthropic{} }

func (*Anthropic) Chat(context.Context, Request, func(string)) (Reply, error) {
	return Reply{}, errors.New("o Claude só está disponível pelo computador; no celular, use Groq ou Gemini")
}
