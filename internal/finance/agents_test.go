package finance

import (
	"strings"
	"testing"
)

func TestTeam(t *testing.T) {
	var ids []string
	for _, a := range Team {
		ids = append(ids, a.ID)
		if a.Name == "" || a.Role == "" || a.Quote == "" {
			t.Errorf("%+v sem nome, papel ou frase", a)
		}
	}
	if got := strings.Join(ids, ","); got != "otavio,lia,tito" {
		t.Errorf("time = %s", got)
	}
}

func TestAgentFor(t *testing.T) {
	for kind, want := range map[string]string{
		"debt-due": "Otávio", "debt-growing": "Otávio", "budget-near": "Lia", "budget-over": "Lia", "bucket-over": "Lia",
		"unusual": "Lia", "goal-late": "Tito", "goal-done": "Tito", "qualquer-outro": "Otávio",
	} {
		if got := agentFor(kind); got.Name != want {
			t.Errorf("agentFor(%q) = %s, quer %s", kind, got.Name, want)
		}
	}
}

func TestRoute(t *testing.T) {
	for q, want := range map[string]string{
		"Qual dívida devo pagar primeiro?":        "otavio",
		"como saio do rotativo do cartão":         "otavio",
		"quando vence a fatura?":                  "otavio",
		"Como chego na minha meta mais rápido?":   "tito",
		"quanto guardar para a reserva":           "tito",
		"Quanto gastei com iFood este ano?":       "lia",
		"tenho assinatura esquecida?":             "lia",
		"meu orçamento de mercado estourou":       "lia",
		"oi, tudo bem?":                           "",
		"o que eu devo fazer com o meu dinheiro?": "", // "devo" de dever fazer não é dívida
	} {
		if got := Route(q); got != want {
			t.Errorf("Route(%q) = %q, quer %q", q, got, want)
		}
	}
}

func TestMeses(t *testing.T) {
	for n, want := range map[int]string{0: "0 meses", 1: "1 mês", 2: "2 meses", 13: "13 meses"} {
		if got := Meses(n); got != want {
			t.Errorf("Meses(%d) = %q, quer %q", n, got, want)
		}
	}
}
