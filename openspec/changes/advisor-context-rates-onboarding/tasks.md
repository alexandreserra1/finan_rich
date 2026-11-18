## 1. Conversa
- [x] 1.1 Resumo das mensagens antigas em `<conversa_anterior>` (`earlier`), com teste.
- [x] 1.2 Sugestões pelos dados (`finance.Suggestions`), no `finance.wasm` e na tela.
## 2. Taxas do dia
- [x] 2.1 `internal/rates` (Selic, CDI, IPCA) com Banco Central falso nos testes.
- [x] 2.2 Taxas nos dados da IA e resposta pronta para "qual a selic hoje?".
- [x] 2.3 Cache no computador (6 h) e no app/site (por dia).
## 3. Erros
- [x] 3.1 `llm.Friendly` no motivo da resposta sem IA; "Tentar de novo" na tela.
## 4. Começo do zero
- [x] 4.1 Ferramenta `criar_conta_fixa` e cartões de conta fixa no app.
- [x] 4.2 Modo do começo do zero (só Otávio, até a 4ª resposta) e resposta sem IA.
## 5. Validação
- [x] 5.1 Testes Go e e2e do app em Chromium e WebKit.
- [ ] 5.2 Começo do zero com IA real (TestEval não cobre conversa de vários passos).
