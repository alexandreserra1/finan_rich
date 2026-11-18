## 1. Resposta aos poucos
- [x] 1.1 `RevealStream` que nunca mostra apelido partido, com teste de pedaços de 1 a 7 bytes.
- [x] 1.2 `Answer` anonimizado repassa os pedaços; `askapi.Ask` e `advisor.wasm` aceitam a função.
- [x] 1.3 `cloud-ai.js` e a tela do Consultor mostram o texto enquanto chega.
- [x] 1.4 No app, os pedidos à IA vão pelo `CapacitorWebFetch` (o `CapacitorHttp` entrega tudo no fim); e2e imita o `CapacitorHttp`.
## 2. Formato pela pergunta
- [x] 2.1 Instruções separam pergunta direta de pedido de conselho, com exemplo curto.
- [x] 2.2 `TestEval` mede o tamanho da resposta direta; `FINAN_EVAL_ONLY` escolhe as perguntas.
- [x] 2.3 "Quanto gastei com <lugar>" vai com o total pronto (`placeSpending`), com e sem IA.
## 3. Validação
- [x] 3.1 Testes Go e e2e do app em Chromium e WebKit.
- [x] 3.2 `TestEval` com IA real pelo grátis do finan_rich: 16 de 17; a falha (Uber) virou a 2.3 e passou 3 de 3.
- [ ] 3.3 Conferência num celular de verdade (o e2e só imita o `CapacitorHttp`).
