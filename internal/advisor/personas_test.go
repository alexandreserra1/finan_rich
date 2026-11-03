package advisor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func TestAnswerAsPersona(t *testing.T) {
	fake := &scriptedLLM{replies: []llm.Reply{{Text: "Ataque o cartão."}}}
	q := Question{Text: "e agora?", Ledger: toolsLedger(), Today: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), Agent: "lia"}
	if _, err := Answer(context.Background(), fake, q, func(string) {}, func(Proposal) {}); err != nil {
		t.Fatal(err)
	}
	req := fake.asked[0]
	if !strings.HasPrefix(req.System, PromptMarker) || !strings.Contains(req.System, "Você é a Lia") || !strings.Contains(req.System, "o app se chama Rich") {
		t.Errorf("instruções sem o marcador, sem a voz da Lia ou sem o nome Rich:\n%s", req.System)
	}
	var names []string
	for _, tl := range req.Tools {
		names = append(names, tl.Name)
	}
	if got := strings.Join(names, ","); got != "buscar_lancamentos,alterar_lancamento,apagar_lancamento,achados,comparar_meses" {
		t.Errorf("ferramentas da Lia = %s", got)
	}

	// Sem personagem, desconhecido ou de quem saiu do time: fala o Otávio, com as ferramentas dele.
	for _, agent := range []string{"", "quem?", "bento"} {
		fake = &scriptedLLM{replies: []llm.Reply{{Text: "Oi."}}}
		q.Agent = agent
		Answer(context.Background(), fake, q, func(string) {}, func(Proposal) {})
		if !strings.Contains(fake.asked[0].System, "Você é o Otávio") || len(fake.asked[0].Tools) != len(persona("otavio").Tools) {
			t.Errorf("%q: Otávio com %d ferramentas", agent, len(fake.asked[0].Tools))
		}
	}
}

// A IA grátis (Groq) recusa pedido maior que a cota por minuto do modelo (7 mil
// tokens no menor): as ferramentas de cada um do time precisam caber com folga.
func TestPersonaToolsFitFreeQuota(t *testing.T) {
	for _, id := range []string{"otavio", "lia", "tito"} {
		tools := NewTools(toolsLedger(), time.Now(), nil)
		tools.only = persona(id).Tools
		defs, _ := json.Marshal(tools.Defs())
		if len(defs) > 4100 {
			t.Errorf("%s: ferramentas com %d caracteres (até 4100)", id, len(defs))
		}
	}
	// Com ferramentas, o Otávio busca nos lançamentos: a lista de estabelecimentos
	// (assunto da Lia) fica fora do pedido dele.
	fake := &scriptedLLM{replies: []llm.Reply{{Text: "Oi."}}}
	Answer(context.Background(), fake, Question{Text: "oi", Ledger: teamLedger(), Today: teamDay}, func(string) {}, func(Proposal) {})
	if msg := fake.asked[0].Messages[len(fake.asked[0].Messages)-1].Content; strings.Contains(msg, "## Gastos por estabelecimento") || !strings.Contains(msg, "## Dívidas") {
		t.Errorf("pedido do Otávio:\n%s", msg)
	}
	tools := NewTools(toolsLedger(), time.Now(), nil)
	tools.only = persona("otavio").Tools
	var names []string
	for _, tl := range tools.Defs() {
		names = append(names, tl.Name)
	}
	for _, want := range []string{"simular_dividas", "proximos_vencimentos", "registrar_pagamento", "criar_divida", "criar_meta", "propor_lancamento", "alterar_lancamento", "buscar_lancamentos"} {
		if !strings.Contains(","+strings.Join(names, ",")+",", ","+want+",") {
			t.Errorf("Otávio sem %s: %v", want, names)
		}
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

// Cada personagem recebe só as seções do resumo que usa: prompt menor, mais
// rápido (principalmente no modelo local) e menos chance de misturar assuntos.
func TestSnapshotFor(t *testing.T) {
	f := BuildFacts(evalLedger(), time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
	full := SnapshotFor(f, "otavio")
	if full != Snapshot(f) {
		t.Error("o Otávio recebe o resumo inteiro")
	}
	for agent, c := range map[string]struct{ has, hasNot []string }{
		"tito": {[]string{"## Mês atual", "## Metas do usuário", "## Reserva de emergência"}, []string{"## Gastos por estabelecimento", "## Diagnóstico das dívidas"}},
		"lia":  {[]string{"## Mês atual", "## Gastos por estabelecimento", "## Ritmo do mês"}, []string{"## Diagnóstico das dívidas", "## Metas do usuário"}},
	} {
		s := SnapshotFor(f, agent)
		for _, w := range c.has {
			if !strings.Contains(s, w) {
				t.Errorf("%s sem %q", agent, w)
			}
		}
		for _, w := range c.hasNot {
			if strings.Contains(s, w) {
				t.Errorf("%s não deveria ter %q", agent, w)
			}
		}
		if len(s) >= len(full) {
			t.Errorf("%s: %d bytes, não é menor que o resumo inteiro (%d)", agent, len(s), len(full))
		}
		t.Logf("%s: %d de %d bytes", agent, len(s), len(full))
	}
}
