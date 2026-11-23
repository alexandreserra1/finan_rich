## ADDED Requirements

### Requirement: Explicação pronta para pergunta de conceito
Uma pergunta de conceito conhecida ("o que é CDB?", "como funciona o rotativo?", "vale a pena
antecipar parcelas?", "o que faço com o 13º?") SHALL ser respondida pelo app, sem chamar a IA,
com um texto revisado e os números da pessoa quando existem. Taxas do dia SHALL vir do Banco
Central ou a resposta SHALL mandar conferir, sem inventar número. Conceito com outra coisa junto
SHALL ir para a IA.

#### Scenario: Conceito com a taxa do dia
- **WHEN** a pessoa pergunta "o que é CDB?" e o app tem as taxas do dia
- **THEN** a resposta explica o CDB, diz o CDI de hoje e, se houver dívida cara, que quitar rende mais

#### Scenario: Rotativo de quem paga só o mínimo
- **WHEN** a pessoa pergunta "devo pagar só o mínimo do cartão?" e o mínimo dela não cobre os juros
- **THEN** a resposta explica o rotativo e diz que a parcela mínima dela nem cobre os juros

#### Scenario: Conceito com outra coisa junto
- **WHEN** a pessoa pergunta "o que é cdb e lci?"
- **THEN** a pergunta vai para a IA
