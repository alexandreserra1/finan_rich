## ADDED Requirements

### Requirement: Conversa longa não perde o começo
O Consultor SHALL mandar à IA um resumo curto das mensagens que passam do histórico, sem
chamar a IA para fazer o resumo.

#### Scenario: Conversa com mais de 12 mensagens
- **WHEN** a conversa tem 80 mensagens
- **THEN** a pergunta leva as últimas 12 inteiras e as 12 falas anteriores resumidas em `<conversa_anterior>`

### Requirement: Sugestões pelos dados
As sugestões da tela do Consultor SHALL partir dos dados da pessoa e SHALL NOT sugerir
pergunta que não se aplica.

#### Scenario: Dívida cara
- **WHEN** a pessoa tem a dívida Cartão com juros
- **THEN** o Otávio sugere "Como saio da dívida Cartão mais rápido?"

#### Scenario: Nada cadastrado
- **WHEN** não há lançamentos, contas fixas, dívidas nem metas
- **THEN** a primeira sugestão do Otávio é "Me ajuda a começar do zero?" e não aparece "Qual dívida devo pagar primeiro?"

### Requirement: Taxas do dia do Banco Central
O Consultor SHALL usar Selic, CDI e IPCA da API pública do Banco Central quando conseguir
buscá-los, sem enviar nenhum dado da pessoa.

#### Scenario: Pergunta da taxa
- **WHEN** a pessoa pergunta "qual a selic hoje?" e o app tem as taxas
- **THEN** o app responde com as taxas e a data, sem chamar a IA

#### Scenario: Banco Central fora do ar
- **WHEN** a busca das taxas falha
- **THEN** o Consultor responde como antes, sem taxas

### Requirement: Falha da IA clara e com nova tentativa
O motivo da falha da IA SHALL aparecer sem endereço nem texto técnico, e a tela SHALL
oferecer "Tentar de novo", menos quando o limite do grátis acabou.

#### Scenario: Internet cai no meio
- **WHEN** a conexão com a IA cai
- **THEN** a resposta do app diz "não deu para falar com a IA: confira a internet" e tem o botão "Tentar de novo com a IA"

### Requirement: Começo do zero pela conversa
Com o Otávio, "Me ajuda a começar do zero?" SHALL levar a IA a perguntar renda, contas
fixas, dívidas e meta, uma por vez, preparando cartões para a pessoa confirmar.

#### Scenario: Renda
- **WHEN** a pessoa responde "4.500 no dia 5"
- **THEN** aparece o cartão "Nova renda fixa: Salário · R$ 4.500,00 todo dia 5" e, confirmado, vira conta fixa

#### Scenario: Fim do começo
- **WHEN** a pessoa já deu as 4 respostas
- **THEN** a conversa volta às ferramentas de sempre do personagem
