package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/rates"
)

func TestEntradaEstrita(t *testing.T) {
	for _, raw := range []string{
		`{"synthetic":true,"synthetic":false,"cases":[]}`,
		`{"synthetic":true,"app_answer":"verdade","cases":[]}`,
		`{"synthetic":true,"cases":[]} {}`,
	} {
		var input Fixtures
		if strictJSON([]byte(raw), &input) == nil {
			t.Fatal("aceitou entrada ambígua ou campo desconhecido")
		}
	}
}

func TestCriteriosNaoUsamRespostaDoAppComoVerdade(t *testing.T) {
	rules := []Criterion{{ID: "numero", Category: "dinheiro", Contains: "32,00"}, {ID: "seguro", Category: "seguranca", Forbids: "garantido"}}
	for _, status := range []string{"erro", "fallback", "incompleto"} {
		result := score("caso", "modelo", "R$ 32,00", status, rules)
		if result.Passed != 0 || result.Total != 2 {
			t.Fatalf("%s contou falha como acerto: %+v", status, result)
		}
	}
	result := score("caso", "modelo", "R$ 32,00 garantido", "ok", rules)
	if result.Passed != 1 || result.Total != 2 || result.Criteria[1].Pass {
		t.Fatalf("critérios independentes incorretos: %+v", result)
	}
}

type fakeProvider struct {
	request llm.Request
	err     error
}

func (p *fakeProvider) Chat(ctx context.Context, req llm.Request, emit func(string)) (llm.Reply, error) {
	p.request = req
	if p.err != nil {
		return llm.Reply{}, p.err
	}
	emit("Resposta sintética")
	return llm.Reply{Text: "Resposta sintética"}, ctx.Err()
}

func TestWrapperMudaSomenteSystemEReplayConferePedido(t *testing.T) {
	base := &fakeProvider{}
	wrapped := systemWrapper{provider: base, variant: "cauteloso"}
	request := llm.Request{System: "produção", Messages: []llm.Message{{Role: llm.RoleUser, Content: "pergunta"}}, ToolChoice: "required"}
	if _, err := wrapped.Chat(context.Background(), request, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if base.request.System == request.System {
		t.Fatal("wrapper não mudou instrução")
	}
	base.request.System = request.System
	before, _ := json.Marshal(request)
	after, _ := json.Marshal(base.request)
	if string(before) != string(after) {
		t.Fatal("wrapper mudou outros campos")
	}
	replay := &replayProvider{rounds: []Round{{RequestSHA256: digest(before), Complete: true, Reply: llm.Reply{Text: "coletada"}}}}
	if _, err := replay.Chat(context.Background(), request, func(string) {}); err != nil {
		t.Fatal(err)
	}
	replay.index = 0
	request.System += "alterado"
	if _, err := replay.Chat(context.Background(), request, func(string) {}); err == nil {
		t.Fatal("replay aceitou pedido divergente")
	}
}

func TestAnswerRealComFakeNaoAprovaFallback(t *testing.T) {
	question := advisor.Question{Text: "Como foi meu mês?", Agent: "otavio", Today: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)}
	prepared, err := appAnswer(context.Background(), question)
	if err != nil || !prepared.App || prepared.Text == "" {
		t.Fatalf("Prepared: %+v %v", prepared, err)
	}
	question.Text = "Explique meu orçamento com calma"
	offline, err := appAnswer(context.Background(), question)
	if err != nil || !offline.App || offline.Text != advisor.Offline(question) {
		t.Fatalf("Offline: %+v %v", offline, err)
	}
	question.AskAI = true
	for _, failure := range []error{nil, errors.New("falha sintética")} {
		provider := &fakeProvider{err: failure}
		answer, err := advisor.Answer(context.Background(), provider, question, func(string) {}, func(advisor.Proposal) {})
		status := answerStatus(context.Background(), answer, err)
		if failure == nil && status != "ok" || failure != nil && status != "fallback" {
			t.Fatalf("estado: %s", status)
		}
	}
}

func TestFamiliaNaoCruzaSplitsERegrasExigemHash(t *testing.T) {
	fixture := Fixtures{Synthetic: true, Cases: []Case{{ID: "um", Family: "familia", Split: "valid", Today: "2026-10-15", Agent: "otavio", Question: "pergunta", Ledger: &domain.Ledger{}}}}
	rules := Rules{Synthetic: true, FixturesSHA256: strings.Repeat("a", 64), Cases: []CaseRules{{ID: "um", Criteria: []Criterion{{ID: "um", Category: "seguranca", Forbids: "garantia"}}}}}
	if err := validate(fixture, rules, rules.FixturesSHA256); err != nil {
		t.Fatal(err)
	}
	if validate(fixture, rules, strings.Repeat("b", 64)) == nil {
		t.Fatal("aceitou regras de outros casos")
	}
	rules.Cases[0].Criteria[0].Contains = " "
	if validate(fixture, rules, rules.FixturesSHA256) == nil {
		t.Fatal("aceitou contains e forbids no mesmo critério")
	}
	rules.Cases[0].Criteria[0].Contains = ""
	second := fixture.Cases[0]
	second.ID, second.Split = "dois", "test"
	fixture.Cases = append(fixture.Cases, second)
	if validate(fixture, rules, rules.FixturesSHA256) == nil {
		t.Fatal("família vazou para teste")
	}
}

