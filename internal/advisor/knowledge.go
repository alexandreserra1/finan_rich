package advisor

import (
	"strings"
)

// Base de conhecimento do consultor: o "treinamento" em finanças pessoais no
// Brasil. Em vez de mandar tudo a cada pergunta (gastaria a cota grátis e
// confundiria modelos pequenos), vão só os temas da pergunta, até maxTopics.
// Números que mudam com frequência (Selic, CDI, inflação) ficam de fora de
// propósito: a IA não deve inventá-los.

type topic struct {
	id    string
	words []string // palavras da pergunta (sem acento, minúsculas) que puxam o tema
	text  string
}

const maxTopics = 2

var knowledge = []topic{
	{"prioridades", []string{"comecar", "por onde", "o que faco", "organizar", "prioridade", "ajuda", "melhorar", "dica", "conselho", "economizar", "sobrar"}, `Ordem para usar o dinheiro (do mais urgente ao menos):
1. Contas essenciais em dia (moradia, luz, água, comida, transporte para trabalhar).
2. Sair do rotativo do cartão e do cheque especial: são os juros mais caros do mercado.
3. Reserva inicial pequena (1 mês de gastos essenciais), para não voltar às dívidas no primeiro imprevisto.
4. Quitar as outras dívidas caras (avalanche: maior juros primeiro).
5. Completar a reserva de emergência (3 a 6 meses de gastos; 6 a 12 para autônomo ou renda variável).
6. Metas e investimentos de longo prazo.
"Pague-se primeiro": no dia em que o salário cai, separe o valor da reserva ou do investimento antes de gastar.`},

	{"dividas", []string{"divida", "deve", "devendo", "juros", "rotativo", "cheque especial", "emprestimo", "renegoci", "quitar", "pagar primeiro", "nome sujo", "serasa", "atrasad", "consignado", "portabilidade", "avalanche", "bola de neve"}, `Dívidas:
- Avalanche (maior juros primeiro) economiza mais dinheiro; bola de neve (menor saldo primeiro) dá vitórias rápidas e ajuda quem desanima. Pague o mínimo em todas e jogue o resto numa só.
- Se a parcela mínima não cobre os juros do mês, a dívida cresce para sempre: é prioridade máxima.
- Rotativo do cartão: por lei, juros e encargos não podem passar de 100% do valor original da dívida (Lei 14.690/2023), mas continua sendo das dívidas mais caras. O parcelamento da fatura costuma sair mais barato que o rotativo; compare o CET.
- Cheque especial: juros limitados a 8% ao mês pelo Banco Central — ainda assim muito caro.
- Trocar dívida cara por barata: consignado, empréstimo com garantia ou portabilidade de crédito (direito do cliente: outro banco quita e cobra menos). Compare sempre o CET, não só a taxa.
- Renegociar: bancos e plataformas (Serasa Limpa Nome, feirões de renegociação) costumam dar descontos grandes para pagamento à vista de dívidas atrasadas. Só aceite parcela que cabe no orçamento.
- Enquanto quita, não faça dívida nova no cartão: use débito ou Pix.`},

	{"reserva", []string{"reserva", "emergencia", "imprevisto", "colchao", "guardar", "poupanca", "seguranca", "desempreg"}, `Reserva de emergência:
- Tamanho: 3 a 6 meses dos gastos essenciais (CLT); 6 a 12 meses para autônomo, MEI ou renda variável. Comece com 1 mês.
- Onde guardar: algo seguro e com resgate no mesmo dia — Tesouro Selic, CDB ou conta remunerada com liquidez diária pagando pelo menos 100% do CDI, em instituição coberta pelo FGC (até R$ 250 mil por CPF por instituição).
- Não use ações, fundos de risco nem previdência para reserva: podem estar em baixa justo quando você precisar.
- A poupança costuma render menos que essas opções quando a Selic está alta (acima de 8,5% ao ano ela rende 0,5% ao mês + TR).
- Reserva não é para viagem nem compra planejada: isso é meta.`},

	{"investir", []string{"invest", "render", "rendimento", "aplicar", "tesouro", "cdb", "lci", "lca", "acoes", "acao", "fii", "fundo", "cripto", "bitcoin", "dividendo", "previdencia", "aposentad", "pgbl", "vgbl", "renda fixa", "cdi", "selic"}, `Investimentos (conceitos; não recomende produto específico nem prometa retorno):
- Antes de investir em risco: sem dívidas caras e com reserva de emergência montada.
- Renda fixa: Tesouro Direto (Selic para curto prazo/reserva; IPCA+ protege da inflação no longo prazo; prefixado tem taxa fixa), CDB, LCI e LCA (isentas de IR para pessoa física). Compare pela rentabilidade líquida, prazo e liquidez.
- IR na renda fixa é regressivo pelo prazo: 22,5% até 180 dias, 20% até 360, 17,5% até 720 e 15% acima disso (confira se a regra mudou). IOF só nos primeiros 30 dias.
- Ações, fundos imobiliários e cripto oscilam muito: só dinheiro de longo prazo (5+ anos), diversificado, e sem tentar adivinhar o mercado. Fundos: olhe a taxa de administração.
- Previdência: PGBL só vale para quem faz a declaração completa do IR (deduz até 12% da renda bruta tributável); para os outros, VGBL. Olhe taxas.
- Juros compostos: aporte todo mês, mesmo pequeno, faz mais diferença que achar o investimento "perfeito".
- Taxas atuais (Selic, CDI, inflação) mudam: não cite números de taxa, diga para conferir a taxa do dia.`},

	{"orcamento", []string{"orcamento", "salario", "dividir", "divisao", "50/30/20", "planej", "controle", "gasto", "gastando", "ritmo", "sobra", "falta dinheiro", "nao sobra", "aperto", "cortar", "reduzir", "assinatura", "delivery", "ifood", "uber", "mercado", "impulso"}, `Orçamento e cortes:
- 50/30/20: necessidades até 50% da renda, desejos até 30%, investir 20%. Se as necessidades passam de 50%, o ajuste é estrutural (moradia, carro, planos) e não só "cortar cafezinho".
- Comece pelos maiores gastos variáveis e pelos que cresceram contra a média; um corte grande vale mais que dez pequenos.
- Assinaturas: liste todas, cancele as que não usou no último mês; some o custo por ano para decidir.
- Delivery e transporte por app: defina um limite semanal; cozinhar em lote e combinar corridas costuma cortar pela metade.
- Compra por impulso: regra das 48 horas (espere 2 dias antes de comprar o que não é essencial) e pense no custo em horas de trabalho.
- Divida o que ainda pode gastar pelos dias que faltam no mês: vira um limite diário fácil de seguir.
- Renda variável: planeje com a média dos últimos 12 meses (ou o pior mês) e pague a si mesmo um "salário" fixo.`},

	{"cartao", []string{"cartao", "fatura", "limite", "parcel", "anuidade", "cashback", "milhas", "credito"}, `Cartão de crédito:
- Pague sempre a fatura inteira; o mínimo joga o resto no rotativo.
- Use o cartão como forma de pagamento, não como renda extra: o limite não é seu dinheiro.
- Parcelado sem juros só compensa se o preço à vista for o mesmo; com desconto à vista, quase sempre é melhor pagar à vista.
- Some as parcelas que já vão cair nos próximos meses antes de parcelar algo novo.
- Melhor dia de compra: logo depois do fechamento da fatura (mais tempo até pagar).
- Anuidade: negocie ou troque por cartão sem anuidade; cashback e milhas só valem sem gastar a mais.`},

	{"compras", []string{"comprar", "compra grande", "financi", "carro", "casa", "imovel", "apartamento", "consorcio", "a vista", "parcelado", "celular novo", "viagem", "sac", "price"}, `Compras grandes e financiamento:
- Junte para a entrada (quanto maior, menos juros) e compare sempre o CET, não só a taxa.
- Tabela SAC (parcela começa alta e cai) paga menos juros no total que a Price (parcela fixa).
- Consórcio não tem juros, mas tem taxa de administração e não garante quando você recebe.
- À vista com desconto costuma ganhar do parcelado; se for parcelar sem juros, deixe o dinheiro rendendo enquanto paga.
- Carro tem custo além da parcela: seguro, IPVA, manutenção, combustível, depreciação.
- Transforme a compra numa meta no app: valor, prazo e quanto guardar por mês.`},

	{"renda_extra", []string{"13o", "13º", "decimo", "ferias", "restituicao", "bonus", "plr", "renda extra", "dinheiro extra", "heranca", "fgts"}, `Dinheiro extra (13º, férias, restituição, PLR, bônus):
- Use na ordem de prioridades: primeiro dívidas caras, depois reserva de emergência, depois metas.
- Separe uma parte pequena para lazer, para o plano ser sustentável.
- Antecipar o 13º ou a restituição só vale se for para quitar dívida mais cara que os juros da antecipação.`},

	{"golpes", []string{"golpe", "pix errado", "fraude", "clonad", "phishing", "boleto falso", "estelionat"}, `Golpes:
- Banco nunca pede senha, código por SMS ou para "testar" Pix. Desconfie de urgência.
- Pix por golpe ou fraude: avise o banco na hora e peça o MED (Mecanismo Especial de Devolução); faça boletim de ocorrência.
- Cartão clonado: bloqueie no app do banco e conteste as compras.`},
}

// knowledgeFor escolhe os temas da pergunta (até maxTopics, na ordem da
// lista). Sem nenhum tema, vai "prioridades": é a base de qualquer conselho.
func knowledgeFor(question string) string {
	q := " " + fold(question) + " "
	var out []string
	for _, t := range knowledge {
		for _, w := range t.words {
			if strings.Contains(q, w) {
				out = append(out, t.text)
				break
			}
		}
		if len(out) == maxTopics {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, knowledge[0].text)
	}
	return strings.Join(out, "\n\n")
}
