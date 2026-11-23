## ADDED Requirements

### Requirement: Resposta do app para pergunta de número
Uma pergunta de número que o app sabe calcular (gasto, quanto cabe no mês, plano e total das
dívidas, vencimentos, metas, reserva, assinaturas, comparação com o mês passado) SHALL ser
respondida pelo app, sem chamar a IA, com as mesmas contas das telas e para qualquer pessoa do
time. Pergunta de conselho, de ação, com duas perguntas ou com período que a conta não cobre
SHALL ir para a IA.

#### Scenario: Gasto com um lugar
- **WHEN** a pessoa pergunta "quanto gastei com uber?"
- **THEN** a resposta é "Gastos com Uber: …" com o total do mês, marcada como resposta do app, e nenhum pedido sai para a IA

#### Scenario: Cabe no mês
- **WHEN** a pessoa pergunta "consigo comprar um celular de 2 mil?"
- **THEN** o app compara R$ 2.000,00 com o que ainda cabe no mês e diz se cabe

#### Scenario: Pedido de conselho
- **WHEN** a pessoa pergunta "vale a pena antecipar parcelas?" ou "gastei 50 no mercado"
- **THEN** a pergunta vai para a IA, como antes

### Requirement: Perguntar à IA
Embaixo da resposta que o app deu sozinho, o Consultor SHALL oferecer "Perguntar à IA", que
manda a mesma pergunta para a IA (com apelidos, quando a IA é de fora do aparelho).

#### Scenario: A pessoa quer conversar
- **WHEN** a pessoa toca em "Perguntar à IA" depois da resposta do app
- **THEN** a mesma pergunta vai para a IA, e a resposta da IA aparece embaixo, sem o selo de resposta do app
