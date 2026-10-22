package advisor

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func anonLedger() domain.Ledger {
	l := toolsLedger()
	l.Debts = []domain.Debt{{Meta: domain.Meta{ID: "d1", UpdatedAt: 1}, Name: "Cartão Nubank", Creditor: "Nu Pagamentos", BalanceCents: 320000, MonthlyRateBP: 1290, MinPaymentCents: 48000}}
	l.Goals = []domain.Goal{{Meta: domain.Meta{ID: "g1", UpdatedAt: 1}, Name: "Viagem para Recife", TargetCents: 600000, StartDate: "2026-01-01"}}
	l.Notes = []domain.Note{{Meta: domain.Meta{ID: "n1", UpdatedAt: 1}, Title: "Senha do banco", Body: "não conte a ninguém"}}
	return l
}

func TestAnonymizeHidesNames(t *testing.T) {
	anon, a := Anonymize(anonLedger())
	raw, _ := json.Marshal(anon)
	snap := Snapshot(BuildFacts(anon, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)))
	for _, secret := range []string{"iFood", "IFOOD", "Uber", "Nubank", "Nu Pagamentos", "Recife", "Senha", "ninguém"} {
		if strings.Contains(string(raw), secret) || strings.Contains(snap, secret) {
			t.Errorf("%q vazou para a IA", secret)
		}
	}
	// Mesmo estabelecimento, mesmo apelido; valores e categorias continuam.
	if anon.Transactions[0].Description != anon.Transactions[1].Description || !strings.HasPrefix(anon.Transactions[0].Description, "Loja ") ||
		anon.Transactions[3].Description != "Fonte 3" || anon.Transactions[0].AmountCents != 5230 || anon.Debts[0].Name != "Dívida 1" || anon.Debts[0].Creditor != "Credor 1" {
		t.Errorf("anônimo = %+v %+v", anon.Transactions, anon.Debts)
	}

	// A pergunta da pessoa também sai com apelidos (sem acento, maiúscula ou não).
	q := a.Text("Quanto gastei com iFood e com o UBER? E a dívida do cartão nubank com a nu pagamentos?")
	if strings.Contains(strings.ToLower(q), "ifood") || strings.Contains(strings.ToLower(q), "uber") || strings.Contains(strings.ToLower(q), "nubank") ||
		!strings.Contains(q, anon.Transactions[0].Description) || !strings.Contains(q, "Dívida 1") || !strings.Contains(q, "Credor 1") {
		t.Errorf("pergunta = %q", q)
	}
	if got := a.Text("superuber não é uber"); !strings.HasPrefix(got, "superuber") {
		t.Errorf("só palavras inteiras: %q", got)
	}
	// Na volta, os nomes reaparecem.
	if got := a.Reveal("Você gastou R$ 82,30 com " + anon.Transactions[0].Description + " e deve na Dívida 1."); got != "Você gastou R$ 82,30 com iFood e deve na Cartão Nubank." {
		t.Errorf("resposta = %q", got)
	}
}

// Uma alteração proposta pela IA não pode gravar o apelido no lugar do nome.
func TestRevealProposalKeepsRealNames(t *testing.T) {
	anon, a := Anonymize(anonLedger())
	tools := NewTools(anon, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), nil)
	edit, err := tools.editTransaction("3", "", "30", "", "")
	if err != nil {
		t.Fatal(err)
	}
	got := a.RevealProposal(edit)
	var tx domain.Transaction
	json.Unmarshal(got.Record, &tx)
	if tx.Description != "Uber" || tx.AmountCents != 3000 || !strings.Contains(got.Label, "Uber") || strings.Contains(got.Label, "Loja") {
		t.Errorf("alteração = %+v %q", tx, got.Label)
	}
	pay, _, err := tools.payDebt("Dívida 1", "500", "")
	if err != nil {
		t.Fatal(err)
	}
	var d domain.Debt
	json.Unmarshal(a.RevealProposal(pay).Record, &d)
	if d.Name != "Cartão Nubank" || d.Creditor != "Nu Pagamentos" || d.BalanceCents != 270000 {
		t.Errorf("pagamento = %+v", d)
	}
	// Lançamento novo com nome que a pessoa escreveu: passa por Reveal.
	create, _ := tools.newTransaction(anon.Transactions[2].Description+" aeroporto", "40", "", "Transporte", "")
	json.Unmarshal(a.RevealProposal(create).Record, &tx)
	if tx.Description != "Uber aeroporto" {
		t.Errorf("novo = %q", tx.Description)
	}
}
