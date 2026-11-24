## Why

Num teste de uso das mudanças do Consultor (#42 a #44), com o programa do computador e um APK de
teste no emulador Android, apareceram quatro problemas que os testes automáticos não pegavam:
1. "Posso gastar 300?" respondia "Cabe no mês" mesmo com dívida pendente, ao contrário do
   Início, que nesses casos não mostra a sobra como dinheiro para gastar (`finance.Home`).
2. Lançamento com uma categoria que não existe mais saía sem nome (": R$ 610,00"), nas contas
   e nas respostas do app; as telas já mostravam "Sem categoria".
3. As explicações diziam "Hoje (19/11/2026)" com a data da última taxa do Banco Central, num
   dia 23.
4. No celular, as taxas do Banco Central nunca chegavam (ENG-010).

## What Changes

- "Quanto ainda posso gastar?" e "posso gastar X?" seguem a regra do Início: com dívida,
  cartão em aberto ou sem renda registrada no mês, dizem "Sem estimativa por enquanto" e o
  motivo, sem chamar a sobra de limite. Com a estimativa valendo, dizem que é estimativa do app.
- O resumo do mês, as tendências por categoria e o rótulo dos lançamentos dizem
  "Sem categoria" quando a categoria não existe.
- As taxas aparecem como "CDI mais recente do Banco Central (data)".
- No app, os pedidos ao Banco Central vão pelo `fetch` do WebView.

## Capabilities

### Modified Capabilities
- advisor: estimativa com a regra do Início; taxas no celular.

## Impact

`internal/advisor`, `internal/finance` (summary, insights), `internal/domain` (constante do
nome), `js/cloud-ai.js`, `e2e/app.test.mjs`, docs (ARCHITECTURE, ROADMAP, known-issues).
