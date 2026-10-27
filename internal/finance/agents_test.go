package finance

import "testing"

func TestAgentFor(t *testing.T) {
	for kind, want := range map[string]string{
		"budget-near": "Luna", "budget-over": "Luna", "goal-late": "Tito", "goal-done": "Tito", "qualquer-outro": "Jade",
	} {
		if got := agentFor(kind); got.Name != want {
			t.Errorf("agentFor(%q) = %s, quer %s", kind, got.Name, want)
		}
	}
}

func TestRoute(t *testing.T) {
	for q, want := range map[string]string{
		"Qual dívida devo pagar primeiro?":        "bento",
		"como saio do rotativo do cartão":         "bento",
		"Como chego na minha meta mais rápido?":   "tito",
		"quanto guardar para a reserva":           "tito",
		"Quanto gastei com iFood este ano?":       "otavio",
		"tenho assinatura esquecida?":             "otavio",
		"quando vence a fatura?":                  "luna",
		"meu orçamento de mercado estourou":       "luna",
		"oi, tudo bem?":                           "",
		"o que eu devo fazer com o meu dinheiro?": "", // "devo" de dever fazer não é dívida
	} {
		if got := Route(q); got != want {
			t.Errorf("Route(%q) = %q, quer %q", q, got, want)
		}
	}
}
