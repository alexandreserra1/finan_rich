package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

type scriptedCollection struct {
	replies []llm.Reply
	rounds  []Round
}

func (p *scriptedCollection) Chat(ctx context.Context, req llm.Request, emit func(string)) (llm.Reply, error) {
	if len(p.rounds) >= len(p.replies) {
		return llm.Reply{}, fmt.Errorf("roteiro esgotado")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return llm.Reply{}, err
	}
	reply := p.replies[len(p.rounds)]
	p.rounds = append(p.rounds, Round{RequestSHA256: digest(raw), Complete: true, Reply: reply})
	emit(reply.Text)
	return reply, ctx.Err()
}

func structuredRules(t *testing.T, criteria string) Rules {
	t.Helper()
	var rules Rules
	if err := strictJSON([]byte(`{"synthetic":true,"fixtures_sha256":"hash","cases":[{"id":"um","criteria":`+criteria+`}]}`), &rules); err != nil {
		t.Fatal(err)
	}
	return rules
}

func structuredFixture() Fixtures {
	return Fixtures{Synthetic: true, Cases: []Case{{ID: "um", Family: "familia", Split: "valid", Today: "2026-10-15", Agent: "lia", Question: "Corrija o gasto Padaria para 32 reais", Ledger: &domain.Ledger{
		Categories: []domain.Category{{Meta: domain.Meta{ID: "cat"}, Name: "Mercado", Bucket: domain.BucketNecessidades}},
		Transactions: []domain.Transaction{
			{Meta: domain.Meta{ID: "origem", UpdatedAt: 1}, Kind: domain.KindSaida, Description: "Padaria", AmountCents: 3100, CategoryID: "cat", Date: "2026-10-14"},
			{Meta: domain.Meta{ID: "outro", UpdatedAt: 1}, Kind: domain.KindSaida, Description: "Outro", AmountCents: 3100, CategoryID: "cat", Date: "2026-10-14"},
		},
	}}}}
}

func replayScript(t *testing.T, fixtures Fixtures, rules Rules, calls ...llm.ToolCall) Result {
	t.Helper()
	provider := &scriptedCollection{}
	for _, call := range calls {
		provider.replies = append(provider.replies, llm.Reply{ToolCalls: []llm.ToolCall{call}})
	}
	provider.replies = append(provider.replies, llm.Reply{Text: "Confira o cartão antes de confirmar."})
	item := fixtures.Cases[0]
	today, _ := time.Parse("2006-01-02", item.Today)
	question := advisor.Question{Text: item.Question, Agent: item.Agent, Ledger: *item.Ledger, Today: today, AskAI: true}
	if _, err := advisor.Answer(context.Background(), provider, question, func(string) {}, func(advisor.Proposal) {}); err != nil {
		t.Fatal(err)
	}
	collection := &Collection{Cases: []CollectedCase{{ID: item.ID, Rounds: provider.rounds}}}
	return evaluate(context.Background(), fixtures, rules, "valid", "producao", collection, time.Second)[1]
}

func TestAvaliaChamadasArgumentosEOrigemDaProposta(t *testing.T) {
	rules := structuredRules(t, `[
		{"id":"chamadas","category":"ferramentas","calls":[{"name":"buscar_lancamentos","args":{"texto":"Padaria"}},{"name":"alterar_lancamento","args":{"id":"origem","valor":"32"}}]},
		{"id":"cartoes","category":"propostas","proposals":[{"action":"alterar","type":"transaction","record":{"id":"origem","amountCents":3200,"date":"2026-10-14"},"before":{"id":"origem","amountCents":3100}}]}
	]`)
	fixtures := structuredFixture()
	if err := validate(fixtures, rules, "hash"); err != nil {
		t.Fatal(err)
	}
	search := llm.ToolCall{ID: "busca", Name: "buscar_lancamentos", Args: json.RawMessage(`{"texto":"Padaria"}`)}
	edit := llm.ToolCall{ID: "edicao", Name: "alterar_lancamento", Args: json.RawMessage(`{"valor":"32", "id":"origem"}`)}
	if result := replayScript(t, fixtures, rules, search, edit); result.Status != "ok" || result.Passed != 2 {
		t.Fatalf("chamada e proposta corretas não passaram: %+v", result)
	}
	for _, mutation := range []struct {
		name  string
		calls []llm.ToolCall
	}{
		{"sem chamadas", nil},
		{"sem proposta", []llm.ToolCall{search}},
		{"id errado", []llm.ToolCall{search, {ID: "edicao", Name: edit.Name, Args: json.RawMessage(`{"id":"outro","valor":"32"}`)}}},
		{"centavos errados", []llm.ToolCall{search, {ID: "edicao", Name: edit.Name, Args: json.RawMessage(`{"id":"origem","valor":"32,01"}`)}}},
		{"data errada", []llm.ToolCall{search, {ID: "edicao", Name: edit.Name, Args: json.RawMessage(`{"id":"origem","valor":"32","data":"2026-10-13"}`)}}},
		{"ferramenta errada", []llm.ToolCall{search, {ID: "edicao", Name: "apagar_lancamento", Args: json.RawMessage(`{"id":"origem"}`)}}},
		{"cartao extra", []llm.ToolCall{search, edit, {ID: "extra", Name: edit.Name, Args: json.RawMessage(`{"id":"outro","valor":"32"}`)}}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if result := replayScript(t, fixtures, rules, mutation.calls...); result.Passed != 0 {
				t.Fatalf("texto igual ocultou erro estruturado: %+v", result)
			}
		})
	}
	t.Run("ordem das chamadas", func(t *testing.T) {
		result := replayScript(t, fixtures, rules, edit, search)
		if result.Criteria[0].Pass || !result.Criteria[1].Pass {
			t.Fatalf("não distinguiu ordem das chamadas e dados do cartão: %+v", result)
		}
	})
	t.Run("estado anterior errado", func(t *testing.T) {
		(*rules.Cases[0].Criteria[1].Proposals)[0].Before = json.RawMessage(`{"id":"origem","amountCents":3000}`)
		result := replayScript(t, fixtures, rules, search, edit)
		if !result.Criteria[0].Pass || result.Criteria[1].Pass {
			t.Fatalf("não conferiu o estado anterior: %+v", result)
		}
	})
}

