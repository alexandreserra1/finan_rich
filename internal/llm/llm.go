// Package llm abstrai o modelo de linguagem usado pelo consultor.
//
// Qualquer provedor implementa a interface Provider. Hoje existem três:
//   - Ollama: roda modelos abertos (Qwen, DeepSeek, Llama...) no próprio PC, grátis e privado.
//   - OpenAI-compatível: qualquer API no formato /v1/chat/completions (Groq, DeepSeek,
//     OpenRouter, LM Studio, a própria OpenAI...).
//   - Anthropic: modelos Claude (pago).
//
// Para adicionar um provedor novo, implemente Provider e registre-o em New.
package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

// Request é uma conversa completa a ser continuada pelo modelo.
type Request struct {
	System   string
	Messages []Message // alternando user/assistant, terminando em user
}

// Provider gera a próxima resposta da conversa. onText recebe cada pedaço de
// texto conforme ele chega (streaming); o retorno é a resposta completa.
type Provider interface {
	Chat(ctx context.Context, req Request, onText func(string)) (string, error)
}

// Tipos de provedor.
const (
	KindOllama    = "ollama"
	KindOpenAI    = "openai"
	KindAnthropic = "anthropic"
)

// Config descreve qual provedor usar e como acessá-lo.
type Config struct {
	Kind    string
	BaseURL string
	Model   string
	APIKey  string
}

// ErrNotConfigured indica que falta alguma informação para usar o provedor.
var ErrNotConfigured = errors.New("assistente de IA não configurado")

// New cria o provedor descrito em cfg.
func New(cfg Config) (Provider, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("%w: informe o modelo", ErrNotConfigured)
	}
	switch cfg.Kind {
	case KindOllama:
		return NewOllama(cfg.BaseURL, cfg.Model), nil
	case KindOpenAI:
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("%w: informe a URL da API", ErrNotConfigured)
		}
		return NewOpenAI(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	case KindAnthropic:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("%w: informe a chave da API da Anthropic", ErrNotConfigured)
		}
		return NewAnthropic(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("%w: provedor desconhecido %q", ErrNotConfigured, cfg.Kind)
	}
}

// httpClient é compartilhado pelos provedores HTTP. Sem timeout total, pois
// respostas em streaming de modelos locais podem levar minutos; o cancelamento
// vem do contexto da requisição.
var httpClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 5 * time.Minute, // modelo local pode demorar para carregar
	},
}

// APIError é um erro devolvido pelo servidor do modelo.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("erro HTTP %d do provedor de IA", e.Status)
	}
	return fmt.Sprintf("provedor de IA respondeu %d: %s", e.Status, e.Message)
}

// trimSlash remove a barra final para concatenar caminhos com segurança.
func trimSlash(s string) string { return strings.TrimRight(s, "/") }
