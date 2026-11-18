## Why

O Consultor esquecia o começo de conversas longas (só as últimas 12 mensagens iam junto),
sugeria sempre as mesmas três perguntas, mandava "confira a taxa do dia" mesmo com a taxa
pública do Banco Central à mão, mostrava erros técnicos ("Post https://...: dial tcp") sem
um jeito de tentar de novo e não ajudava quem está começando do zero.

## What Changes

- O que passa das 12 mensagens vira um resumo curto (sem chamar a IA), em
  `<conversa_anterior>` na pergunta.
- As sugestões da tela vêm dos dados (`finance.Suggestions`, no `finance.wasm`): a dívida mais
  cara, a categoria que disparou, a meta atrasada; com tudo vazio, "Me ajuda a começar do zero?".
- Selic, CDI e IPCA do Banco Central (`internal/rates`, API pública, sem chave e sem dado da
  pessoa) vão nos dados da IA; "qual a selic hoje?" tem resposta pronta, sem IA. O computador
  guarda por 6 horas; o app e o site, por dia.
- O motivo da falha da IA aparece em palavras do dia a dia (`llm.Friendly`), com o botão
  "Tentar de novo" (menos quando acabou o limite do grátis, em que não adianta).
- O começo do zero: com o Otávio, a IA pergunta renda, contas fixas, dívidas e meta, uma por
  vez, e prepara os cartões; a ferramenta nova `criar_conta_fixa` cria recorrências. Sem IA, o
  app mostra onde cadastrar à mão.

## Capabilities

### New Capabilities
- advisor: memória da conversa, sugestões pelos dados, taxas do dia, erros claros e começo do zero.

## Impact

`internal/advisor`, `internal/finance`, `internal/llm`, `internal/rates` (novo), `internal/askapi`,
`internal/web` (cache das taxas), `internal/wasmapi`, `cmd/finanwasm`, `cmd/finanadvisor`,
`cmd/finanrich`, `cloud-ai.js`, `finance.js`, `notes-save.js`, `pages/advisor.js`, `app.css` e o
e2e do app. Rede nova: só `api.bcb.gov.br` (GET das séries 432, 4389 e 13522). Sem dependência
nova e sem mudança no intermediário.
