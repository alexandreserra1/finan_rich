package advisor

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func TestBuildRequestKeepsRecentHistory(t *testing.T) {
	var history []llm.Message
	for i := range 10 { // 20 mensagens
		history = append(history,
			llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("pergunta %d", i)},
			llm.Message{Role: llm.RoleAssistant, Content: fmt.Sprintf("resposta %d", i)})
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
	history := []llm.Message{
		{Role: llm.RoleAssistant, Content: "órfã"},
		{Role: llm.RoleUser, Content: "p"},
		{Role: llm.RoleAssistant, Content: "r"},
	}
	req := BuildRequest(history, "q", "")
	if req.Messages[0].Content != "p" || len(req.Messages) != 3 {
		t.Errorf("mensagens = %+v", req.Messages)
	}
}

type fakeProvider struct{ got llm.Request }

func (f *fakeProvider) Chat(_ context.Context, req llm.Request, onText func(string)) (llm.Reply, error) {
	f.got = req
	onText("ok")
	return llm.Reply{Text: "ok"}, nil
}

func TestAsk(t *testing.T) {
	p := &fakeProvider{}
	var streamed string
	answer, err := Ask(context.Background(), p, nil, "oi", "dados", nil, func(s string) { streamed += s })
	if err != nil || answer != "ok" || streamed != "ok" || len(p.got.Messages) != 1 || p.got.Tools != nil {
		t.Errorf("answer=%q streamed=%q err=%v req=%+v", answer, streamed, err, p.got)
	}
}

func TestSnapshot(t *testing.T) {
	cat := func(id, name, bucket string) domain.Category {
		return domain.Category{Meta: domain.Meta{ID: id, UpdatedAt: 1}, Name: name, Bucket: bucket}
	}
	tx := func(id, kind, catID string, cents int64) domain.Transaction {
		return domain.Transaction{Meta: domain.Meta{ID: id, UpdatedAt: 1}, Kind: kind, Description: id, AmountCents: cents, CategoryID: catID, Date: "2026-10-05"}
	}
	l := domain.Ledger{
		Categories:   []domain.Category{cat("sal", "Salário", domain.BucketRenda), cat("laz", "Lazer", domain.BucketDesejos)},
		Transactions: []domain.Transaction{tx("a", domain.KindEntrada, "sal", 500000), tx("Bar do Zé", domain.KindSaida, "laz", 200000)},
		Debts:        []domain.Debt{{Meta: domain.Meta{ID: "d", UpdatedAt: 1}, Name: "Cartão", BalanceCents: 400000, MonthlyRateBP: 1250, MinPaymentCents: 40000}},
		Notes: []domain.Note{
			{Meta: domain.Meta{ID: "n1", UpdatedAt: 5}, Title: "Viagem", Body: "Juntar R$ 3.000\naté julho"},
			{Meta: domain.Meta{ID: "n2", UpdatedAt: 6, Deleted: true}, Title: "Apagada", Body: "não deve aparecer"},
		},
		Goals: []domain.Goal{{Meta: domain.Meta{ID: "g", UpdatedAt: 1}, Name: "Reserva", TargetCents: 600000, SavedCents: 150000, Deadline: "2027-03", StartDate: "2026-10-01"}},
		Recurring: []domain.Recurring{{Meta: domain.Meta{ID: "r", UpdatedAt: 1}, Kind: domain.KindSaida, Description: "Academia",
			AmountCents: 9990, CategoryID: "laz", Day: 5, Start: "2026-01"}},
	}
	l.Categories[1].BudgetCents = 240000 // teto de R$ 2.400 em Lazer: 83% usado
	got := Snapshot(BuildFacts(l, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)))
	for _, want := range []string{
		"## Orçamentos por categoria", "- Lazer: teto R$ 2.400,00; usado R$ 2.000,00 (83%); restam R$ 400,00",
		"Orçamento de Lazer em 83%: Restam R$ 400,00 de R$ 2.400,00 no mês.",
		"10/10/2026", "outubro de 2026", "Entradas: R$ 5.000,00",
		"desejos: meta 30% = R$ 1.500,00; gasto R$ 2.000,00 (133% da meta); passou R$ 500,00 da meta",
		"Lazer (desejos): R$ 2.000,00",
		"juros de 12,50% a.m. = R$ 500,00 de juros por mês",
		"o mínimo NÃO cobre os juros (R$ 400,00 < R$ 500,00)",
		"Reserva mínima (3 meses de gastos): R$ 6.000,00",
		"sobra livre (R$ 2.600,00/mês, já descontadas as parcelas de dívidas): mínima em 3 meses",
		"## Pontos de atenção", "Cartão cresce todo mês", "Gastos com desejos passaram da meta em R$ 500,00",
		"Parcelas mínimas que ainda faltam pagar este mês (não estão nas saídas): R$ 400,00",
		"Sobra livre depois de pagar as parcelas mínimas: R$ 3.000,00 - R$ 400,00 = R$ 2.600,00",
		"pagando R$ 400,00 + R$ 2.600,00 = R$ 3.000,00 por mês",
		"## Anotações do usuário", "- Viagem: Juntar R$ 3.000 até julho",
		"- Reserva: juntou R$ 1.500,00 de R$ 6.000,00 (25%); prazo março de 2027; precisa guardar R$ 750,00 por mês nos próximos 6 meses",
		"- Academia: saída de R$ 99,90 todo dia 5, categoria Lazer",
		"## Diagnóstico das dívidas (contas feitas pelo app; use estes números, não refaça)",
		"- Dívida mais cara: Cartão, R$ 500,00 de juros por mês (12,50% a.m.)",
		"- O que mais pesou em outubro de 2026: Lazer, R$ 2.000,00",
		"## Gastos por estabelecimento", "- Bar do Zé: R$ 2.000,00 em 1 compra(s) nos últimos 12 meses; R$ 2.000,00 em 2026; R$ 2.000,00 neste mês; última em 05/10/2026",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("snapshot sem %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Apagada") {
		t.Error("nota apagada não pode ir para a IA")
	}
}