func TestAvaliaErroEsperadoERecuperacao(t *testing.T) {
	fixtures := structuredFixture()
	fixtures.Cases[0].Agent = "otavio"
	fixtures.Cases[0].Question = "Antes de registrar, liste os próximos vencimentos"
	rules := structuredRules(t, `[
		{"id":"recuperacao","category":"ferramentas","calls":[{"name":"proximos_vencimentos","args":{"dias":"0"},"error":true},{"name":"proximos_vencimentos","args":{"dias":"20"}}]},
		{"id":"sem-cartoes","category":"propostas","proposals":[]}
	]`)
	bad := llm.ToolCall{ID: "erro", Name: "proximos_vencimentos", Args: json.RawMessage(`{"dias":"0"}`)}
	good := llm.ToolCall{ID: "corrigida", Name: bad.Name, Args: json.RawMessage(`{"dias":"20"}`)}
	if result := replayScript(t, fixtures, rules, bad, good); result.Status != "ok" || result.Passed != 2 {
		t.Fatalf("recuperação de erro conhecido não foi avaliada: %+v", result)
	}
	(*rules.Cases[0].Criteria[0].Calls)[0].Error = false
	if result := replayScript(t, fixtures, rules, bad, good); result.Status != "ok" || result.Criteria[0].Pass {
		t.Fatalf("erro inesperado passou como sucesso: %+v", result)
	}
}

func TestRejeitaCriteriosEstruturadosAmbiguos(t *testing.T) {
	for _, criterion := range []string{
		`{"contains":"texto","calls":[]}`,
		`{"calls":[],"proposals":[]}`,
		`{"calls":[{"name":"buscar_lancamentos","args":null}]}`,
		`{"calls":[{"name":"buscar_lancamentos","args":[]}]}`,
		`{"proposals":[{"action":"alterar","type":"transaction","record":{}}]}`,
		`{"proposals":[{"action":"alterar","type":"transaction","record":{"id":"origem"}}]}`,
		`{"proposals":[{"action":"alterar","type":"transaction","record":{"id":"origem"},"before":{"id":"outro"}}]}`,
	} {
		rules := structuredRules(t, `[{"id":"regra","category":"engenharia",`+criterion[1:]+`]`)
		if validate(structuredFixture(), rules, "hash") == nil {
			t.Fatalf("aceitou critério ambíguo ou sem origem: %s", criterion)
		}
	}
}

func TestArgumentosEstritosPreservamInteiros(t *testing.T) {
	for _, test := range []struct {
		want, got string
		pass      bool
	}{
		{`{"valor":9007199254740993}`, `{"valor":9007199254740993}`, true},
		{`{"valor":9007199254740993}`, `{"valor":9007199254740992}`, false},
		{`{"valor":"32"}`, `{"valor":32}`, false},
		{`{"valor":"32"}`, `{"valor":"32","extra":true}`, false},
		{`{"valor":null}`, `{}`, false},
		{`{}`, `null`, false},
		{`{}`, `[]`, false},
		{`{"valor":"32"}`, `{"valor":"31","valor":"32"}`, false},
	} {
		if matchesObject(json.RawMessage(test.want), json.RawMessage(test.got), false) != test.pass {
			t.Fatalf("comparação incorreta: %s contra %s", test.want, test.got)
		}
	}
}

