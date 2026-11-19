## ADDED Requirements

### Requirement: Gasto por categoria e por mês sem IA
"Quanto gastei com <categoria ou lugar>" SHALL trazer o total já calculado, no mês pedido
("mês passado", nome do mês) ou no atual, para a IA e na resposta sem IA.

#### Scenario: Categoria por uma palavra
- **WHEN** a pessoa pergunta "quanto gastei de água e luz mês passado?"
- **THEN** a conta pronta traz o total de "Contas da casa (luz, água, internet)" do mês anterior

### Requirement: Começo do zero cria os cartões
No começo do zero, uma mensagem com valores SHALL fazer a IA chamar a ferramenta, e a IA SHALL
NOT dizer que deixou um cartão pronto sem ter chamado a ferramenta.

#### Scenario: Renda informada
- **WHEN** a pessoa responde "Recebo 4.500 de salário no dia 5"
- **THEN** aparece o cartão "Nova renda fixa: Salário · R$ 4.500,00 todo dia 5"

#### Scenario: Afirmação falsa
- **WHEN** a IA responde "deixei pronto" sem chamar a ferramenta
- **THEN** o app pede de novo uma vez e, se repetir, a resposta diz que não conseguiu preparar
