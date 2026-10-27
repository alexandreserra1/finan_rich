package advisor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func TestAnswerAsPersona(t *testing.T) {
	fake := &scriptedLLM{replies: []llm.Reply{{Text: "Ataque o cartão."}}}
	q := Question{Text: "e agora?", Ledger: toolsLedger(), Today: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), Agent: "bento"}
	if _, _, err := Answer(context.Background(), fake, q, func(string) {}, func(Proposal) {}); err != nil {
		t.Fatal(err)
	}
	req := fake.asked[0]
	if !strings.HasPrefix(req.System, PromptMarker) || !strings.Contains(req.System, "Você é o Bento") {
		t.Errorf("instruções sem o marcador ou sem a voz do Bento:\n%s", req.System)
	}
	var names []string
	for _, tl := range req.Tools {
		names = append(names, tl.Name)
	}
	if got := strings.Join(names, ","); got != "buscar_lancamentos,criar_divida,registrar_pagamento,simular_dividas" {
		t.Errorf("ferramentas do Bento = %s", got)
	}

	// Sem personagem (ou desconhecido), fala a Jade, com todas as ferramentas.
	fake = &scriptedLLM{replies: []llm.Reply{{Text: "Oi."}}}
	q.Agent = "quem?"
	Answer(context.Background(), fake, q, func(string) {}, func(Proposal) {})
	if !strings.Contains(fake.asked[0].System, "Você é a Jade") || len(fake.asked[0].Tools) != len(NewTools(toolsLedger(), time.Now(), nil).Defs()) {
		t.Errorf("Jade: %d ferramentas", len(fake.asked[0].Tools))
	}
}

func TestAskWithoutToolsKeepsPersona(t *testing.T) {
	fake := &scriptedLLM{
		errs:    []error{&llm.APIError{Status: 400, Message: "model does not support tools"}},
		replies: []llm.Reply{{}, {Text: "ok"}},
	}
	q := Question{Text: "oi", Ledger: toolsLedger(), Today: time.Now(), Agent: "tito"}
	Answer(context.Background(), fake, q, func(string) {}, func(Proposal) {})
	if s := fake.asked[1].System; fake.asked[1].Tools != nil || !strings.Contains(s, "Você é o Tito") || strings.Contains(s, "Ferramentas:") {
		t.Errorf("sem ferramentas, a voz continua e a lista de ferramentas sai:\n%s", s)
	}
}

func TestPersonaToolsAreEnforced(t *testing.T) {
	tools := NewTools(toolsLedger(), time.Now(), func(Proposal) { t.Error("não podia propor") })
	tools.only = persona("tito").Tools
	if got := tools.Run(call("criar_divida", `{"nome":"X","saldo":"100"}`)); !strings.Contains(got, "não é com você") {
		t.Errorf("Tito criou dívida: %s", got)
	}
}

func TestLocalRequestAsPersona(t *testing.T) {
	r := LocalRequest(Question{Text: "oi", Ledger: toolsLedger(), Today: time.Now(), Agent: "otavio"})
	if !strings.HasPrefix(r.System, PromptMarker) || !strings.Contains(r.System, "Você é o Otávio") {
		t.Errorf("IA do celular sem a voz do Otávio:\n%s", r.System)
	}
}
