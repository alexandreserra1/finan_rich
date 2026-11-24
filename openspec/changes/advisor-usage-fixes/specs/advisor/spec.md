## ADDED Requirements

### Requirement: Estimativa de gasto com a regra do Início
As respostas do app sobre quanto ainda dá para gastar SHALL seguir a mesma regra do Início: com
dívida pendente, cartão em aberto ou sem renda registrada no mês, SHALL dizer que não há
estimativa e o motivo, e SHALL NOT chamar a sobra de limite de gasto.

#### Scenario: Com dívida
- **WHEN** a pessoa tem uma dívida com saldo e pergunta "posso gastar 300?"
- **THEN** a resposta diz "Sem estimativa por enquanto" com o motivo do Início, sem "cabe no mês"

### Requirement: Taxas do dia no celular
No app do celular, as taxas do Banco Central SHALL chegar ao Consultor como no computador.

#### Scenario: Explicação com a taxa
- **WHEN** a pessoa pergunta "o que é o CDI?" no app, com internet
- **THEN** a resposta traz o CDI mais recente do Banco Central, com a data dele
