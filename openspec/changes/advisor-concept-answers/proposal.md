## Why

Depois das perguntas de número (`advisor-app-answers`), o que mais sobra para a IA grátis são
perguntas de conceito: "o que é CDB?", "como funciona o rotativo?", "vale a pena antecipar
parcelas?", "o que faço com o 13º?". A resposta muda pouco de pessoa para pessoa, e a IA gasta
de 2 a 5 chamadas de uns 4 mil tokens em cada uma. Um texto pronto e revisado responde na hora,
sem errar regra, e ainda pode trazer os números da pessoa.

## What Changes

- O app responde sozinho, sem IA, as perguntas de conceito mais comuns: CDB, Tesouro Selic, CDI,
  Selic, IPCA, poupança (e "poupança ou CDB?"), LCI e LCA, FGC, rotativo (e "devo pagar só o
  mínimo?"), cheque especial, juros compostos, avalanche e bola de neve, score e nome sujo, CET,
  consignado, portabilidade, antecipar parcelas, o que fazer com o 13º, a regra 50/30/20,
  reserva de emergência e ações.
- Cada explicação é curta e traz, quando existem, os números da pessoa: a dívida mais cara e os
  juros dela ao ano, o cartão que nem paga os juros, a reserva pelas contas do app, as metas, a
  divisão do salário. As taxas do dia vêm do Banco Central; sem elas, a resposta manda conferir.
- Conceito com outra coisa junto ("cdb e lci", "cdb do banco X para a minha reserva", "recebi o
  13º de R$ 4.500") vai para a IA. "Perguntar à IA" continua embaixo de toda resposta do app.

## Capabilities

### Modified Capabilities
- advisor: explicações prontas para perguntas de conceito, antes da IA.

## Impact

Só `internal/advisor` (`concepts.go`, e o roteador chama as explicações antes do filtro de
conselho), docs e um passo no e2e. Na amostra de 34 perguntas típicas, o app passa de 18 para 25.
