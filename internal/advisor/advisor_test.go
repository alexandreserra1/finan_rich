package advisor

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/llm"
	"github.com/alexandreserra1/finan_rich/internal/store"
)

func TestBuildRequestKeepsRecentHistory(t *testing.T) {
	var history []store.ChatMessage
	for i := range 10 { // 20 mensagens
		history = append(history,
			store.ChatMessage{Role: store.RoleUser, Content: fmt.Sprintf("pergunta %d", i)},
			store.ChatMessage{Role: store.RoleAssistant, Content: fmt.Sprintf("resposta %d", i)})
	}
	req := BuildRequest(history, "e agora?", "RETRATO")

	if req.System != SystemPrompt {
		t.Error("system prompt diferente")
	}
	if len(req.Messages) != maxHistory+1 {
		t.Fatalf("mensagens = %d, want %d", len(req.Messages), maxHistory+1)
	}
	if req.Messages[0].Role != llm.RoleUser || req.Messages[0].Content != "pergunta 4" {
		t.Errorf("primeira mensagem = %+v", req.Messages[0])
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "RETRATO") || !strings.HasSuffix(last.Content, "e agora?") {
		t.Errorf("última mensagem = %+v", last)
	}
	for _, m := range req.Messages[:len(req.Messages)-1] {
		if strings.Contains(m.Content, "dados_financeiros") {
			t.Error("retrato só deve ir na última pergunta")
		}
	}
}

func TestBuildRequestStartsWithUser(t *testing.T) {
	history := []store.ChatMessage{
		{Role: store.RoleAssistant, Content: "órfã"},
		{Role: store.RoleUser, Content: "p"},
		{Role: store.RoleAssistant, Content: "r"},
	}
	req := BuildRequest(history, "q", "")
	if req.Messages[0].Content != "p" || len(req.Messages) != 3 {
		t.Errorf("mensagens = %+v", req.Messages)
	}
}

type fakeProvider struct{ got llm.Request }

func (f *fakeProvider) Chat(_ context.Context, req llm.Request, onText func(string)) (string, error) {
	f.got = req
	onText("ok")
	return "ok", nil
}

func TestAsk(t *testing.T) {
	p := &fakeProvider{}
	var streamed string
	answer, err := Ask(context.Background(), p, nil, "oi", "dados", func(s string) { streamed += s })
	if err != nil || answer != "ok" || streamed != "ok" || len(p.got.Messages) != 1 {
		t.Errorf("answer=%q streamed=%q err=%v req=%+v", answer, streamed, err, p.got)
	}
}

func TestSnapshot(t *testing.T) {
	m := store.NewMonth(2026, time.October)
	sum := finance.Summarize(m, []store.Transaction{
		{Kind: store.KindEntrada, AmountCents: 500000, CategoryName: "Salário", Bucket: store.BucketRenda},
		{Kind: store.KindSaida, AmountCents: 200000, CategoryName: "Lazer", Bucket: store.BucketDesejos},
	}, map[string]int{"necessidades": 50, "desejos": 30, "investimentos": 20})
	plan := finance.PlanDebts([]store.Debt{
		{Name: "Cartão", BalanceCents: 400000, MonthlyRateBP: 1250, MinPaymentCents: 40000},
	})
	got := Snapshot(Facts{
		Today: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Month: sum, Debts: plan, Plan: finance.PlanMonth(sum, plan),
		Emergency: finance.PlanEmergencyFund([]store.Totals{{InCents: 500000, OutCents: 200000, BalanceCents: 300000}}, 40000),
	})
	for _, want := range []string{
		"10/10/2026", "outubro de 2026", "Entradas: R$ 5.000,00",
		"desejos: meta 30% = R$ 1.500,00; gasto R$ 2.000,00 (133% da meta); passou R$ 500,00 da meta",
		"Lazer (desejos): R$ 2.000,00",
		"juros de 12,50% a.m. = R$ 500,00 de juros por mês",
		"o mínimo NÃO cobre os juros (R$ 400,00 < R$ 500,00)",
		"Reserva mínima (3 meses de gastos): R$ 6.000,00", "sobra livre (R$ 2.600,00/mês, já descontadas as parcelas de dívidas): mínima em 3 meses",
		"## Pontos de atenção", "Cartão cresce todo mês",
		"Parcelas mínimas que ainda faltam pagar este mês (não estão nas saídas): R$ 400,00",
		"Sobra livre depois de pagar as parcelas mínimas: R$ 3.000,00 - R$ 400,00 = R$ 2.600,00",
		"pagando R$ 400,00 + R$ 2.600,00 = R$ 3.000,00 por mês", "Gastos com desejos passaram da meta em R$ 500,00",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("snapshot sem %q:\n%s", want, got)
		}
	}
}
