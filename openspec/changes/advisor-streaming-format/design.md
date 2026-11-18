## Context

Os apelidos têm sempre duas palavras ("Loja AB", "Dívida 12", "Categoria 3"). O `Reveal`
troca apelidos por nomes com regex de palavra inteira, então um apelido só pode ser trocado
quando está completo e seguido de algo que não é letra nem número.

## Decisions

- `RevealStream` guarda o texto bruto e libera até um corte seguro: olha só as duas últimas
  palavras; segura a última se o texto termina nela (pode estar partida) e a que é começo de
  apelido (a primeira palavra de cada apelido, sem acento), com tudo o que vem depois. O
  corte cai sempre no começo de uma palavra, então nenhum apelido fica dos dois lados.
- Um byte solto (letra cortada no meio) conta como palavra e espera. Na prática os
  provedores mandam texto já decodificado do JSON.
- O regex do `Reveal` é montado uma vez por conversa (`sync.Once`), porque agora roda a
  cada pedaço.
- A resposta final continua sendo `Reveal` do texto inteiro; a tela troca o texto parcial
  por ela no fim, como já fazia no computador.
- Formato: a regra fica nas instruções fixas (`SystemPrompt`), com um exemplo curto de
  pergunta direta sem nome de loja (para o modelo não copiar um apelido do exemplo).

## Validation

Teste Go com o mesmo texto partido em pedaços de 1 a 7 bytes, comparando com o `Reveal`
inteiro; teste do `Answer` e do `askapi.Ask` com o parser SSE real; e2e do app em Chromium
e WebKit com a IA falsa mandando "Loj" + "a AB" com 1,5 s de intervalo. O formato novo só
se prova com IA real (`TestEval`), que gasta cota e não roda no CI.

## Risks

- Se um intermediário juntar o streaming (buffer), a resposta volta a chegar de uma vez,
  sem erro. O intermediário da Rich repassa com `Flush`.
- Se a IA falhar no meio, o texto parcial aparece e depois é trocado pela resposta do app,
  como já acontecia no computador com IA local.
