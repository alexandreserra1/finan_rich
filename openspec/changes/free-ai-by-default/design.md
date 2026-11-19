## Context

O intermediário (`cmd/api`, Vercel) guarda as chaves dos provedores grátis e limita por
aparelho, por IP e por rajada. O app manda a pergunta com nomes trocados por apelidos e um
identificador aleatório do aparelho como `Bearer`.

## Decisions

- **Ligado de fábrica no app e no site.** Esses aparelhos não têm IA local; sem a nuvem, o
  Consultor não conversa. A pergunta só sai quando a pessoa pergunta, com apelidos, e a tela
  diz para onde vai antes do envio.
- **Um clique no computador, não automático.** O computador promete que nada sai dele sem a
  pessoa escolher. Ligar sozinho também faria as suítes e2e (que sobem o programa sem escolher
  IA) chamarem a IA de verdade.
- **Desligar é uma escolha guardada** (`{ off: true }`), para a grátis não voltar sozinha.
  Apagar todos os dados do aparelho volta ao de fábrica.
- **O padrão não é gravado**: sem nada guardado, `config()` devolve o grátis. Assim uma versão
  futura pode trocar o padrão sem migrar dados.

## Validation

e2e do app em Chromium e WebKit: de fábrica o Consultor responde pela IA grátis (intermediário
falso, `Bearer` aleatório, nada vai para a real); desligar fica guardado; um toque liga de novo.
Go: o `/api/chat` do computador oferece o grátis quando a IA local não está pronta e some
depois do clique.

## Risks

- **Cota compartilhada.** Com o grátis ligado para todos, a cota do Groq (algumas centenas de
  perguntas por dia, com o cache) acaba antes. O intermediário já passa para os outros
  provedores grátis se as chaves estiverem na Vercel; acabando tudo, o time responde com as
  contas do app.
- **Abuso.** O intermediário é aberto (o código é público). Os limites do intermediário (por
  aparelho, IP e rajada) e a regra de limite por IP do Firewall da Vercel (ativa desde 05/11,
  conferida em 19/11: depois de 11 pedidos seguidos, 429 com `x-vercel-mitigated: deny`)
  seguram um script.
