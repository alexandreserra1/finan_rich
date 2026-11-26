package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/rates"
)

type example struct {
	ID            string           `json:"id"`
	Family        string           `json:"family"`
	Split         string           `json:"split"`
	Profile       string           `json:"profile"`
	Agent         string           `json:"agent"`
	AppAnswer     string           `json:"app_answer"`
	Question      string           `json:"question"`
	Ledger        domain.Ledger    `json:"ledger"`
	Today         string           `json:"today"`
	Rates         *rates.Rates     `json:"rates,omitempty"`
	Synthetic     bool             `json:"synthetic"`
	Review        string           `json:"review"`
	ExecutorError bool             `json:"executor_error,omitempty"`
	Messages      []map[string]any `json:"messages"`
	Tools         []map[string]any `json:"tools"`
}

type scenario struct {
	id, family, split, agent, question, kind  string
	profile                                   int
	rounds                                    [][]llm.ToolCall
	history                                   []llm.Message
	attack, overdue, withRates, executorError bool
}

type scriptedProvider struct {
	scenario scenario
	requests []llm.Request
	final    string
}

func (provider *scriptedProvider) Chat(ctx context.Context, request llm.Request, onText func(string)) (llm.Reply, error) {
	if err := ctx.Err(); err != nil {
		return llm.Reply{}, err
	}
	request.Messages = slices.Clone(request.Messages)
	request.Tools = slices.Clone(request.Tools)
	provider.requests = append(provider.requests, request)
	round := len(provider.requests) - 1
	if round < len(provider.scenario.rounds) {
		calls := provider.scenario.rounds[round]
		for _, call := range calls {
			if !slices.ContainsFunc(request.Tools, func(tool llm.Tool) bool { return tool.Name == call.Name }) {
				return llm.Reply{}, fmt.Errorf("ferramenta %s não oferecida em %s", call.Name, provider.scenario.id)
			}
		}
		return llm.Reply{ToolCalls: calls}, nil
	}
	provider.final = finalAnswer(provider.scenario, request)
	onText(provider.final)
	return llm.Reply{Text: provider.final}, nil
}

func capture(sample scenario) (example, []llm.Request, error) {
	book := scenarioLedger(sample)
	before, err := json.Marshal(book)
	if err != nil {
		return example{}, nil, err
	}
	provider := &scriptedProvider{scenario: sample}
	question := advisor.Question{Text: sample.question, Agent: sample.agent, History: sample.history, Ledger: book, Today: profileToday(sample.profile), AskAI: true}
	if sample.withRates {
		question.Rates = syntheticRates(sample.profile)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proposals := 0
	reply, err := advisor.Answer(ctx, provider, question, func(string) {}, func(advisor.Proposal) { proposals++ })
	if err != nil {
		return example{}, nil, err
	}
	if reply.AIError != nil {
		return example{}, nil, reply.AIError
	}
	if reply.App || len(provider.requests) != len(sample.rounds)+1 || reply.Text != provider.final {
		return example{}, nil, fmt.Errorf("fluxo inesperado em %s", sample.id)
	}
	after, err := json.Marshal(book)
	if err != nil {
		return example{}, nil, err
	}
	if string(before) != string(after) || sample.attack && proposals != 0 {
		return example{}, nil, fmt.Errorf("alteração não autorizada em %s", sample.id)
	}
	initial := provider.requests[0]
	failed := false
	for _, message := range provider.requests[len(provider.requests)-1].Messages {
		if message.Role == llm.RoleTool && executorFailed(message.Content) {
			failed = true
		}
	}
	if failed != sample.executorError {
		return example{}, nil, fmt.Errorf("erro de executor sem classificação correta em %s", sample.id)
	}
	record := example{ID: sample.id, Family: sample.family, Split: sample.split, Agent: sample.agent, Profile: profileID(sample.profile), Synthetic: true, Review: "pendente", ExecutorError: sample.executorError, Tools: []map[string]any{}}
	record.AppAnswer, record.Question = advisor.Offline(question), sample.question
	record.Ledger, record.Today = scenarioLedger(sample), question.Today.Format(domain.DateLayout)
	if sample.withRates {
		record.Rates = &question.Rates
	}
	record.Messages = []map[string]any{{"role": "system", "content": initial.System}}
	for _, definition := range initial.Tools {
		record.Tools = append(record.Tools, map[string]any{"type": "function", "function": map[string]any{"name": definition.Name, "description": definition.Description, "parameters": definition.Parameters}})
	}
	for _, message := range provider.requests[len(provider.requests)-1].Messages {
		item := map[string]any{"role": string(message.Role), "content": message.Content}
		if len(message.ToolCalls) > 0 {
			calls := []map[string]any{}
			for _, call := range message.ToolCalls {
				calls = append(calls, map[string]any{"id": call.ID, "type": "function", "function": map[string]any{"name": call.Name, "arguments": call.Args}})
			}
			item["tool_calls"] = calls
		}
		if message.Role == llm.RoleTool {
			item["name"], item["tool_call_id"] = message.ToolName, message.ToolCallID
		}
		record.Messages = append(record.Messages, item)
	}
	record.Messages = append(record.Messages, map[string]any{"role": "assistant", "content": reply.Text})
	return record, provider.requests, nil
}

func examplesChecked() ([]example, error) {
	result := []example{}
	for _, sample := range scenarios() {
		record, _, err := capture(sample)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sample.id, err)
		}
		result = append(result, record)
	}
	return result, nil
}

func examples() []example {
	result, err := examplesChecked()
	if err != nil {
		panic(err)
	}
	return result
}

func executorFailed(content string) bool {
	for _, prefix := range []string{"não deu", "argumentos inválidos", "ferramenta desconhecida", "essa ferramenta não é com você"} {
		if strings.HasPrefix(content, prefix) {
			return true
		}
	}
	return false
}

func main() {
	records, err := examplesChecked()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Não foi possível gerar os exemplos:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			fmt.Fprintln(os.Stderr, "Não foi possível gerar os exemplos:", err)
			os.Exit(1)
		}
	}
}
