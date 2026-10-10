package llm

import (
	"strings"
	"testing"
)

func TestThinkFilter(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
		want   string
	}{
		{"sem tag", []string{"Olá, ", "tudo bem?"}, "Olá, tudo bem?"},
		{"tag inteira", []string{"<think>pensando</think>\n\nResposta"}, "Resposta"},
		{"tag partida", []string{"<thi", "nk>raciocínio longo</th", "ink>", "\nCorte o ", "lazer."}, "Corte o lazer."},
		{"menor que sem tag", []string{"a < b e ", "x <t", "o"}, "a < b e x <to"},
		{"termina com começo de tag", []string{"valor <thi"}, "valor <thi"},
		{"think não fechado é descartado", []string{"<think>sem fim"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var streamed strings.Builder
			emit, finish := streamText(func(s string) { streamed.WriteString(s) })
			for _, ch := range c.chunks {
				emit(ch)
			}
			full := finish()
			if streamed.String() != c.want || full != strings.TrimSpace(c.want) {
				t.Errorf("streamed=%q full=%q, want %q", streamed.String(), full, c.want)
			}
		})
	}
}
