package advisor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// Perguntas de conceito ("o que é CDB?", "como funciona o rotativo?", "vale a pena
// antecipar parcelas?"): o app responde com um texto curto e revisado e, quando tem, os
// números da pessoa (a dívida mais cara, a reserva, as metas) e as taxas do dia. As regras
// do Brasil que mudam (taxas, limites) ficam só onde são estáveis; a taxa do dia vem do
// Banco Central (Question.Rates) ou a resposta manda conferir. Conceito junto com outra
// coisa ("cdb e lci", "cdb do banco X para a minha reserva") vai para a IA.

type concept struct {
	subject *regexp.Regexp // o assunto depois de "o que é", "como funciona" (ou a pergunta só com ele)
	ask     *regexp.Regexp // outras formas da pergunta inteira; nil se não tem
	text    func(q Question, f string) string
}

var (
	// defRe: o começo que pede uma definição ("me explica o que é", "como funciona").
	defRe = regexp.MustCompile(`^((me )?(explica|explique|fala sobre|fale sobre) |sobre )?(o que (e|eh|sao|seria|significa)|o que quer dizer|como funcionam?|pra que servem?|para que servem?|qual (e )?a diferenca entre)? ?`)
	// articleRe: artigo antes do assunto ("o que é um CDB?", "o tal do score").
	articleRe = regexp.MustCompile(`^((o|a|os|as|um|uma|esse|essa|esses|essas|o tal|a tal|tal) (de |do |da )?)+`)
)

func re(expr string) *regexp.Regexp { return regexp.MustCompile(expr) }

// conceptAnswer é a explicação pronta para a pergunta, ou "".
func conceptAnswer(q Question, f string) string {
	subject := articleRe.ReplaceAllString(f[len(defRe.FindString(f)):], "")
	for _, c := range concepts {
		if c.subject != nil && c.subject.MatchString(subject) || c.ask != nil && c.ask.MatchString(f) {
			return c.text(q, f)
		}
	}
	return ""
}

