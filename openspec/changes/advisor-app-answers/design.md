## Context

O Consultor já tinha as contas do app em Go: `prefetch` (contas prontas que vão junto do pedido
à IA) e `Offline` (a resposta de cada personagem quando a IA falha). `Prepared` respondia sem
IA só a lista fixa de sugestões, com o texto exatamente igual, e as taxas do dia.

## Decisions

- **Roteador em Go (`app_answers.go`)**, compilado também no `advisor.wasm`: vale no computador, no
  app e no site. Cada assunto é uma expressão sobre a pergunta sem acentos, mais a resposta
  com as contas que já existem (`placeSpending`, `simulateDebts`, `upcoming`, `prefetch` do
  Tito, `Insights`, `Summarize`). O primeiro assunto que casa decide; se faltar o que a conta
  precisa (lugar que não existe, metas sem nenhuma em andamento quando o assunto exige), a
  pergunta vai para a IA.
- **Na dúvida, IA.** Antes dos assuntos, a pergunta vai para a IA quando:
  - pede conselho, decisão ou explicação ("vale a pena", "como faço", "o que é", "por que");
  - pede uma ação ("gastei 50", "paguei", "lança", "cria", "apaga"): a IA prepara o cartão;
  - tem mais de 100 letras, mais de uma frase ou duas perguntas ("quando fico livre e quanto
    pago de juros?"; "se eu guardar 400, quando chego?" é uma pergunta só);
  - fala de um período que a conta não cobre (ano, semana, média, desde) ou, no resumo, na
    comparação e nos vencimentos, de outro mês ("mês passado", "agosto", "mês que vem");
  - pede uma escolha ("luz ou água?") ou uma tendência ("qual gasto mais cresceu?");
  - cita um tipo de dívida que a pessoa não cadastrou ("juros do cheque especial"): é conceito;
  - é o começo do zero com o Otávio.
- **O mesmo para todo o time.** A conta não depende de quem responde; antes, a lista pronta era
  por personagem. Depois da resposta do app não aparece o "Isso é com a Lia": ela daria a
  mesma resposta.
- **"Perguntar à IA" reenvia a pergunta** com `askAI`, como o "Tentar de novo com a IA" já
  fazia: a pergunta aparece de novo na conversa e a IA vê a resposta do app no histórico. Sem
  o cartão "Os números do app", que está logo acima.
- **Sem mudança no banco do computador.** A resposta que o app deu sozinho é gravada com
  `numbers` igual ao texto. O histórico marca como do app quando o texto termina com os
  números, como já fazia com a resposta sem IA, e não mostra o cartão quando são iguais.
  Antes o histórico recalculava pela pergunta, o que daria errado com o roteador (depende dos
  dados de hoje) e com "Perguntar à IA" (a mesma pergunta teve resposta da IA).

## Validation

- `app_answers_test.go`: tabela de perguntas que o app responde (com o número esperado) e de
  perguntas que vão para a IA (conselho, ação, duas perguntas, período, lugar desconhecido).
- As perguntas da avaliação com IA que o app agora responde passam nos mesmos critérios
  (`must`/`never`) sem IA; o `TestEval` manda direto para a IA (`AskAI`).
- e2e: pelo app, "Quanto gastei na Feira?" sai do app sem chamar a IA, e "Perguntar à IA" leva
  a mesma pergunta com apelidos; pelo celular pareado, o mesmo com a IA do computador.

## Risks

- Uma pergunta de número com nuance ("posso gastar 300?" com uma dívida cara) recebe só a
  conta e um lembrete da dívida mais cara; a conversa fica a um toque, no "Perguntar à IA".
- Expressões novas da pessoa caem na IA, como hoje. A tabela de testes cresce com os casos
  reais que aparecerem.
