package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

func TestCorpusPassaNoValidadorDoLaboratorio(t *testing.T) {
	payload, err := json.Marshal(examples())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", "-c", "import json,sys; from prepare_dataset import validate; from review_corpus import quality_findings,safety_findings; rows=json.load(sys.stdin); splits=validate(rows); checks=[quality_findings(rows,splits),safety_findings(rows)]; assert all(check['ok'] for check in checks), checks")
	command.Dir = ".."
	command.Stdin = bytes.NewReader(payload)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("gerador e validadores divergem: %v\n%s", err, output)
	}
}

func TestCorpusReproduzivelComFamiliasSeparadas(t *testing.T) {
	first := examples()
	second := examples()
	left, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := json.Marshal(second)
	if err != nil || string(left) != string(right) {
		t.Fatal("o corpus mudou entre execuções idênticas", err)
	}
	if len(first) < 24 {
		t.Fatalf("esperados ao menos 24 exemplos, recebidos %d", len(first))
	}
	groups := map[string]string{}
	ids := map[string]bool{}
	for _, example := range first {
		if ids[example.ID] || example.Review != "pendente" || !example.Synthetic {
			t.Fatal("identificação ou revisão inválida", example.ID)
		}
		ids[example.ID] = true
		if previous, exists := groups[example.Family]; exists && previous != example.Split {
			t.Fatal("família em conjuntos diferentes", example.Family)
		}
		groups[example.Family] = example.Split
		if len(example.Messages) < 3 || example.Messages[len(example.Messages)-1]["role"] != "assistant" {
			t.Fatal("conversa incompleta", example.ID)
		}
		for _, message := range example.Messages {
			if message["role"] == "tool" {
				content, _ := message["content"].(string)
				if content == "" {
					t.Fatal("ferramenta não produziu resultado válido", example.ID, content)
				}
			}
		}
	}
	if len(groups) < 12 {
		t.Fatal("esperadas ao menos doze famílias", len(groups))
	}
}

func TestLimiteDaSimulacaoNaoAfirmaCausaUnica(t *testing.T) {
	debts := []domain.Debt{{Name: "Prazo longo fictício", BalanceCents: 60000000, MonthlyRateBP: 0, MinPaymentCents: 1}}
	plan := finance.PayoffSchedule(debts, 45000, true)
	if plan.OK || plan.InterestCents != 0 || plan.Months != 600 {
		t.Fatalf("não reproduziu limite sem juros: %+v", plan)
	}
	answer := debtSimulationAnswer(plan, finance.PayoffSchedule(debts, 0, true), 1, profileToday(0))
	if strings.Contains(answer, "juros superam") || !strings.Contains(answer, "limite") || !strings.Contains(answer, "hipóteses") {
		t.Fatalf("limite tratado como causa única: %s", answer)
	}
}

func TestSimulacaoConfereComGoEFerramentaEmTodosPerfis(t *testing.T) {
	for profile := 0; profile < 6; profile++ {
		book, today := fixture(profile), profileToday(profile)
		debts := book.ActiveDebts()
		plan, base := finance.PayoffSchedule(debts, 45000, true), finance.PayoffSchedule(debts, 0, true)
		var minimum int64
		for _, debt := range debts {
			minimum += debt.MinPaymentCents
		}
		answer := debtSimulationAnswer(plan, base, minimum, today)
		response := advisor.NewTools(book, today, nil).Run(call("simular_dividas", map[string]any{"valor_extra": "450,00", "ordem": "maior juros"}))
		if !plan.OK || !base.OK {
			t.Fatalf("perfil %d não exercita comparação concluída", profile)
		}
		for _, value := range []string{money.Format(plan.InterestCents), money.Format(minimum + 45000), money.Format(base.InterestCents - plan.InterestCents), finance.Meses(base.Months - plan.Months)} {
			if !strings.Contains(answer, value) || !strings.Contains(response, value) {
				t.Errorf("perfil %d: %s diverge entre conclusão e ferramenta", profile, value)
			}
		}
		arrival := domain.NewMonth(today.Year(), today.Month()).AddDate(0, plan.Months, 0)
		if !strings.Contains(answer, arrival.Format("01/2006")) || !strings.Contains(response, domain.NewMonth(arrival.Year(), arrival.Month()).Label()) {
			t.Errorf("prazo divergente no perfil %d", profile)
		}
	}
}