func TestReconheceFamiliasDeErroDoExecutor(t *testing.T) {
	for _, text := range []string{"não deu: informe quantos dias olhar", "não deu para preparar: categoria ausente", "argumentos inválidos: valor", "ferramenta desconhecida: inexistente", "essa ferramenta não é com você: criar_meta"} {
		if !executorFailed(text) {
			t.Fatalf("erro do executor ignorado: %s", text)
		}
	}
	if executorFailed("Pronto na tela para o usuário confirmar") {
		t.Fatal("sucesso marcado como erro")
	}
}

func TestReplayRejeitaIdentidadeAmbiguaEArgumentosInvalidos(t *testing.T) {
	for _, calls := range [][]llm.ToolCall{
		{{ID: "", Name: "buscar_lancamentos", Args: json.RawMessage(`{}`)}},
		{{ID: "repetido", Name: "buscar_lancamentos", Args: json.RawMessage(`{}`)}, {ID: "repetido", Name: "buscar_lancamentos", Args: json.RawMessage(`{}`)}},
		{{ID: "um", Name: "buscar_lancamentos", Args: json.RawMessage(`null`)}},
		{{ID: "um", Name: "buscar_lancamentos", Args: json.RawMessage(`{"texto":"a","texto":"b"}`)}},
	} {
		request := llm.Request{Tools: []llm.Tool{{Name: "buscar_lancamentos"}}}
		raw, _ := json.Marshal(request)
		provider := &replayProvider{rounds: []Round{{RequestSHA256: digest(raw), Complete: true, Reply: llm.Reply{ToolCalls: calls}}}}
		if _, err := provider.Chat(context.Background(), request, func(string) {}); err == nil || !provider.failed {
			t.Fatalf("aceitou chamadas ambíguas: %+v", calls)
		}
	}
}

func TestErroDePropostaPermiteNovaChamada(t *testing.T) {
	fixtures := structuredFixture()
	rules := structuredRules(t, `[
		{"id":"chamadas","category":"ferramentas","calls":[{"name":"alterar_lancamento","args":{"id":"ausente","valor":"32"},"error":true},{"name":"alterar_lancamento","args":{"id":"origem","valor":"32"}}]},
		{"id":"propostas","category":"propostas","proposals":[{"action":"alterar","type":"transaction","record":{"id":"origem","amountCents":3200},"before":{"id":"origem","amountCents":3100}}]}
	]`)
	bad := llm.ToolCall{ID: "erro", Name: "alterar_lancamento", Args: json.RawMessage(`{"id":"ausente","valor":"32"}`)}
	good := llm.ToolCall{ID: "corrigida", Name: bad.Name, Args: json.RawMessage(`{"id":"origem","valor":"32"}`)}
	if result := replayScript(t, fixtures, rules, bad, good); result.Status != "ok" || result.Passed != 2 || result.CallCount != 2 || result.ProposalCount != 1 {
		t.Fatalf("recuperação da proposta não foi avaliada: %+v", result)
	}
}

func TestAvaliaDoisCartoesDoPagamentoSemPersistir(t *testing.T) {
	fixtures := structuredFixture()
	item := &fixtures.Cases[0]
	item.Agent, item.Question = "otavio", "Registre o pagamento de 100 reais na dívida Cartão"
	item.Ledger.Debts = []domain.Debt{{Meta: domain.Meta{ID: "divida", UpdatedAt: 1}, Name: "Cartão", BalanceCents: 240000, MinPaymentCents: 10000}}
	item.Ledger.Categories = append(item.Ledger.Categories, domain.Category{Meta: domain.Meta{ID: "pagamentos"}, Name: domain.DebtPaymentCategory, Bucket: domain.BucketNecessidades})
	before, _ := json.Marshal(item.Ledger)
	rules := structuredRules(t, `[
		{"id":"pagamento","category":"ferramentas","calls":[{"name":"registrar_pagamento","args":{"divida":"Cartão","valor":"100"}}]},
		{"id":"dois-cartoes","category":"propostas","proposals":[
			{"action":"alterar","type":"debt","record":{"id":"divida","balanceCents":230000},"before":{"id":"divida","balanceCents":240000}},
			{"action":"criar","type":"transaction","record":{"amountCents":10000,"date":"2026-10-15","categoryId":"pagamentos"}}
		]}
	]`)
	if err := validate(fixtures, rules, "hash"); err != nil {
		t.Fatal(err)
	}
	call := llm.ToolCall{ID: "pagamento", Name: "registrar_pagamento", Args: json.RawMessage(`{"divida":"Cartão","valor":"100"}`)}
	result := replayScript(t, fixtures, rules, call)
	if result.Status != "ok" || result.Passed != 2 || result.ProposalCount != 2 || result.ProposalsSHA256 == "" {
		t.Fatalf("não conferiu os dois cartões: %+v", result)
	}
	proposals := rules.Cases[0].Criteria[1].Proposals
	*proposals = (*proposals)[:1]
	if result := replayScript(t, fixtures, rules, call); result.Criteria[1].Pass {
		t.Fatal("aceitou somente um cartão esperado para dois emitidos")
	}
	after, _ := json.Marshal(item.Ledger)
	if string(before) != string(after) {
		t.Fatal("avaliação alterou o livro inicial")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"calls", "proposals", "record", "args"} {
		if _, exists := decoded[field]; exists {
			t.Fatalf("relatório expôs dados estruturados em %s", field)
		}
	}
}

