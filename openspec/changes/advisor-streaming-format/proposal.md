## Why

No app do celular e no site, o Consultor com IA na nuvem mostrava a resposta só no fim,
depois de vários segundos com os três pontinhos. O motivo era a anonimização: um apelido
("Dívida 1") podia chegar partido entre dois pedaços do streaming, então a resposta inteira
esperava. O mesmo valia no computador quando a IA configurada era de nuvem.

Além disso, toda resposta seguia o mesmo formato de plano (conclusão em negrito, passos
numerados, "hoje no app"), até para perguntas diretas como "quanto gastei com iFood?".

## What Changes

- A resposta anonimizada passa a aparecer aos poucos: só o fim que ainda pode ser um
  apelido partido espera o próximo pedaço (`RevealStream` em `internal/advisor/anon.go`).
- O `advisor.wasm` aceita uma função que recebe os pedaços; `cloud-ai.js` e a tela do
  Consultor mostram o texto enquanto ele chega.
- As instruções do consultor separam pergunta direta (resposta curta, número em negrito,
  sem plano) de pedido de conselho (formato de plano de antes).
- A avaliação com IA real (`eval_test.go`) passa a medir o tamanho da resposta direta.

## Capabilities

### New Capabilities
- advisor: resposta do Consultor aos poucos sem expor apelidos, e formato pelo tipo de pergunta.

## Impact

`internal/advisor`, `internal/askapi`, `cmd/finanadvisor`, `cloud-ai.js`, `pages/advisor.js`
e o e2e do app. Sem mudança de dados, dependências, intermediário ou publicação. O marcador
das instruções (`PromptMarker`) não muda.
