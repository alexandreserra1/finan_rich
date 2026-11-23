## Context

O roteador de perguntas de número (`app_answers.go`) manda para a IA tudo que parece conselho
("o que é", "como funciona", "vale a pena"), e é justamente assim que as perguntas de conceito
começam. A base `knowledge.go` tem as regras de finanças no Brasil, mas é escrita para a IA ler.

## Decisions

- **Texto próprio em `concepts.go`**, para a pessoa ler: uma frase principal em negrito e até 4
  itens. A base da IA (`knowledge.go`) continua separada: tem outro leitor e outro formato.
- **Reconhecer pelo assunto**: tira o começo que pede definição ("me explica o que é", "como
  funciona", "qual a diferença entre") e os artigos, e o que sobra precisa ser exatamente um
  assunto conhecido ("cdb", "tesouro selic", "avalanche e bola de neve"). Assim "o que é cdb e
  lci?" não casa. Algumas perguntas têm outras formas, também inteiras ("devo pagar só o mínimo
  do cartão?", "vale a pena antecipar parcelas?", "o que faço com o 13º?", "poupança ou CDB?").
- **Ordem no roteador**: depois do filtro de ação ("recebi o 13º de R$ 4.500" continua indo para
  a IA, que pode preparar o cartão) e antes do filtro de conselho.
- **Números da pessoa só quando existem**, e sempre do Go: os juros ao ano saem dos juros ao mês
  compostos, só com inteiros; "dobra em N meses" idem. Dívida cara é a de 2% ao mês ou mais
  (quase 27% ao ano), acima do que qualquer investimento seguro rende.
- **Sem número que muda**: as taxas do dia só com `Question.Rates`; sem elas, "confira a taxa do
  dia". Regras estáveis ficam no texto: Lei 14.690/2023 (rotativo até 100% da dívida), 8% ao mês
  no cheque especial, FGC de R$ 250 mil por CPF por instituição e R$ 1 milhão a cada 4 anos,
  poupança a 0,5% ao mês mais TR com a Selic acima de 8,5%, 5 dias úteis para limpar o nome.
  Nenhuma explicação indica produto, banco ou ação.

## Validation

- `concepts_test.go`: cada assunto pelos 3 do time, com as taxas do dia e os números da pessoa;
  sem taxas, manda conferir; sem dívida nem meta, sem a parte da pessoa; conceito com outra coisa
  junto vai para a IA; juros ao ano e "dobra em" conferidos à mão (10% ao mês = 213,84% ao ano).
- As perguntas da avaliação com IA que viraram explicação (ações, reserva, pagar só o mínimo)
  passam nos mesmos critérios, sem IA.
- e2e: "O que é CDB?" no app sai com a taxa do dia, sem chamar a IA.

## Risks

- Uma regra pode mudar (limite do FGC, regra da poupança, teto do rotativo). Os textos ficam
  num arquivo só, com as regras listadas acima, para revisar quando mudar.
- A pessoa pode querer uma resposta para o caso dela ("vale a pena antecipar a do carro?"): o
  texto dá o critério e os números dela, e "Perguntar à IA" fica a um toque.
