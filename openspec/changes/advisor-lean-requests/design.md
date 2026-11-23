## Context

O `Ask` sempre mandava as ferramentas do personagem e o `toolsPrompt`. Medido com os dados da
avaliação (`evalLedger`), a primeira chamada do Otávio tinha: instruções ~1.120 tokens, voz ~315,
regras das ferramentas ~270, definições ~1.010, conhecimento ~165 e dados com a pergunta ~1.150.
Nas 6 perguntas da avaliação que ainda vão para a IA, cada uma fez uma chamada só: as idas e
voltas eram das perguntas de número, que o app agora responde.

## Decisions

- **Quatro modos no `Ask`**, pela pergunta:
  - começo do zero: igual a antes;
  - ação (`actionRe` do roteador, mais "quero/preciso/pode … lançar/criar/juntar…"): as do
    personagem;
  - busca (`lookupRe`: gasto, compra, lançamento, categoria, valor, errado, repetido): buscar,
    alterar e apagar, com um `lookupPrompt` curto;
  - o resto: sem ferramentas.
- **Voz sem as linhas das ferramentas** (`leanSystemFor`): tira da voz as linhas que citam o nome
  de uma ferramenta. Assim a IA não tenta chamar o que não foi no pedido. A linha de dívidas do
  Otávio virou duas: a regra (ordenar pela taxa) fica, e o "use simular_dividas" sai. No pedido
  com ferramentas, a voz é a mesma de antes.
- **Rede de segurança**: sem ferramentas, uma resposta com "deixei pronto" (`falseClaimRe`) faz
  uma nova tentativa com as ferramentas do personagem e o aviso (`falseClaimNudge`); se repetir,
  vale a nota de que não deu. No máximo 2 pedidos, como no caminho com ferramentas.
- **Instrução do conhecimento**: "traga as 2 ou 3 que valem para a pergunta, aplicadas ao caso
  do usuário; não copie o bloco inteiro" (era "aplique ao caso do usuário, não copie").

## Validation

IA grátis real (`FINAN_EVAL_PRESET=finan`), dados da avaliação com apelidos, as 6 perguntas da
avaliação que ainda vão para a IA (conselho, Selic sem taxas, 13º, piada, financiamento), com um
teste descartável que conta chamadas e tokens:

| Versão | Nota | Chamadas | Tokens enviados |
|---|---|---|---|
| Antes | 24 de 26 | 6 | ~24 mil |
| Sem ferramentas, instrução antiga | 21 de 26 | 6 | ~17,1 mil |
| Sem ferramentas, instrução nova | 23 de 26 | 6 | ~17,2 mil |

A pergunta do financiamento caiu de 6 e 4 de 6 (antes) para 3 de 6 três vezes seguidas: sem as
ferramentas, o modelo deixou de usar o conhecimento (CET, entrada, custos). Com a instrução nova:
5, 5 e 4 de 6. A do 13º perde o mesmo critério nas duas versões.

## Risks

- Uma ação que os padrões não reconhecem ("aquele cinema foi 30") vai sem ferramentas; a rede de
  segurança cobre quando a IA diz que deixou pronto, com um pedido a mais.
- A avaliação é pequena e o modelo grátis varia entre respostas: a nota foi conferida com 6
  perguntas e algumas repetições, não com uma amostra grande.
