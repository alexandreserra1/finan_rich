## Context

O Consultor já tinha contas prontas (`prefetch`) e respostas sem IA (`Offline`); o começo do
zero (#27) dependia só das instruções para a IA chamar as ferramentas.

## Decisions

- **Categoria ou lugar**: a categoria casa pelo nome inteiro ou por palavra de 5 letras ou mais
  (mais "agua"); o lugar casa pela chave da descrição. Vence o trecho mais longo; empatado, a
  categoria (é o mais amplo, como "mercado").
- **Mês**: só o nome é o mais recente que já começou (em novembro, "dezembro" é do ano
  passado), ao contrário das metas, em que o nome do mês é o próximo.
- **`tool_choice: "required"` só no começo do zero e só no 1º passo**, quando a mensagem tem
  número. Fora disso, obrigar poderia criar cartão sem pedido. Se o provedor recusar o campo,
  o caminho de sempre tira as ferramentas.
- **Afirmação falsa**: regex curta ("deixei pronto", "preparei o cartão", "pronto para
  confirmar") e só quando nenhuma ferramenta foi chamada na pergunta. Frases como "conta
  criada" não entram: no começo do zero o `required` já resolve, e em conversa comum
  "criada" pode falar de algo antigo.
- **Uma nova tentativa no máximo** em cada caso (cota grátis).

## Validation

Testes Go com IA falsa e `TestEvalOnboarding` com a IA real do grátis da Rich (5 turnos):
antes 7 de 10 critérios e nenhum cartão em 2 turnos; com o `required`, 9 de 10; com o lembrete
dos outros itens, 10 de 10 (salário, aluguel, internet e meta viraram cartões).

## Risks

- Dois itens na mesma mensagem podem sair como um só cartão; o lembrete resolveu na avaliação,
  mas o modelo grátis muda. A pessoa vê o que faltou e repete.