var concepts = []concept{
	{re(`^cdbs?( de liquidez diaria| de banco| do banco)?$`), nil, func(q Question, _ string) string {
		return explain("**CDB é um empréstimo que você faz ao banco**, e ele paga juros por isso.",
			"O mais comum rende um percentual do CDI: \"100% do CDI\" rende perto da Selic.",
			"Tem a garantia do FGC: até R$ 250 mil por CPF em cada instituição.",
			"Alguns deixam resgatar em qualquer dia (liquidez diária); outros, só no vencimento.",
			"Paga IR, menor quanto mais tempo o dinheiro fica (de 22,5% a 15%), e IOF se sair antes de 30 dias.") +
			extra(rateNote(q, "cdi"), payFirst(q))
	}},
	{re(`^(tesouro selic|tesouro direto)$`), nil, func(q Question, _ string) string {
		return explain("**O Tesouro Selic é um título do governo que rende a Selic do dia**: é o investimento mais seguro do país, garantido pelo Tesouro Nacional.",
			"Dá para resgatar em qualquer dia útil, e o valor quase não oscila: serve para a reserva de emergência.",
			"Compra pelo app do banco ou da corretora, no Tesouro Direto.",
			"Paga IR como o CDB (de 22,5% a 15%, conforme o prazo) e uma taxa pequena de custódia da B3.") +
			extra(rateNote(q, "selic"), payFirst(q))
	}},
	{re(`^cdi$`), nil, func(q Question, _ string) string {
		return explain("**CDI é a taxa que os bancos cobram entre si por um dia**, e anda colada na Selic.",
			"É a régua da renda fixa: um CDB ou uma conta que paga \"100% do CDI\" rende o próprio CDI; \"110% do CDI\" rende 10% a mais que ele.",
			"Renda fixa abaixo de 100% do CDI costuma ter opção melhor, exceto LCI e LCA, que não pagam IR.") +
			extra(rateNote(q, "cdi"))
	}},
	{re(`^(selic|taxa selic|taxa basica( de juros)?)$`), nil, func(q Question, _ string) string {
		return explain("**A Selic é a taxa básica de juros do país**, definida pelo Banco Central no Copom, a cada 45 dias.",
			"Quando ela sobe, empréstimos e cartão ficam mais caros e a renda fixa rende mais; quando cai, o contrário.",
			"O Tesouro Selic e o CDI acompanham essa taxa.") +
			extra(rateNote(q, "selic"))
	}},
	{re(`^(ipca|inflacao)$`), nil, func(q Question, _ string) string {
		return explain("**O IPCA é a inflação oficial do Brasil**, medida pelo IBGE todo mês: quanto os preços subiram.",
			"Investimento que rende menos que o IPCA perde poder de compra: o dinheiro compra menos no fim.",
			"O Tesouro IPCA+ paga a inflação mais uma taxa fixa, e protege no longo prazo.") +
			extra(rateNote(q, "ipca"))
	}},
	{re(`^(poupanca|caderneta de poupanca)$`),
		re(`^(a )?poupanca ou (o )?(cdb|tesouro selic|tesouro)$|^(o )?(cdb|tesouro selic|tesouro) ou (a )?poupanca$|^(qual|o que) (e |rende )?(mais|melhor),? (a )?poupanca ou (o )?(cdb|tesouro selic|tesouro)$|^(vale a pena|compensa) (deixar|guardar|ter) (o )?(dinheiro )?na poupanca$`),
		func(q Question, _ string) string {
			note := rateNote(q, "")
			if q.Rates.Date != "" {
				note = fmt.Sprintf("Hoje (%s), a Selic está em %s ao ano: a poupança rende 0,5%% ao mês mais a TR.", dateBR(q.Rates.Date), RatePct(q.Rates.SelicBP))
				if q.Rates.SelicBP <= 850 {
					note = fmt.Sprintf("Hoje (%s), a Selic está em %s ao ano: a poupança rende 70%% dela mais a TR.", dateBR(q.Rates.Date), RatePct(q.Rates.SelicBP))
				}
			}
			return explain("**A poupança rende 0,5% ao mês mais a TR quando a Selic está acima de 8,5% ao ano**; com a Selic menor, rende 70% da Selic mais a TR.",
				"Não paga IR e tem a garantia do FGC (até R$ 250 mil por CPF em cada instituição).",
				"Rende no \"aniversário\": o dinheiro tirado antes do dia do mês em que entrou perde o rendimento daquele mês.",
				"Com a Selic alta, um CDB de liquidez diária com 100% do CDI ou o Tesouro Selic costumam render mais, mesmo depois do IR.") +
				extra(note)
		}},
	{re(`^(lci|lca|lci e (a )?lca|lci ou (a )?lca|letras? de credito( imobiliario| do agronegocio)?)$`), nil, func(q Question, _ string) string {
		return explain("**LCI e LCA são como um CDB, mas o dinheiro vai para crédito imobiliário (LCI) ou do agronegócio (LCA)**.",
			"Não pagam IR para pessoa física e têm a garantia do FGC (até R$ 250 mil por CPF em cada instituição).",
			"Costumam ter um prazo mínimo antes de poder resgatar (carência).",
			"Para comparar com um CDB, desconte o IR dele: 90% do CDI sem IR rende mais que 100% do CDI com IR.") +
			extra(payFirst(q))
	}},
	{re(`^(fgc|fundo garantidor( de creditos?)?)$`), nil, func(Question, string) string {
		return explain("**O FGC (Fundo Garantidor de Créditos) devolve o seu dinheiro se o banco quebrar**: até R$ 250 mil por CPF em cada instituição, com limite de R$ 1 milhão a cada 4 anos.",
			"Cobre conta, poupança, CDB, LCI e LCA.",
			"Não cobre o Tesouro Direto (garantido pelo governo), nem fundos, ações ou cripto.")
	}},
	{re(`^(credito )?rotativo( do cartao( de credito)?)?$|^juros do rotativo$`),
		re(`^(devo |posso |e se eu |o que acontece se eu |e ruim |tem problema )?(pagar|pago|pagando) (so |apenas |somente )?o (pagamento |valor )?minimo( do cartao( de credito)?| da fatura)?$`),
		func(q Question, f string) string {
			head := "**Rotativo é o que sobra da fatura quando você paga menos que o total**: vira uma dívida com os juros mais caros do mercado."
			if strings.Contains(f, "minimo") {
				head = "**Evite pagar só o mínimo: o resto da fatura vira rotativo**, a dívida com os juros mais caros do mercado."
			}
			return explain(head,
				"O melhor é pagar a fatura inteira. Se não der, pague o máximo que puder.",
				"Por lei (Lei 14.690/2023), juros e encargos do rotativo não podem passar de 100% do valor da dívida.",
				"No mês seguinte, o banco tem de oferecer um parcelamento, que costuma sair mais barato: compare o CET.") +
				extra(debtLines(q, "cartao", "rotativo")...)
		}},
	{re(`^(cheque especial|limite (da conta|do cheque especial))$`), nil, func(q Question, _ string) string {
		return explain("**O cheque especial é um limite que o banco põe na sua conta**: usou, paga juros por dia, dos mais caros do mercado.",
			"Os juros são limitados a 8% ao mês pelo Banco Central (151,82% ao ano), e ainda assim é muito caro.",
			"Para sair, troque por um empréstimo mais barato (consignado ou pessoal) ou negocie com o banco, e evite voltar ao limite.") +
			extra(debtLines(q, "cheque")...)
	}},
	{re(`^(juros compostos?|juros sobre juros)$`), nil, func(q Question, _ string) string {
		var note string
		if d := costlyDebt(q.Ledger); d != nil {
			note = fmt.Sprintf("A dívida %s, a %s ao mês, dobra em %s se nada for pago.", d.Name, RatePct(d.MonthlyRateBP), finance.Meses(doublingMonths(d.MonthlyRateBP)))
		}
		return explain("**Juros compostos são juros sobre juros**: o que rendeu num mês também passa a render nos seguintes.",
			"Guardando R$ 100 por mês a 1% ao mês, em 10 anos você deposita R$ 12 mil e chega a uns R$ 23 mil.",
			"Na dívida, o efeito é o mesmo, contra você.") +
			extra(note)
	}},
	{re(`^((o )?metodo )?(avalanche|bola de neve)( e (a |o )?(metodo )?(avalanche|bola de neve))?$`), nil, func(q Question, _ string) string {
		var note string
		if debts := openDebts(q.Ledger); len(debts) > 1 {
			p := finance.PlanDebts(debts)
			note = fmt.Sprintf("Nas suas dívidas: avalanche é %s; bola de neve é %s.", debtNames(p.Avalanche), debtNames(p.Snowball))
		}
		return explain("**São duas ordens para quitar várias dívidas**: pague o mínimo em todas e mande o que sobrar para uma só.",
			"Avalanche: primeiro a de maior juros. Economiza mais dinheiro.",
			"Bola de neve: primeiro a de menor saldo. Acaba com dívidas mais rápido e ajuda a não desanimar.") +
			extra(note)
	}},
	{re(`^(score( de credito)?|nome (sujo|negativado)|cadastro positivo)$`), nil, func(Question, string) string {
		return explain("**O score é uma nota de 0 a 1.000** que os birôs de crédito (como Serasa, Boa Vista e SPC) dão para dizer se você costuma pagar em dia. Bancos e lojas usam para dar crédito.",
			"Sobe com contas pagas em dia, dívidas atrasadas negociadas e o Cadastro Positivo ativo.",
			"Nome sujo é uma dívida atrasada registrada nesses birôs. Depois de pagar, o credor tem 5 dias úteis para pedir a retirada.")
	}},
	{re(`^(cet|custo efetivo total)$`), nil, func(Question, string) string {
		return explain("**CET é o Custo Efetivo Total**: todos os custos de um empréstimo ou financiamento numa taxa só (juros, tarifas, seguros e IOF).",
			"Para comparar ofertas, olhe o CET, não só a taxa de juros: uma taxa menor com seguro embutido pode sair mais cara.",
			"O banco é obrigado a informar o CET antes de você contratar.")
	}},
	{re(`^((emprestimo|credito) )?consignado$`), nil, func(q Question, _ string) string {
		var note string
		if d := costlyDebt(q.Ledger); d != nil {
			note = fmt.Sprintf("A dívida %s cobra %s ao mês: trocar por um consignado costuma baixar muito os juros. Compare o CET.", d.Name, RatePct(d.MonthlyRateBP))
		}
		return explain("**Consignado é um empréstimo com a parcela descontada direto do salário ou do benefício do INSS**.",
			"Como o banco quase não corre risco, os juros são bem menores que os do cartão e do cheque especial.",
			"A parcela tem um limite (a margem consignável) e compromete a renda por anos: use para trocar dívida cara, não para gastar mais.") +
			extra(note)
	}},
	{re(`^portabilidade( de (credito|divida|emprestimo)| da divida)?$`), nil, func(q Question, _ string) string {
		return explain("**Portabilidade de crédito é levar a sua dívida para outro banco** que cobre menos juros: o banco novo quita a antiga e você passa a pagar a ele.",
			"É um direito seu, sem custo. O banco atual pode fazer uma contraproposta.",
			"Vale para empréstimo pessoal, consignado e financiamento. Compare o CET.")
	}},
	{re(`^antecipa(r|cao) (de )?(as )?parcelas$`),
		re(`^(vale a pena|compensa|e bom|devo|e vantagem|vale) antecipar (as )?(parcelas?|prestac\w*)( do (emprestimo|financiamento|carro|consignado))?$`),
		func(q Question, _ string) string {
			var notes []string
			for _, d := range openDebts(q.Ledger) {
				notes = append(notes, fmt.Sprintf("%s: %s ao mês (%s ao ano).", d.Name, RatePct(d.MonthlyRateBP), RatePct(annualBP(d.MonthlyRateBP))))
			}
			if len(notes) > 0 {
				notes[0] = "Suas dívidas:\n- " + strings.Join(notes, "\n- ")
				notes = notes[:1]
			}
			if q.Rates.Date != "" {
				notes = append(notes, fmt.Sprintf("Guardado, o dinheiro rende perto do CDI: %s ao ano hoje (%s).", RatePct(q.Rates.CDIBP), dateBR(q.Rates.Date)))
			}
			return explain("**Antecipar parcelas dá desconto dos juros daquelas parcelas**: é um direito seu, pelo Código de Defesa do Consumidor.",
				"Vale a pena quando os juros da dívida são maiores do que o dinheiro renderia guardado, e depois de ter uma reserva para imprevistos.",
				"Comece pelas últimas parcelas: têm mais juros embutidos, então o desconto é maior.") +
				extra(notes...)
		}},
	{nil, re(`^o que (eu )?(faco|fazer|posso fazer) com (o |meu |a minha |minha |a )?(13|13o|13º|decimo terceiro|decimo|dinheiro extra|restituicao|plr|bonus)( salario)?$|^onde (eu )?(coloco|ponho|aplico|uso) (o |meu )?(13|13o|13º|decimo terceiro)( salario)?$`),
		func(q Question, _ string) string {
			steps := []string{"1. Dívidas caras: rotativo, cheque especial e as de juros mais altos."}
			var costly []string
			for _, d := range finance.PlanDebts(openDebts(q.Ledger)).Avalanche {
				if d.MonthlyRateBP >= costlyBP && len(costly) < 2 {
					costly = append(costly, fmt.Sprintf("%s: %s a %s ao mês", d.Name, money.Format(d.BalanceCents), RatePct(d.MonthlyRateBP)))
				}
			}
			if len(costly) > 0 {
				steps[0] += " " + strings.Join(costly, "; ") + "."
			}
			reserve := "2. Reserva de emergência: de 3 a 6 meses de gastos, num lugar seguro com resgate no mesmo dia."
			if e := BuildFacts(q.Ledger, q.Today).Emergency; e.AvgMonthlyOutCents > 0 {
				reserve = fmt.Sprintf("2. Reserva de emergência: a mínima, pelas suas contas, é %s (3 meses de gastos).", money.Format(e.MinCents))
			}
			goals := "3. Metas."
			var named []string
			for _, g := range finance.Goals(q.Ledger, q.Today) {
				if !g.Done && len(named) < 2 {
					named = append(named, fmt.Sprintf("%s: faltam %s", g.Name, money.Format(g.RemainingCents)))
				}
			}
			if len(named) > 0 {
				goals += " " + strings.Join(named, "; ") + "."
			}
			return "**Use na ordem de prioridade:**\n\n" + strings.Join(append(steps, reserve, goals), "\n") +
				"\n\nGuarde uma parte pequena para lazer: o plano fica mais fácil de seguir."
		}},
	{re(`^(regra |metodo )?50 ?/ ?30 ?/ ?20$|^regra 50 30 20$|^divisao do salario$`), nil, func(q Question, _ string) string {
		s := q.Ledger.Split()
		note := fmt.Sprintf("Sua divisão no app: necessidades %d%%, desejos %d%%, investimentos %d%%.",
			s[domain.BucketNecessidades], s[domain.BucketDesejos], s[domain.BucketInvestimentos])
		return explain("**A regra 50/30/20 divide a renda em três**: até 50% para necessidades (casa, mercado, contas), até 30% para desejos (lazer, delivery, compras) e 20% para investir ou quitar dívidas.",
			"Se as necessidades passam de 50%, o ajuste é nos gastos grandes (moradia, carro, planos), não só no cafezinho.",
			"Dá para mudar a divisão nos Ajustes do app.") +
			extra(note)
	}},
	{re(`^(reserva|reserva de emergencia|fundo de emergencia)$`),
		re(`^(como|onde) (montar|monto|fazer|faco|comecar|comeco|guardar|guardo|deixar|deixo|colocar|coloco|investir|invisto|aplicar|aplico) (a |uma |minha |a minha )?(reserva|reserva de emergencia)$`),
		func(q Question, _ string) string {
			notes := reserveLines(q)
			if d := costlyDebt(q.Ledger); d != nil {
				notes = append(notes, fmt.Sprintf("Com dívida cara (%s, a %s ao mês), junte só 1 mês de gastos e quite a dívida antes de completar a reserva.", d.Name, RatePct(d.MonthlyRateBP)))
			}
			return explain("**Reserva de emergência é o dinheiro para imprevistos** (perder o emprego, uma doença, o carro quebrar), para não precisar de empréstimo nem de cartão.",
				"Tamanho: de 3 a 6 meses de gastos (de 6 a 12 se a renda varia). Comece com 1 mês.",
				"Onde guardar: algo seguro e com resgate no mesmo dia, como o Tesouro Selic ou um CDB de liquidez diária com 100% do CDI ou mais, com a garantia do FGC.",
				"Não use ações nem fundos de risco: podem estar em baixa justo quando você precisar.") +
				extra(notes...)
		}},
	{re(`^(acoes|acao|bolsa|bolsa de valores)$`),
		re(`^(vale a pena|compensa|e bom|devo|e hora de) (investir|aplicar) (em |na )(acoes|bolsa|bolsa de valores|renda variavel)( agora| hoje| este ano| esse ano)?$`),
		func(q Question, _ string) string {
			return explain("**Ação é um pedaço de uma empresa**: o preço sobe e desce todo dia, e ninguém sabe para onde vai no curto prazo.",
				"Só vale para dinheiro que pode ficar 5 anos ou mais sem você precisar dele, espalhado em várias empresas (um fundo de índice ajuda).",
				"Antes: dívidas caras quitadas e reserva de emergência montada.",
				"O app não indica ação, fundo ou corretora.") +
				extra(payFirst(q))
		}},
}

