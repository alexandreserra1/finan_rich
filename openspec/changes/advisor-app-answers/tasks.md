## 1. Roteador
- [x] 1.1 Tabela de perguntas (app e IA) em `app_answers_test.go`, falhando antes do código.
- [x] 1.2 `app_answers.go`: filtros da dúvida, assuntos e respostas com as contas existentes.
- [x] 1.3 `Prepared`, `Offline` e `Numbers` passam pelo roteador; `AskAI` passa direto.
- [x] 1.4 "2 mil", "1,5 mil" e "2k" nas contas prontas (`questionMoney`).
- [x] 1.5 Respostas do app nos critérios da avaliação, sem IA (5 das 14 perguntas); `TestEval`
  com `AskAI`. Na amostra de 34 perguntas típicas, 18 saem do app.
## 2. Perguntar à IA
- [x] 2.1 `askAI` no pedido do computador (`/api/chat`) e do app (`askapi`).
- [x] 2.2 Histórico do computador marca a resposta do app pelos números gravados.
- [x] 2.3 Botão "Perguntar à IA"; "Tentar de novo com a IA" sempre para a IA; sem "Isso é com"
  depois da resposta do app; a troca para outro do time mantém o `askAI`.
- [x] 2.4 e2e no app (nuvem com apelidos) e no celular pareado (IA do computador).
## 3. Fechamento
- [x] 3.1 Telas conferidas no navegador, com o programa e dados fictícios: computador e
  celular, claro e escuro, sem rolagem lateral. O único erro no console é o 400 do
  `/api/openfinance` sem a Pluggy configurada, que já acontecia em todas as telas.
- [x] 3.2 Docs: `IA-GRATIS.md`, `ARCHITECTURE.md` e `passo-a-passo.md`.
