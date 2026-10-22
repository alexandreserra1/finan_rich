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
	got := Snapshot(BuildFacts(l, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)))
	for _, want := range []string{
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