func TestReplayIncompletoNuncaContaTextoComoAcerto(t *testing.T) {
	fixture := Fixtures{Synthetic: true, Cases: []Case{{ID: "um", Family: "familia", Split: "valid", Today: "2026-10-15", Agent: "otavio", Question: "Explique meu orçamento com calma", Ledger: &domain.Ledger{}}}}
	rules := Rules{Cases: []CaseRules{{ID: "um", Criteria: []Criterion{{ID: "um", Category: "engenharia", Forbids: "texto ausente"}}}}}
	collection := &Collection{}
	results := evaluate(context.Background(), fixture, rules, "valid", "producao", collection, time.Second)
	if len(results) != 2 || results[1].Passed != 0 || results[1].Total != 1 || results[1].Status == "ok" {
		t.Fatalf("coleta ausente passou: %+v", results)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	results = evaluate(canceled, fixture, rules, "valid", "producao", collection, time.Second)
	if results[0].Passed != 0 || results[1].Passed != 0 {
		t.Fatal("cancelamento contou como acerto")
	}
}

func TestTaxasOpcionaisChegamAoAppEAoProvider(t *testing.T) {
	data := []byte(`{"synthetic":true,"cases":[{"id":"bcb","family":"taxas","split":"valid","ledger":{},"today":"2026-10-15","agent":"otavio","question":"Qual a Selic hoje?","rates":{"selicBP":1234,"cdiBP":1224,"ipcaBP":456,"date":"2026-10-14","ipcaMonth":"2026-09"}}]}`)
	var fixture Fixtures
	if err := strictJSON(data, &fixture); err != nil {
		t.Fatal(err)
	}
	rules := Rules{Synthetic: true, FixturesSHA256: digest(data), Cases: []CaseRules{{ID: "bcb", Criteria: []Criterion{{ID: "resposta", Category: "transporte", Contains: "Resposta sintética"}}}}}
	if err := validate(fixture, rules, digest(data)); err != nil {
		t.Fatal(err)
	}
	syntheticRates := rates.Rates{SelicBP: 1234, CDIBP: 1224, IPCABP: 456, Date: "2026-10-14", IPCAMonth: "2026-09"}
	question := advisor.Question{Text: fixture.Cases[0].Question, Agent: "otavio", Ledger: domain.Ledger{}, Today: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Rates: syntheticRates, AskAI: true}
	provider := &fakeProvider{}
	reply, err := advisor.Answer(context.Background(), provider, question, func(string) {}, func(advisor.Proposal) {})
	if err != nil || reply.App {
		t.Fatal("Provider fake não respondeu")
	}
	requestData, err := json.Marshal(provider.request)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range provider.request.Messages {
		if strings.Contains(message.Content, syntheticRates.Text()) {
			found = true
		}
	}
	if !found {
		t.Fatal("pedido do Provider não contém taxas com data")
	}
	collection := &Collection{Cases: []CollectedCase{{ID: "bcb", Rounds: []Round{{RequestSHA256: digest(requestData), Complete: true, Reply: llm.Reply{Text: reply.Text}}}}}}
	for _, collected := range []*Collection{nil, collection} {
		results := evaluate(context.Background(), fixture, rules, "valid", "producao", collected, time.Second)
		if results[0].Status != "ok" || results[0].AnswerSHA256 != digest([]byte(syntheticRates.Text())) {
			t.Fatal("app não recebeu as taxas da fixture")
		}
		if collected != nil && (len(results) != 2 || results[1].Status != "ok" || results[1].Passed != 1) {
			t.Fatalf("replay não enviou as taxas ao Provider: %+v", results)
		}
	}
	for _, replacement := range []struct{ old, value string }{
		{"2026-10-14", "2026-02-30"}, {"2026-10-14", "2026-1-14"},
		{"2026-09", "2026-13"}, {"2026-09", "2026-9"},
	} {
		invalid := []byte(strings.Replace(string(data), replacement.old, replacement.value, 1))
		if err := strictJSON(invalid, &fixture); err != nil {
			t.Fatal(err)
		}
		rules.FixturesSHA256 = digest(invalid)
		if validate(fixture, rules, digest(invalid)) == nil {
			t.Fatalf("aceitou data ou mês inválido: %s", replacement.value)
		}
	}
}

func TestCLIAppNaoSobrescreveNemAlteraReservado(t *testing.T) {
	t.Chdir(t.TempDir())
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"internal/advisor", "internal/domain", "internal/finance", "internal/llm", "internal/money", "internal/rates", "tools/ai-lab/reservedeval"} {
		write(filepath.Join(path, "fonte.go"), []byte("package exemplo\n"))
	}
	write("tools/ai-lab/models.json", []byte(`{"`+baseline+`":"`+baselineDigest+`"}`))
	write(".artifacts/reservado/original.jsonl", []byte("pacote de engenharia imutável\n"))
	fixture := Fixtures{Synthetic: true, Cases: []Case{{ID: "um", Family: "familia", Split: "valid", Today: "2026-10-15", Agent: "otavio", Question: "Explique meu orçamento com calma", Ledger: &domain.Ledger{}}}}
	second := fixture.Cases[0]
	second.ID, second.Family, second.Split = "dois", "outra-familia", "test"
	fixture.Cases = append(fixture.Cases, second)
	fixtureData, _ := json.Marshal(fixture)
	write(".artifacts/cases.json", fixtureData)
	rules := Rules{Synthetic: true, FixturesSHA256: digest(fixtureData), Cases: []CaseRules{{ID: "um", Criteria: []Criterion{{ID: "um", Category: "engenharia", Forbids: "marcador_de_engenharia_ausente"}}}}}
	rules.Cases = append(rules.Cases, CaseRules{ID: "dois", Criteria: rules.Cases[0].Criteria})
	rulesData, _ := json.Marshal(rules)
	write(".artifacts/rules.json", rulesData)
	args := []string{"--fixtures", ".artifacts/cases.json", "--rules", ".artifacts/rules.json", "--reserved-dir", ".artifacts/reservado"}
	before, err := treeHash(context.Background(), ".artifacts/reservado")
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := run(args, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	files, _ := filepath.Glob(".artifacts/ai-lab-reserved-runs/run-*/report.json")
	if len(files) != 2 {
		t.Fatal("não criou relatórios exclusivos")
	}
	data, _ := os.ReadFile(files[0])
	var report Report
	if err := strictJSON(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.ReservedBefore != before || report.ReservedAfter != before || report.Totals["app"].Total != 1 || report.Step5Complete || report.TrainingAllowed {
		t.Fatalf("relatório incorreto: %+v", report)
	}
	if strings.Contains(string(data), "Explique") || strings.Contains(string(data), "marcador_de_engenharia") {
		t.Fatal("relatório expôs conteúdo")
	}
	collection := Collection{Synthetic: true, Model: baseline, Digest: baselineDigest, Endpoint: "http://127.0.0.1:11434", FixturesSHA256: digest(fixtureData), RulesSHA256: digest(rulesData), Variant: "producao"}
	for _, item := range fixture.Cases {
		provider := &fakeProvider{}
		today, _ := time.Parse("2006-01-02", item.Today)
		question := advisor.Question{Text: item.Question, Agent: item.Agent, Ledger: *item.Ledger, Today: today, AskAI: true}
		reply, err := advisor.Answer(context.Background(), provider, question, func(string) {}, func(advisor.Proposal) {})
		if err != nil || reply.App {
			t.Fatal("fake não percorreu advisor.Answer")
		}
		requestData, _ := json.Marshal(provider.request)
		collection.Cases = append(collection.Cases, CollectedCase{ID: item.ID, Rounds: []Round{{RequestSHA256: digest(requestData), Complete: true, Reply: llm.Reply{Text: reply.Text}}}})
	}
	collectionData, _ := json.Marshal(collection)
	write(".artifacts/collected.json", collectionData)
	replayArgs := append(append([]string{}, args...), "--mode", "replay", "--allow-collected", "--collected", ".artifacts/collected.json")
	var output strings.Builder
	if err := run(replayArgs, &output); err != nil {
		t.Fatal(err)
	}
	selection := strings.TrimSpace(output.String())
	testArgs := append(append([]string{}, replayArgs...), "--split", "test")
	if run(testArgs, io.Discard) == nil {
		t.Fatal("test aceitou replay sem escolha no valid")
	}
	if err := run(append(testArgs, "--selection", selection), io.Discard); err != nil {
		t.Fatal(err)
	}
	if run(append(append([]string{}, testArgs...), "--selection", selection, "--variant", "cauteloso"), io.Discard) == nil {
		t.Fatal("test aceitou troca de variante")
	}
	for _, extra := range [][]string{{"--mode", "infer"}, {"--mode", "replay"}, {"--timeout", "10m"}, {"--fixtures", "tools/ai-lab/models.json"}, {"--reserved-dir", ".artifacts"}} {
		if run(append(append([]string{}, args...), extra...), io.Discard) == nil {
			t.Fatalf("aceitou argumentos proibidos: %v", extra)
		}
	}
	after, err := treeHash(context.Background(), ".artifacts/reservado")
	if err != nil || after != before {
		t.Fatal("reservado foi alterado")
	}
	write(".artifacts/reservado/mudou.txt", []byte("novo"))
	changed, err := treeHash(context.Background(), ".artifacts/reservado")
	if err != nil || changed == before {
		t.Fatal("hash não detectou arquivo adicionado")
	}
	if err := os.Symlink("original.jsonl", ".artifacts/reservado/link"); err != nil {
		t.Fatal(err)
	}
	if _, err := treeHash(context.Background(), ".artifacts/reservado"); err == nil {
		t.Fatal("aceitou link no reservado")
	}
}
