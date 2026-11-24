## Why

Num teste de uso do app no emulador Android, de uma instalação limpa, apareceram três problemas
fora do Consultor (ENG-011):
1. O lançamento novo ia para a primeira categoria da lista ("Uber" em "Contas da casa"), e o
   histórico errado passava a ensinar a sugestão seguinte.
2. "Permitir notificações?" aparecia depois de cada gasto salvo, até a pessoa marcar "não
   perguntar de novo".
3. Depois de liberar a notificação, o app abria a tela de sistema "Alarmes e lembretes".

## What Changes

- No lançamento novo, a categoria vem da descrição enquanto a pessoa não escolhe uma, com as
  regras da importação (histórico da pessoa, depois palavras-chave), pelo `finance.wasm`.
- Sem a permissão de notificação, os lembretes do time ficam desligados: o app pede uma vez,
  os Ajustes mostram desligado, e ligar pede de novo e avisa quando não vem.
- Os lembretes vão sem alarme exato (às 9h, alguns minutos depois serve).

## Capabilities

### Modified Capabilities
- frontend: categoria sugerida no lançamento; lembretes sem pedidos repetidos.

## Impact

`internal/wasmapi` e `cmd/finanwasm` (`SuggestCategory`), `js/finance.js`,
`js/pages/transactions.js`, `js/native.js`, `js/pages/device.js`, e2e e docs.
