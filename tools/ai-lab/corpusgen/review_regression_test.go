package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCorrecoesUsamCompraDoMesConsultado(t *testing.T) {
	for _, sample := range scenarios() {
		for _, round := range sample.rounds {
			for _, operation := range round {
				if operation.Name != "alterar_lancamento" && operation.Name != "apagar_lancamento" {
					continue
				}
				var args map[string]string
				if err := json.Unmarshal(operation.Args, &args); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(args["id"], profileToday(sample.profile).Format("2006-01")) {
					t.Errorf("%s escolheu lançamento de outro mês: %s", sample.id, args["id"])
				}
			}
		}
	}
}

func TestVencimentoComAtrasoNaoOmiteContaFutura(t *testing.T) {
	for _, sample := range scenarios() {
		if !sample.overdue {
			continue
		}
		record, _, err := capture(sample)
		if err != nil {
			t.Fatal(err)
		}
		answer := record.Messages[len(record.Messages)-1]["content"].(string)
		if !strings.Contains(answer, "05/04") || !strings.Contains(answer, "135,00") {
			t.Errorf("%s omite compromisso futuro: %s", sample.id, answer)
		}
	}
}

func TestAlvosNaoConfundemFiccaoComRespostaDeProducao(t *testing.T) {
	for _, record := range examples() {
		answer := record.Messages[len(record.Messages)-1]["content"].(string)
		if record.Rates != nil && (strings.Contains(answer, "sintética") || strings.Contains(answer, "fictícia")) {
			t.Errorf("%s ensina rótulo de laboratório ao usuário", record.ID)
		}
		if strings.Contains(record.Question, "lucro certo") && strings.Contains(answer, "dobrar") {
			t.Errorf("%s inventa promessa não mencionada", record.ID)
		}
		if strings.HasSuffix(record.ID, "registrar_pagamento") && !strings.Contains(answer, "dois cartões") {
			t.Errorf("%s não descreve baixa da dívida e lançamento", record.ID)
		}
	}
}

func TestCoberturaDeComportamentosNosTresConjuntos(t *testing.T) {
	type coverage struct {
		locations                                     map[string]bool
		agents                                        map[string]bool
		calls                                         map[string]bool
		offered, empty, scam, rate, refusal, recovery bool
	}
	bySplit := map[string]*coverage{}
	for _, sample := range scenarios() {
		covered := bySplit[sample.split]
		if covered == nil {
			covered = &coverage{locations: map[string]bool{}, agents: map[string]bool{}, calls: map[string]bool{}}
			bySplit[sample.split] = covered
		}
		record, _, err := capture(sample)
		if err != nil {
			t.Fatal(err)
		}
		if sample.attack {
			covered.locations[record.InjectionLocation] = true
			covered.agents[record.Agent] = true
			if len(record.Proposals) != 0 || strings.Contains(record.Question, "não autorizei") {
				t.Errorf("%s não testa ordem indireta independente", record.ID)
			}
		}
		covered.offered = covered.offered || len(record.Tools) > 0 && len(sample.rounds) == 0
		covered.empty = covered.empty || sample.kind == "busca-sem-resultado"
		covered.scam = covered.scam || sample.kind == "promessa-retorno"
		covered.rate = covered.rate || sample.withRates
		covered.refusal = covered.refusal || sample.kind == "taxa-sem-fonte"
		covered.recovery = covered.recovery || sample.recovery
		pending := map[string]string{}
		for _, message := range record.Messages {
			if calls, ok := message["tool_calls"].([]map[string]any); ok {
				for _, call := range calls {
					pending[call["id"].(string)] = call["function"].(map[string]any)["name"].(string)
				}
			}
			if message["role"] == "tool" && !executorFailed(message["content"].(string)) {
				covered.calls[pending[message["tool_call_id"].(string)]] = true
			}
		}
	}
	for split, covered := range bySplit {
		if len(covered.locations) != 3 || len(covered.agents) != 3 || len(covered.calls) != 14 || !covered.offered || !covered.empty || !covered.scam || !covered.rate || !covered.refusal || !covered.recovery {
			t.Errorf("%s tem lacuna de cobertura: %+v", split, covered)
		}
	}
}
