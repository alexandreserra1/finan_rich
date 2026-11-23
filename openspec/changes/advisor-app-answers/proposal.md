## Why

A IA grátis da Rich é uma cota só, dividida por todas as pessoas: uns 200 mil tokens por dia
no Groq. Cada chamada do Otávio leva uns 4 mil tokens, e uma pergunta faz de 2 a 5 chamadas.
Isso dá de algumas dezenas a uns 200 perguntas por dia, somando todo mundo. Numa amostra de
34 perguntas típicas, o app só respondia sozinho as sugestões prontas, com o texto idêntico, e
nenhuma das 34 era assim. Para várias delas o app já tinha a conta certa (gasto com mercado ou
Uber, plano das dívidas, quanto guardar para a meta, vencimentos), mas só mostrava quando a IA
falhava.

## What Changes

- **O app responde primeiro as perguntas de número**, em Go e sem IA:
  - gasto com um lugar, uma categoria ou no mês;
  - quanto ainda cabe no mês e se cabe um gasto ("posso gastar 300?", "consigo comprar um
    celular de 2 mil?");
  - plano das dívidas (quando fico livre, qual pagar primeiro, e se eu pagar X a mais), quanto
    devo e os juros de cada dívida;
  - o que vence, as metas, a reserva, as assinaturas e a categoria em que mais gasta;
  - se gastou mais que no mês passado e o saldo do mês.

  A resposta vem na hora, com as mesmas contas das telas, para qualquer pessoa do time. Na
  dúvida, a pergunta vai para a IA como antes: pedido de conselho ou de ação, pergunta longa,
  duas perguntas numa só ou período que a conta não cobre.
- **"Perguntar à IA"** embaixo da resposta do app manda a mesma pergunta para a IA, que vê a
  resposta do app na conversa. O "Tentar de novo com a IA" também vai sempre para a IA.
- "2 mil" e "1,5 mil" passam a valer R$ 2.000 e R$ 1.500 nas contas prontas.
- `TestEval` continua medindo a IA (passa direto pelo app); um teste novo, sem IA, confere as
  respostas do app com os mesmos critérios.

## Capabilities

### Modified Capabilities
- advisor: respostas do app para perguntas de número, antes da IA, e "Perguntar à IA".

## Impact

`internal/advisor` (roteador e contas), `internal/askapi` e `internal/web/chat.go` (o pedido
leva `askAI`), `js/pages/advisor.js` e `js/cloud-ai.js` (botão e pedido). Sem mudança no banco:
no computador, a resposta do app é gravada com os números iguais ao texto, e o histórico a
reconhece por isso. Respostas prontas gravadas antes desta mudança perdem o selo "Resposta do
app" ao recarregar; o texto continua igual.
