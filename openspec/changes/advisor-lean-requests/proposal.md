## Why

Com as perguntas de número e de conceito respondidas pelo app (`advisor-app-answers`,
`advisor-concept-answers`), o que ainda vai para a IA grátis é sobretudo conselho e conversa.
Mesmo assim, todo pedido levava as definições das ferramentas do personagem (uns 1.000 tokens no
Otávio, com 9 ferramentas), as instruções de uso delas e as linhas da voz sobre elas: uns 4 mil
tokens por chamada, a maior parte sem uso.

## What Changes

- Conselho e conversa vão sem ferramentas e sem as linhas da voz que falam delas: uns 2,6 mil
  tokens, uma chamada só.
- Pedido de ação (lançar, corrigir, apagar, criar, pagar, guardar, "quero juntar") leva as
  ferramentas do personagem, como antes.
- Busca nos lançamentos (gasto, compra, valor, categoria, lançamento errado ou repetido) leva só
  buscar, alterar e apagar.
- O começo do zero continua igual.
- Se a IA disser que deixou um cartão pronto sem ter as ferramentas, era uma ação que o app não
  reconheceu: uma nova tentativa vai com as ferramentas do personagem e o aviso de sempre.
- As instruções pedem para trazer do bloco de conhecimento as 2 ou 3 regras que valem para a
  pergunta (sem isso, o pedido menor deixava de citar CET, entrada e custos num financiamento).

## Capabilities

### Modified Capabilities
- advisor: ferramentas só quando a pergunta pede.

## Impact

`internal/advisor` (`Ask`, `leanSystemFor`, uma linha da voz do Otávio dividida em duas, uma
frase das instruções), testes Go que usavam "oi" para ter ferramentas, um passo do e2e e docs.