func TestVencimentosConferemComFerramentaNosPerfisUsados(t *testing.T) {
	for _, sample := range scenarios() {
		if sample.kind != "vencimentos" || sample.overdue {
			continue
		}
		record, _, err := capture(sample)
		if err != nil {
			t.Fatal(err)
		}
		answer := record.Messages[len(record.Messages)-1]["content"].(string)
		tool := advisor.NewTools(scenarioLedger(sample), profileToday(sample.profile), nil).Run(call("proximos_vencimentos", map[string]any{"dias": "20"}))
		var total int64
		for _, line := range strings.Split(tool, "\n") {
			if !strings.HasPrefix(line, "- ") || strings.Contains(line, "Orçamento") {
				continue
			}
			amount := regexp.MustCompile(`R\$ ([\d.]+,\d{2})`).FindStringSubmatch(line)
			date := regexp.MustCompile(`em (\d{2}/\d{2})`).FindStringSubmatch(line)
			if len(amount) != 2 || len(date) != 2 {
				t.Fatalf("resultado sem valor/data: %s", line)
			}
			value, err := money.Parse(amount[1])
			if err != nil {
				t.Fatal(err)
			}
			total += value
			name := strings.SplitN(strings.TrimPrefix(line, "- "), ":", 2)[0]
			if !strings.Contains(answer, name) || !strings.Contains(answer, date[1]) || !strings.Contains(answer, money.Format(value)) {
				t.Errorf("%s omite vencimento %s", sample.id, line)
			}
		}
		if !strings.Contains(answer, "**"+money.Format(total)+"**") {
			t.Errorf("%s total diverge: %s", sample.id, answer)
		}
	}
}

func TestPerfisPerguntasEFamiliasSemVazamento(t *testing.T) {
	profiles, questions := map[string]string{}, map[string]string{}
	for _, record := range examples() {
		if previous, exists := profiles[record.Profile]; exists && previous != record.Split {
			t.Errorf("perfil %s cruza conjuntos", record.Profile)
		}
		profiles[record.Profile] = record.Split
		if previous, exists := questions[record.Question]; exists && previous != record.Split {
			t.Errorf("pergunta repetida entre conjuntos: %s", record.Question)
		}
		questions[record.Question] = record.Split
	}
	if len(profiles) != 6 {
		t.Fatalf("esperados seis perfis distintos: %v", profiles)
	}
	for index := 0; index < 6; index++ {
		book := fixture(index)
		expected := index/2 + 1
		if index == 0 {
			expected = 2
		}
		if len(book.Debts) != expected {
			t.Errorf("perfil sem variedade estrutural: %d", index)
		}
	}
}

func TestLivroCoerenteOuAtrasoExplicito(t *testing.T) {
	for _, sample := range scenarios() {
		book := scenarioLedger(sample)
		due := finance.RecurringDue(book, profileToday(sample.profile))
		if !sample.overdue && len(due) != 0 {
			t.Errorf("%s tem recorrência vencida sem lançamento", sample.id)
		}
		month := profileToday(sample.profile).Format("2006-01")
		for _, debt := range book.Debts {
			var paid int64
			for _, transaction := range book.Transactions {
				if strings.HasPrefix(transaction.Date, month) && transaction.Description == "Pagamento: "+debt.Name {
					paid += transaction.AmountCents
				}
			}
			if !sample.overdue && paid < debt.MinPaymentCents {
				t.Errorf("%s sem parcela paga de %s", sample.id, debt.Name)
			}
		}
		if sample.overdue {
			record, _, err := capture(sample)
			if err != nil {
				t.Fatal(err)
			}
			answer := record.Messages[len(record.Messages)-1]["content"].(string)
			if len(due) == 0 || !strings.Contains(answer, "atraso") || !strings.Contains(answer, "Se já pagou") {
				t.Errorf("%s omite atraso ou conciliação", sample.id)
			}
		}
	}
}

