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
	"encoding/json"
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
	RoleTool      Role = "tool" // resultado de uma ferramenta pedida pelo modelo
)

type Message struct {
	Role    Role
	Content string
	// Assistente: ferramentas que o modelo pediu nesta resposta.
	ToolCalls []ToolCall
	// RoleTool: a qual pedido este resultado responde.
	ToolCallID, ToolName string
}

// Tool é uma função que o modelo pode pedir para o app executar (ex.: buscar
// lançamentos). Parameters é um JSON Schema do objeto de argumentos.
type Tool struct {
	Name, Description string
	Parameters        json.RawMessage
}

// ToolCall é um pedido do modelo para executar uma ferramenta.
type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage // objeto JSON
}

// Request é uma conversa completa a ser continuada pelo modelo.
type Request struct {
	System   string
	Messages []Message // alternando user/assistant (e tool), terminando em user ou tool
	Tools    []Tool    // opcional
}

// Reply é a resposta do modelo: texto, pedidos de ferramenta, ou os dois.
type Reply struct {
	Text      string
	ToolCalls []ToolCall
}

// Provider gera a próxima resposta da conversa. onText recebe cada pedaço de
// texto conforme ele chega (streaming); o retorno é a resposta completa.
type Provider interface {
	Chat(ctx context.Context, req Request, onText func(string)) (Reply, error)
}

// ToolsUnsupported diz se o erro é do modelo não aceitar ferramentas (muitos
// modelos locais pequenos não aceitam): aí o consultor pergunta sem elas.
func ToolsUnsupported(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status < 400 || apiErr.Status >= 500 {
		return false
	}
	msg := strings.ToLower(apiErr.Message)
	return strings.Contains(msg, "tool") || strings.Contains(msg, "function")
}

// args devolve os argumentos como objeto JSON válido (vazio vira {}).
func args(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage("{}")
	}
	return raw
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
// vem do contexto da requisição (fechar a página cancela).
var httpClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		// O Ollama só responde depois de carregar o modelo na memória. Num PC
		// com HD lento, um modelo de 7 GB pode levar vários minutos na 1ª vez.
		ResponseHeaderTimeout: 20 * time.Minute,
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
