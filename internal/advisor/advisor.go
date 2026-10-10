// Package advisor conversa com o Claude sobre as finanças do usuário.
//
// A cada pergunta enviamos, junto com o texto, um retrato atualizado dos dados
// (resumo do mês, divisão do salário, dívidas). O histórico é guardado no banco
// exatamente como foi enviado/recebido e reenviado sem alterações, o que mantém
// o cache de prompt da Anthropic funcionando e evita reprocessar a conversa toda.
package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/alexandreserra1/finan_rich/internal/store"
)

// ErrRefused indica que o modelo recusou responder a pergunta.
var ErrRefused = errors.New("o assistente não pôde responder essa pergunta; tente reformular")

const systemPrompt = `Você é o consultor financeiro pessoal do usuário dentro do app finan_rich, um controle de gastos individual.

Como agir:
- Responda em português do Brasil, de forma direta e prática, como um amigo que entende de finanças.
- Cada mensagem do usuário traz um bloco <dados_financeiros> com os números atuais do app. Baseie-se neles e cite valores concretos. Se faltar dado para responder, diga o que falta e peça ao usuário para lançar no app.
- Nunca invente números que não estão nos dados ou na conversa.
- Temas comuns: divisão do salário (ex.: 50/30/20 entre necessidades, desejos e investimentos), onde cortar gastos, reserva de emergência, como sair das dívidas (método avalanche = maior juros primeiro; bola de neve = menor saldo primeiro), renegociação, cartão de crédito e cheque especial.
- Juros de cartão rotativo e cheque especial costumam ser os mais caros: priorize-os quando aparecerem.
- Quando sugerir um plano, dê passos numerados com valores em R$ e prazos.
- Você não é um consultor de investimentos certificado: para escolher produtos específicos de investimento, explique os conceitos (liquidez, risco, prazo) sem recomendar um produto como certo.
- Use markdown simples (listas e **negrito**). Seja breve quando a pergunta for simples.`

type Advisor struct {
	client anthropic.Client
	model  string
}

// New cria o consultor. opts permite trocar a URL da API nos testes.
func New(apiKey, model string, opts ...option.RequestOption) *Advisor {
	opts = append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &Advisor{client: anthropic.NewClient(opts...), model: model}
}

// Turn é uma troca pergunta/resposta pronta para ser gravada no banco.
type Turn struct {
	User      store.ChatMessage
	Assistant store.ChatMessage
}

// Reply envia a pergunta com o histórico e o retrato financeiro atual. onText é
// chamado com cada pedaço de texto conforme a resposta chega (streaming).
func (a *Advisor) Reply(ctx context.Context, history []store.ChatMessage, question, snapshot string, onText func(string)) (Turn, error) {
	messages := make([]anthropic.BetaMessageParam, 0, len(history)+1)
	for _, h := range history {
		var m anthropic.BetaMessageParam
		if err := json.Unmarshal(h.Content, &m); err != nil {
			return Turn{}, fmt.Errorf("lendo histórico (mensagem %d): %w", h.ID, err)
		}
		messages = append(messages, m)
	}
	userMsg := anthropic.NewBetaUserMessage(
		anthropic.NewBetaTextBlock("<dados_financeiros>\n"+snapshot+"\n</dados_financeiros>"),
		anthropic.NewBetaTextBlock(question),
	)
	messages = append(messages, userMsg)

	stream := a.client.Beta.Messages.NewStreaming(ctx, anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 16000,
		System:    []anthropic.BetaTextBlockParam{{Text: systemPrompt}},
		Messages:  messages,
		// Cacheia automaticamente o histórico: cada nova pergunta só paga o trecho novo.
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortMedium},
		// Se o filtro de segurança recusar por engano, a Anthropic tenta de novo
		// automaticamente em outro modelo indicado por ela.
		Fallbacks: anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()},
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
	})
	defer stream.Close()

	var msg anthropic.BetaMessage
	for stream.Next() {
		event := stream.Current()
		if err := msg.Accumulate(event); err != nil {
			return Turn{}, err
		}
		if delta, ok := event.AsAny().(anthropic.BetaRawContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.BetaTextDelta); ok {
				onText(text.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Turn{}, describeError(err)
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return Turn{}, ErrRefused
	}

	answer := ""
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			answer += t.Text
		}
	}
	userJSON, err := json.Marshal(userMsg)
	if err != nil {
		return Turn{}, err
	}
	// ToParam preserva todos os blocos (inclusive os de raciocínio) para reenviar intactos.
	assistantJSON, err := json.Marshal(msg.ToParam())
	if err != nil {
		return Turn{}, err
	}
	return Turn{
		User:      store.ChatMessage{Role: "user", Display: question, Content: userJSON},
		Assistant: store.ChatMessage{Role: "assistant", Display: answer, Content: assistantJSON},
	}, nil
}

func describeError(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 401:
			return errors.New("ANTHROPIC_API_KEY inválida")
		case 429:
			return errors.New("limite de uso da API atingido; tente de novo em instantes")
		case 529, 500, 502, 503:
			return errors.New("a API da Anthropic está instável no momento; tente de novo em instantes")
		}
	}
	return err
}
