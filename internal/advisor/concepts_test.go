package advisor

import (
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/rates"
)

// Perguntas de conceito: o app explica com um texto revisado e, quando tem, os números
// da pessoa (dívidas, reserva, metas, divisão do salário) e as taxas do dia.
func TestConceptAnswers(t *testing.T) {
	today := rates.Rates{SelicBP: 1375, CDIBP: 1365, IPCABP: 463, IPCAMonth: "2026-10", Date: "2026-11-17"}
	for _, c := range []struct {
		text string
		want []string
	}{
		{"o que é CDB?", []string{"**CDB é um empréstimo que você faz ao banco**", "FGC", "R$ 250 mil", "CDI mais recente do Banco Central (17/11/2026): 13,65% ao ano.", "A dívida Cartão cobra 10,00% de juros ao mês (213,84% ao ano)"}},
		{"me explica o que é um CDB", []string{"**CDB é um empréstimo"}},
		{"como funciona o tesouro selic?", []string{"**O Tesouro Selic é um título do governo", "Tesouro Nacional", "Selic mais recente do Banco Central (17/11/2026): 13,75% ao ano."}},
		{"o que significa CDI?", []string{"**CDI é a taxa", "CDI mais recente do Banco Central (17/11/2026): 13,65% ao ano."}},
		{"o que é a selic?", []string{"**A Selic é a taxa básica de juros", "Copom", "Selic mais recente do Banco Central (17/11/2026): 13,75% ao ano."}},
		{"o que é IPCA?", []string{"**O IPCA é a inflação oficial", "Nos 12 meses até outubro de 2026, o IPCA foi de 4,63%."}},
		{"o que é inflação?", []string{"**O IPCA é a inflação oficial"}},
		{"como funciona a poupança?", []string{"0,5% ao mês mais a TR", "13,75% ao ano. Com ela acima de 8,5%, a poupança rende 0,5% ao mês mais a TR."}},
		{"poupança ou CDB?", []string{"0,5% ao mês mais a TR", "CDB"}},
		{"o que é LCI?", []string{"**LCI e LCA", "Não pagam IR"}},
		{"o que é o FGC?", []string{"**O FGC", "R$ 250 mil", "R$ 1 milhão a cada 4 anos"}},
		{"como funciona o rotativo do cartão?", []string{"**Rotativo é o que sobra da fatura", "Lei 14.690/2023", "Cartão: R$ 1.000,00 a 10,00% ao mês (213,84% ao ano)"}},
		{"devo pagar só o mínimo do cartão?", []string{"**Evite pagar só o mínimo", "fatura inteira", "rotativo"}},
		{"o que é cheque especial?", []string{"**O cheque especial", "8% ao mês"}},
		{"o que são juros compostos?", []string{"**Juros compostos são juros sobre juros", "R$ 23 mil", "A dívida Cartão, a 10,00% ao mês, dobra em 8 meses"}},
		{"qual a diferença entre avalanche e bola de neve?", []string{"Avalanche", "Bola de neve", "Nas suas dívidas: avalanche é Cartão → Empréstimo; bola de neve é Cartão → Empréstimo."}},
		{"o que é score?", []string{"**O score é uma nota de 0 a 1.000**", "5 dias úteis"}},
		{"o que é CET?", []string{"**CET é o Custo Efetivo Total**"}},
		{"o que é consignado?", []string{"**Consignado é um empréstimo", "margem consignável"}},
		{"o que é portabilidade de crédito?", []string{"**Portabilidade de crédito", "sem custo"}},
		{"vale a pena antecipar parcelas?", []string{"**Antecipar parcelas dá desconto", "últimas parcelas", "Cartão: 10,00% ao mês (213,84% ao ano)", "13,65% ao ano"}},
		{"o que faço com o 13º?", []string{"**Use na ordem", "Cartão: R$ 1.000,00 a 10,00% ao mês", "Viagem: faltam R$ 6.000,00"}},
		{"o que é a regra 50/30/20?", []string{"**A regra 50/30/20", "até 50%", "Sua divisão no app: necessidades 50%, desejos 30%, investimentos 20%."}},
		{"o que é reserva de emergência?", []string{"**Reserva de emergência é o dinheiro para imprevistos**", "de 3 a 6 meses", "Com dívida cara (Cartão, a 10,00% ao mês)"}},
		{"como montar minha reserva de emergência?", []string{"**Reserva de emergência", "Tesouro Selic"}},
		{"vale a pena investir em ações agora?", []string{"**Ação é um pedaço de uma empresa**", "5 anos", "A dívida Cartão cobra 10,00% de juros ao mês (213,84% ao ano)", "reserva", "O app não indica ação"}},
	} {
		for _, agent := range []string{"otavio", "lia", "tito"} {
			q := Question{Ledger: teamLedger(), Today: teamDay, Agent: agent, Text: c.text, Rates: today}
			got := appAnswer(q)
			wantAll(t, agent+" "+c.text, got, c.want...)
			if !Prepared(q) {
				t.Errorf("%s %q: a explicação vale como resposta do app", agent, c.text)
			}
		}
	}
}

// Sem taxas do dia, a explicação não inventa número: manda conferir.
func TestConceptAnswersWithoutRates(t *testing.T) {
	got := appAnswer(offlineQ("otavio", "o que é CDB?"))
	if !strings.Contains(got, "Confira a taxa do dia") || strings.Contains(got, "13,65") {
		t.Errorf("sem taxas:\n%s", got)
	}
	// Sem dívida nem meta, as explicações seguem sem a parte da pessoa.
	q := offlineQ("otavio", "o que faço com o 13º?")
	q.Ledger.Debts, q.Ledger.Goals = nil, nil
	if got := appAnswer(q); !strings.Contains(got, "**Use na ordem") || strings.Contains(got, "Cartão") {
		t.Errorf("13º sem dívidas:\n%s", got)
	}
}

// Conceito com outra coisa junto (duas perguntas, um caso pessoal, uma ação) vai para a IA.
func TestConceptAnswersLeaveTheRestToAI(t *testing.T) {
	for _, text := range []string{
		"o que é cdb e lci?", "o que é cdb, e como investir?", "vale a pena colocar 5 mil num cdb do banco inter?",
		"o que é melhor para a minha reserva, cdb ou tesouro?", "recebi o 13º de R$ 4.500, o que faço com ele?",
		"como funciona o rotativo se eu já parcelei a fatura duas vezes no ano passado?", "o que é a vida?",
		"o que é o pix?", "vale a pena trocar de carro?", "lança 50 no cdb",
	} {
		if got := appAnswer(offlineQ("otavio", text)); got != "" {
			t.Errorf("%q vai para a IA, mas o app respondeu:\n%s", text, got)
		}
	}
}

func TestAnnualBP(t *testing.T) {
	for monthly, want := range map[int]int{1000: 21384, 1400: 38179, 800: 15182, 100: 1268, 0: 0} {
		if got := annualBP(monthly); got != want {
			t.Errorf("annualBP(%d) = %d, quer %d", monthly, got, want)
		}
	}
	for monthly, want := range map[int]int{1000: 8, 1400: 6, 200: 36} {
		if got := doublingMonths(monthly); got != want {
			t.Errorf("doublingMonths(%d) = %d, quer %d", monthly, got, want)
		}
	}
}