func TestAusenciaExplicitaNaoPassaEmReplayIncompleto(t *testing.T) {
	rules := structuredRules(t, `[{"id":"sem-chamadas","category":"ferramentas","calls":[]},{"id":"sem-cartoes","category":"propostas","proposals":[]}]`)
	if err := validate(structuredFixture(), rules, "hash"); err != nil {
		t.Fatal(err)
	}
	if result := replayScript(t, structuredFixture(), rules); result.Status != "ok" || result.Passed != 2 {
		t.Fatalf("ausência legítima não passou: %+v", result)
	}
	for _, status := range []string{"erro", "fallback", "incompleto"} {
		if result := score("um", "modelo", "texto", status, rules.Cases[0].Criteria); result.Passed != 0 {
			t.Fatalf("%s contou ausência como acerto: %+v", status, result)
		}
	}
}

func TestRubricaNaoPodeOmitirDimensoesEstruturadas(t *testing.T) {
	text := Criterion{ID: "texto", Category: "texto", Contains: "Confira"}
	call := llm.ToolCall{ID: "edicao", Name: "alterar_lancamento", Args: json.RawMessage(`{"id":"origem","valor":"32"}`)}
	complete := structuredRules(t, `[
		{"id":"chamadas","category":"ferramentas","calls":[{"name":"alterar_lancamento","args":{"id":"origem","valor":"32"}}]},
		{"id":"cartoes","category":"propostas","proposals":[{"action":"alterar","type":"transaction","record":{"id":"origem","amountCents":3200},"before":{"id":"origem","amountCents":3100}}]}
	]`)
	for _, test := range []struct {
		name     string
		criteria []Criterion
	}{
		{"somente texto", []Criterion{text}},
		{"sem propostas", []Criterion{text, complete.Cases[0].Criteria[0]}},
		{"sem chamadas", []Criterion{text, complete.Cases[0].Criteria[1]}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rules := complete
			rules.Cases = []CaseRules{{ID: "um", Criteria: test.criteria}}
			result := replayScript(t, structuredFixture(), rules, call)
			if result.Status != "rubrica_incompleta" || result.Passed != 0 {
				t.Fatalf("rubrica omissa aprovou amostra estruturada: %+v", result)
			}
			report := Report{Results: []Result{result}}
			aggregate(&report)
			if report.Totals["modelo"].FailedCases != 1 {
				t.Fatal("rubrica incompleta não reprovou o caso agregado")
			}
		})
	}
	if result := replayScript(t, structuredFixture(), complete, call); result.Status != "ok" || result.Passed != 2 {
		t.Fatalf("rubrica completa deixou de passar: %+v", result)
	}
}

func TestRubricaExigeAusenciaExplicitaMesmoSemEmissao(t *testing.T) {
	rules := structuredRules(t, `[{"id":"sem-chamadas","category":"ferramentas","calls":[]}]`)
	if result := replayScript(t, structuredFixture(), rules); result.Status != "rubrica_incompleta" || result.Passed != 0 {
		t.Fatalf("dimensão declarada permitiu esquecer propostas: %+v", result)
	}
	textOnly := []Criterion{{ID: "texto", Category: "texto", Contains: "Confira"}}
	if result := score("um", "modelo", "Confira", "ok", textOnly); result.Status != "ok" || result.Passed != 1 {
		t.Fatalf("caso puramente textual perdeu compatibilidade: %+v", result)
	}
	rules.Cases[0].Criteria = textOnly
	search := llm.ToolCall{ID: "busca", Name: "buscar_lancamentos", Args: json.RawMessage(`{"texto":"Padaria"}`)}
	if result := replayScript(t, structuredFixture(), rules, search); result.Status != "rubrica_incompleta" || result.Passed != 0 || result.ProposalCount != 0 {
		t.Fatalf("chamada somente de leitura passou sem rubrica estruturada: %+v", result)
	}
}