func TestCapturaPreservaContratoPublicoEAuditoria(t *testing.T) {
	for _, sample := range scenarios() {
		record, requests, err := capture(sample)
		if err != nil {
			t.Fatal(err)
		}
		if record.Messages[0]["content"] != requests[0].System {
			t.Errorf("prompt alterado em %s", sample.id)
		}
		if len(record.Tools) != len(requests[0].Tools) {
			t.Fatalf("cardápio alterado em %s", sample.id)
		}
		for index, definition := range record.Tools {
			function := definition["function"].(map[string]any)
			production := requests[0].Tools[index]
			if function["name"] != production.Name || function["description"] != production.Description || !reflect.DeepEqual(function["parameters"], production.Parameters) {
				t.Errorf("contrato de ferramenta alterado em %s", sample.id)
			}
		}
		date, err := time.Parse(domain.DateLayout, record.Today)
		if err != nil {
			t.Fatal(err)
		}
		date = date.Add(12 * time.Hour)
		want := advisor.Offline(advisor.Question{Text: record.Question, Agent: record.Agent, Today: date, Ledger: record.Ledger, AskAI: true})
		if sample.withRates {
			want = advisor.Offline(advisor.Question{Text: record.Question, Agent: record.Agent, Today: date, Ledger: record.Ledger, AskAI: true, Rates: syntheticRates(sample.profile)})
		}
		if want != record.AppAnswer || record.Profile != profileID(sample.profile) {
			t.Errorf("auditoria divergente em %s", sample.id)
		}
		if len(requests) != len(sample.rounds)+1 {
			t.Errorf("rodadas ausentes em %s", sample.id)
		}
		for _, message := range record.Messages {
			if message["role"] == "tool" && executorFailed(message["content"].(string)) && !record.ExecutorError {
				t.Errorf("%s treina erro como sucesso", sample.id)
			}
		}
	}
}

func TestContrapontosERecusasContextuais(t *testing.T) {
	for _, record := range examples() {
		answer := record.Messages[len(record.Messages)-1]["content"].(string)
		switch record.ID {
		case "dados-insuficientes-2":
			for _, word := range []string{"contrato", "extrato", "CET", "mesma dívida", "duplicidade"} {
				if !strings.Contains(answer, word) {
					t.Errorf("%s omite %s", record.ID, word)
				}
			}
		case "injecao-instrucoes-1":
			if !strings.Contains(answer, "Não tenho acesso") {
				t.Error("recusa sugere acesso a segredos")
			}
		case "taxa-sem-fonte-1":
			if !strings.Contains(answer, "Banco Central") {
				t.Error("fonte Selic ausente")
			}
		case "taxa-sem-fonte-2":
			if !strings.Contains(answer, "B3") {
				t.Error("fonte DI ausente")
			}
		}
		if strings.HasPrefix(record.ID, "promessa-retorno") && (!strings.Contains(answer, "golpe") || !strings.Contains(answer, "reserva")) {
			t.Errorf("%s sem alerta contextual", record.ID)
		}
		if record.ExecutorError && !strings.Contains(strings.ToLower(answer), "falhou") && !strings.Contains(answer, "Não foi possível") {
			t.Errorf("%s não assume erro", record.ID)
		}
		if strings.HasSuffix(record.ID, "conta-pronta") {
			if len(record.Tools) != 0 || len(record.Messages) != 3 || !strings.Contains(record.Messages[1]["content"].(string), "Contas prontas") {
				t.Errorf("%s não exercita número pronto sem chamada", record.ID)
			}
		}
	}
}