// explain monta a explicação: a frase principal e os itens.
func explain(head string, items ...string) string {
	return head + "\n\n- " + strings.Join(items, "\n- ")
}

// extra junta as notas da pessoa (as vazias ficam de fora), cada uma num parágrafo.
func extra(notes ...string) string {
	var out string
	for _, n := range notes {
		if n != "" {
			out += "\n\n" + n
		}
	}
	return out
}

// rateNote: a taxa do dia ("cdi", "selic" ou "ipca"), ou mandar conferir.
func rateNote(q Question, which string) string {
	r := q.Rates
	if r.Date == "" {
		return "Confira a taxa do dia no site do Banco Central."
	}
	switch which {
	case "cdi":
		return fmt.Sprintf("Hoje (%s), o CDI está em %s ao ano.", dateBR(r.Date), RatePct(r.CDIBP))
	case "selic":
		return fmt.Sprintf("Hoje (%s), a Selic está em %s ao ano.", dateBR(r.Date), RatePct(r.SelicBP))
	case "ipca":
		return fmt.Sprintf("Nos 12 meses até %s, o IPCA foi de %s.", domain.ParseMonth(r.IPCAMonth).Label(), RatePct(r.IPCABP))
	}
	return ""
}

// costlyBP: a partir de 2% ao mês (quase 27% ao ano), a dívida custa mais do que
// qualquer investimento seguro rende.
const costlyBP = 200

