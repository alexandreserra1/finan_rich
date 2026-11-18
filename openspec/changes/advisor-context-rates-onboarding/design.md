## Context

O Consultor já tinha as peças: contas prontas (`prefetch`), resposta sem IA (`Offline`),
cartões de confirmar (`Proposal`) e o time com ferramentas por personagem. As melhorias usam
essas peças em vez de criar caminhos novos.

## Decisions

- Resumo da conversa sem IA: as últimas 12 falas que ficaram de fora, cada uma com até 160
  letras (a conclusão da resposta vem primeiro, então o começo basta). Não gasta cota nem
  atrasa a resposta. Com apelidos, o histórico já chega anonimizado.
- Sugestões em Go (`finance`, não `advisor`): o `finance.wasm` roda no computador, no app e
  no site; o `advisor.wasm` só no app e no site. A lista fixa duplicada no JS saiu.
- Taxas: SGS 432 (meta Selic), 4389 (CDI anualizado) e 13522 (IPCA 12 meses), em pontos-base
  (`int`, sem `float`). A API aceita o navegador (CORS `*`). Falhou: segue sem taxas; o
  computador só tenta de novo depois de 10 minutos e espera no máximo 2 s por pergunta.
- Resposta pronta da taxa só para a pergunta curta ("qual a selic hoje?"); pedido de
  conselho ("vale a pena o tesouro selic?") vai para a IA, com as taxas nos dados.
- `llm.Friendly` traduz só o que reconhece (erro do provedor, rede, prazo); os outros erros
  já são do app, em português, e ficam como estão.
- Começo do zero pela IA, com as ferramentas de sempre e a nova `criar_conta_fixa`: só o
  Otávio, só até a 4ª resposta depois da abertura, e só as ferramentas de criar (o pedido
  fica menor que a cota por minuto do grátis).

## Validation

Testes Go (resumo, sugestões, taxas com Banco Central falso, `Friendly`, `criar_conta_fixa`,
modo do começo do zero) e e2e do app em Chromium e WebKit (sugestões pelos dados, "Tentar de
novo", Selic com Banco Central falso, começo do zero até gravar a conta fixa).

## Risks

- O Banco Central pode mudar o formato das séries: a busca falha e o consultor segue sem
  taxas, como antes.
- O começo do zero depende da IA seguir a ordem das perguntas; sem IA, o app mostra onde
  cadastrar à mão.