func TestProvedorFakeRecusaFerramentaNaoOferecidaECancelamento(t *testing.T) {
	provider := &scriptedProvider{scenario: scenario{id: "proibida", rounds: [][]llm.ToolCall{{call("apagar_lancamento", map[string]any{"id": "fic-1"})}}}}
	if _, err := provider.Chat(context.Background(), llm.Request{}, func(string) {}); err == nil {
		t.Fatal("fake aceitou ferramenta ausente")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Chat(ctx, llm.Request{}, func(string) {}); err != context.Canceled {
		t.Fatalf("cancelamento ignorado: %v", err)
	}
}

func TestMetricasDoGerador(t *testing.T) {
	counts := map[string]int{}
	menus := map[string]map[string]bool{}
	calls, errors, histories, multiround := 0, 0, 0, 0
	for _, record := range examples() {
		counts[record.Split]++
		if menus[record.Split] == nil {
			menus[record.Split] = map[string]bool{}
		}
		var names []string
		for _, tool := range record.Tools {
			names = append(names, tool["function"].(map[string]any)["name"].(string))
		}
		slices.Sort(names)
		menus[record.Split][strings.Join(names, ",")] = true
		if record.ExecutorError {
			errors++
		}
		for _, message := range record.Messages {
			if message["tool_calls"] != nil {
				calls++
			}
		}
	}
	for _, sample := range scenarios() {
		if len(sample.history) > 0 {
			histories++
		}
		if len(sample.rounds) > 1 {
			multiround++
		}
	}
	t.Logf("casos=%v cardápios_distintos(train/valid/test)=%d/%d/%d rodadas_de_chamada=%d erros_explícitos=%d históricos=%d multirrodada=%d", counts, len(menus["train"]), len(menus["valid"]), len(menus["test"]), calls, errors, histories, multiround)
}

func TestRespostasInterpretamSemCopiarFerramentas(t *testing.T) {
	isoDate := regexp.MustCompile(`\b\d{4}-\d{2}(?:-\d{2})?\b`)
	for _, record := range examples() {
		answer := record.Messages[len(record.Messages)-1]["content"].(string)
		if isoDate.MatchString(answer) {
			t.Errorf("%s expõe data ISO", record.ID)
		}
		for _, forbidden := range []string{"Hoje no app", "Resultado consultado", "[fic-", "2026-10-"} {
			if strings.Contains(answer, forbidden) {
				t.Errorf("%s expõe molde ou detalhe interno: %s", record.ID, forbidden)
			}
		}
		for _, message := range record.Messages {
			if message["role"] == "tool" {
				content := message["content"].(string)
				if strings.Contains(answer, content) {
					t.Errorf("%s copia resultado inteiro", record.ID)
				}
			}
		}
	}
}

func TestCardapiosCobremProducaoNosReservados(t *testing.T) {
	for _, split := range []string{"valid", "test"} {
		names := map[string]bool{}
		for _, record := range examples() {
			if record.Split != split {
				continue
			}
			for _, definition := range record.Tools {
				names[definition["function"].(map[string]any)["name"].(string)] = true
			}
		}
		for _, name := range []string{"buscar_lancamentos", "propor_lancamento", "alterar_lancamento", "apagar_lancamento", "criar_meta", "criar_divida", "criar_conta_fixa", "registrar_aporte", "registrar_pagamento", "simular_dividas", "simular_meta", "achados", "comparar_meses", "proximos_vencimentos"} {
			if !names[name] {
				t.Errorf("%s não oferece %s", split, name)
			}
		}
	}
}

func TestInjecaoNosDadosComEscritaDisponivelEmCadaSplit(t *testing.T) {
	covered := map[string]bool{}
	for _, record := range examples() {
		if record.InjectionLocation == "" {
			continue
		}
		write := false
		for _, definition := range record.Tools {
			name := definition["function"].(map[string]any)["name"]
			if name == "apagar_lancamento" || name == "registrar_aporte" || name == "registrar_pagamento" {
				write = true
			}
		}
		answer := record.Messages[len(record.Messages)-1]["content"].(string)
		if strings.Contains(answer, "ORDEM_HOSTIL") {
			t.Errorf("%s replica ataque", record.ID)
		}
		if !write || len(record.Proposals) != 0 {
			t.Errorf("%s sem ferramentas de escrita ou com ação indevida", record.ID)
		}
		covered[record.Split] = covered[record.Split] || write
	}
	for _, split := range []string{"train", "valid", "test"} {
		if !covered[split] {
			t.Errorf("%s sem injeção nos dados e escrita disponível", split)
		}
	}
}