// costlyDebt: a dívida com saldo de maior juros, se for cara; nil se não houver.
func costlyDebt(l domain.Ledger) *domain.Debt {
	var top *domain.Debt
	for _, d := range openDebts(l) {
		if d.MonthlyRateBP >= costlyBP && (top == nil || d.MonthlyRateBP > top.MonthlyRateBP) {
			top = &d
		}
	}
	return top
}

// payFirst: antes de investir, a dívida cara ("" sem dívida cara).
func payFirst(q Question) string {
	d := costlyDebt(q.Ledger)
	if d == nil {
		return ""
	}
	return fmt.Sprintf("A dívida %s cobra %s de juros ao mês (%s ao ano): quitar rende mais que qualquer investimento seguro.",
		d.Name, RatePct(d.MonthlyRateBP), RatePct(annualBP(d.MonthlyRateBP)))
}

// debtLines: as dívidas cujo nome tem uma das palavras ("cartao"), com saldo, juros e o
// aviso quando o mínimo nem cobre os juros.
func debtLines(q Question, words ...string) []string {
	var out []string
	for _, v := range finance.PlanDebts(openDebts(q.Ledger)).Debts {
		name := fold(v.Name + " " + v.Creditor)
		if !strings.Contains(name, words[0]) && (len(words) == 1 || !strings.Contains(name, words[1])) {
			continue
		}
		line := fmt.Sprintf("%s: %s a %s ao mês (%s ao ano).", v.Name, money.Format(v.BalanceCents), RatePct(v.MonthlyRateBP), RatePct(annualBP(v.MonthlyRateBP)))
		if !v.Payable && v.MinPaymentCents > 0 {
			line += fmt.Sprintf(" A parcela mínima (%s) nem cobre os juros (%s por mês): a dívida cresce.", money.Format(v.MinPaymentCents), money.Format(v.MonthlyInterestCents))
		}
		out = append(out, line)
	}
	return out
}

func debtNames(ds []domain.Debt) string {
	names := make([]string, len(ds))
	for i, d := range ds {
		names[i] = d.Name
	}
	return strings.Join(names, " → ")
}

// growth: quanto 1 vira depois de n meses a monthlyBP ao mês, em 1e8 (contas só com inteiros).
func growth(monthlyBP, months int) int64 {
	g := int64(100_000_000)
	for range months {
		g = (g*int64(10_000+monthlyBP) + 5_000) / 10_000
	}
	return g
}

// annualBP: os juros de um ano, em pontos-base, a partir dos juros ao mês (compostos).
func annualBP(monthlyBP int) int {
	return int((growth(monthlyBP, 12) - 100_000_000 + 5_000) / 10_000)
}

// doublingMonths: em quantos meses a dívida dobra sem pagamento nenhum.
func doublingMonths(monthlyBP int) int {
	for n := 1; n <= 1200; n++ {
		if growth(monthlyBP, n) >= 200_000_000 {
			return n
		}
	}
	return 0
}
