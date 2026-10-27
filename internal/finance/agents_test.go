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