// Quando o mínimo cobre os juros, o resumo diz quanto sobra para abater: sem
// isso, o modelo já chegou a dizer que R$ 480 "não cobrem" R$ 412,80.
func TestSnapshotSaysMinimumCoversInterest(t *testing.T) {
	l := domain.Ledger{Debts: []domain.Debt{{Meta: domain.Meta{ID: "d", UpdatedAt: 1}, Name: "Cartão", BalanceCents: 320000, MonthlyRateBP: 1290, MinPaymentCents: 48000}}}
	got := Snapshot(BuildFacts(l, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)))
	if !strings.Contains(got, "R$ 412,80 de juros por mês, pagamento mínimo R$ 480,00; o mínimo cobre os juros e abate R$ 67,20 do saldo por mês") {
		t.Errorf("snapshot:\n%s", got)
	}
}

// O conhecimento certo vai junto de cada pergunta (no máximo 2 temas).
func TestKnowledgeFor(t *testing.T) {
	for q, want := range map[string]string{
		"como saio do rotativo do cartão?":         "Lei 14.690",
		"onde guardo minha reserva de emergência?": "FGC",
		"vale investir em LCI?":                    "isentas de IR",
		"oi":                                       "Ordem para usar o dinheiro",
		"recebi o 13º, o que faço?":                "13º, férias",
		"caí num golpe do pix":                     "MED",
	} {
		if got := knowledgeFor(q); !strings.Contains(got, want) {
			t.Errorf("%q: faltou %q", q, want)
		}
	}
	if n := strings.Count(knowledgeFor("dívida do cartão e reserva e investir e golpe"), "\n\n") + 1; n > maxTopics {
		t.Errorf("temas demais: %d", n)
	}
	req := BuildRequest(nil, "como saio do rotativo?", "resumo")
	if c := req.Messages[0].Content; !strings.HasPrefix(c, "<conhecimento>") || !strings.HasSuffix(c, "</dados_financeiros>\n\ncomo saio do rotativo?") {
		t.Errorf("a pergunta precisa ficar no fim, depois do conhecimento e dos dados: %q", c[len(c)-80:])
	}
}

// IA dentro do celular: sem ferramentas, sem apelidos, histórico curto.
func TestLocalRequest(t *testing.T) {
	l := domain.Ledger{
		Categories: []domain.Category{{Meta: domain.Meta{ID: "c", UpdatedAt: 1}, Name: "Lazer", Bucket: domain.BucketDesejos}},
		Debts:      []domain.Debt{{Meta: domain.Meta{ID: "d", UpdatedAt: 1}, Name: "Cartão Nubank", BalanceCents: 100000, MonthlyRateBP: 1000, MinPaymentCents: 20000}},
	}
	for i := range 15 {
		l.Transactions = append(l.Transactions, domain.Transaction{Meta: domain.Meta{ID: fmt.Sprint(i), UpdatedAt: 1}, Kind: domain.KindSaida,
			Description: "Loja " + string(rune('A'+i)) + "x", AmountCents: int64(1000 + i), CategoryID: "c", Date: "2026-10-05"})
	}
	history := []llm.Message{
		{Role: llm.RoleUser, Content: "p1"}, {Role: llm.RoleAssistant, Content: "r1"},
		{Role: llm.RoleUser, Content: "p2"}, {Role: llm.RoleAssistant, Content: "r2"},
	}
	req := LocalRequest(Question{History: history, Text: "e o cartão?", Ledger: l, Today: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)})
	if req.System != SystemPrompt || req.Tools != nil {
		t.Errorf("deve ir sem ferramentas: tools=%v", req.Tools)
	}
	if len(req.Messages) != 3 || req.Messages[0].Content != "p2" || req.Messages[1].Content != "r2" {
		t.Fatalf("histórico = %+v", req.Messages)
	}
	last := req.Messages[2].Content
	if !strings.Contains(last, "Cartão Nubank") || !strings.HasSuffix(last, "e o cartão?") {
		t.Errorf("os nomes vão sem apelido (nada sai do aparelho): %q", last)
	}
	if n := strings.Count(last, "compra(s) nos últimos 12 meses"); n != maxLocalMerchants {
		t.Errorf("estabelecimentos = %d, want %d", n, maxLocalMerchants)
	}
}
