# Arquitetura do finan_rich

Este documento explica como o projeto está organizado: o que cada pasta e arquivo faz,
como as peças se conectam e por que as decisões foram tomadas. Para instalar e usar,
veja o [README](../README.md); para os próximos passos, o [ROADMAP](ROADMAP.md).

## Visão geral

O finan_rich é **um único executável em Go** que roda no computador do usuário. Ele:

1. guarda os dados num arquivo SQLite (`finan.db`);
2. serve a interface web (HTML, CSS, JS e as regras compiladas para WebAssembly);
3. sincroniza os dados com cada aparelho (computador e celular) por `POST /api/sync`;
4. conversa com um modelo de IA (por padrão o Ollama, local) para o Consultor IA.

Cada aparelho guarda **uma cópia dos dados** no próprio navegador (IndexedDB) e monta as
telas sozinho. Por isso o app funciona sem conexão: as alterações entram numa fila e são
enviadas quando o computador volta a ser alcançável.

```
 APARELHO (navegador: computador ou celular)            COMPUTADOR (executável finanrich)
 ┌─────────────────────────────────────────────┐        ┌────────────────────────────────────┐
 │ static/js/pages/*   telas em JavaScript     │        │ internal/web       HTTP, telas      │
 │      │  chamam                              │        │      │                             │
 │ finance.wasm        regras em Go (Wasm)  ◀──┼─ mesmo ─┼▶ internal/finance  regras em Go    │
 │      │                                      │ código │   internal/domain  modelos         │
 │ static/js/db.js     IndexedDB + fila        │        │      │                             │
 │      │                                      │        │ internal/store     SQLite (records) │
 │ static/js/sync.js ──── POST /api/sync ──────┼───────▶│      │                             │
 │                    ◀── o que mudou ─────────┼────────│ internal/advisor + internal/llm    │
 │ sw.js               abre sem conexão        │        │      └──▶ Ollama / Groq / Claude   │
 └─────────────────────────────────────────────┘        └────────────────────────────────────┘
```

### Princípios

- **Regras de negócio escritas uma vez.** Os cálculos (resumo do mês, dívidas, reserva,
  validação) ficam em Go puro e rodam no servidor *e* no navegador, compilados para
  WebAssembly. Não existe uma cópia em JavaScript para manter em sincronia.
- **Dependências apontam para dentro.** `domain` não conhece ninguém; `finance` conhece só
  `domain`; `store`, `advisor` e `web` usam os de dentro. Nada de dentro conhece HTTP.
- **Local-first.** O aparelho é a fonte da verdade para a tela; o servidor é o ponto de
  encontro entre aparelhos e onde a IA roda.
- **Poucas dependências.** Stdlib do Go, o driver SQLite em Go puro, o SDK da Anthropic e um
  gerador de QR code. No navegador, nenhum framework e nenhuma etapa de build de JS.

### Mapa de dependências entre pacotes Go

```
cmd/finanrich ──▶ config, store, web
cmd/finanwasm ──▶ wasmapi ──▶ finance, domain, money

web ──▶ advisor, auth, llm, store, (domain)
advisor ──▶ finance, domain, money, llm, store(ChatMessage)
store ──▶ domain
finance ──▶ domain
domain, money, auth, llm ──▶ (só biblioteca padrão; llm usa o SDK da Anthropic)
```

Pacotes que rodam no navegador (precisam compilar para `js/wasm`, então **não podem**
importar o `store` nem nada com SQLite): `domain`, `finance`, `money`, `wasmapi`.
O `make test-wasm` garante isso rodando os testes deles como WebAssembly.

---

## Fluxos principais

### 1. Abrir uma tela de dados (Painel, Lançamentos, Dívidas, Categorias)

1. O navegador pede `/lancamentos`. O servidor (`web.appShell`) devolve só o **casco**:
   o layout com `<div id="app">` (`templates/app.html`).
2. `static/js/main.js` carrega os dados do IndexedDB (`db.loadLedger`) e o `finance.wasm`
   (`finance.load`), descobre a tela pelo caminho da URL e chama `pages/transactions.js`.
3. A tela pede os números prontos ao Go: `finance.dashboard(ledger, "2026-10")` →
   `wasmapi.Dashboard` → `finance.Dashboard` → JSON.
4. Sem conexão, o `sw.js` entrega o casco guardado e tudo acontece igual, porque os dados
   e as regras já estão no aparelho.

### 2. Salvar uma alteração (ex.: novo lançamento)

1. `pages/transactions.js` lê o formulário e chama `ctx.save("transaction", {...})`.
2. `main.js` cria o UUID (se for novo), põe `updatedAt = agora` e **valida com a mesma regra
   do servidor** (`finance.validate` → `domain.DecodeRecord`).
3. `db.saveLocal` grava o registro e o coloca na fila (`outbox`).
4. A tela é redesenhada na hora; `sync.soon()` agenda o envio para daqui a 0,4 s (junta
   várias alterações seguidas num envio só).

### 3. Sincronizar

```
aparelho                                      servidor
   │  POST /api/sync                             │
   │  { cursor: 41, changes: [fila...] }  ───────▶ store.ApplyChanges: para cada registro,
   │                                             │   grava se updatedAt for mais novo
   │                                             │   e dá a ele a próxima "rev" (42, 43…)
   │  ◀─────── { cursor: 45, changes: [...],     │ store.ChangesSince(41): tudo com rev > 41
   │             more: false }                   │   (inclui o que outros aparelhos mandaram)
   │ db.clearSent(fila enviada)                  │
   │ db.applyRemote(changes)  (mesma regra:      │
   │   vence o updatedAt maior)                  │
   │ salva cursor = 45                           │
```

- Roda ao abrir, ao voltar a conexão (`online`), ao voltar para a aba, a cada 30 s e logo
  depois de cada alteração. Se uma sincronização está rodando, a próxima é enfileirada.
- **Conflito:** vence o `updatedAt` maior, em empate o conteúdo de maior ordem alfabética,
  para todos os aparelhos chegarem ao mesmo resultado. A regra existe no servidor
  (`store.ApplyChanges`) e no aparelho (`db.applyRemote`).
- **Apagar** é marcar `deleted: true` (para o apagamento chegar aos outros aparelhos).
- **Categoria em uso** (`domain.Ledger.CanDeleteCategory`): não pode ser apagada enquanto
  lançamentos, recorrentes ou metas ativos a usam. O aparelho confere antes de apagar; o
  servidor confere ao sincronizar, em qualquer ordem de chegada: se a categoria apagada ainda
  está em uso, ela volta ativa (`updatedAt + 1`) e o aparelho que apagou recebe um aviso
  (`notices` na resposta do sync).
- Respostas: `401` → indicador "Entre de novo para sincronizar"; falha de rede → "Offline";
  `422` → registro inválido (mensagem do domínio).

### 4. Consultor IA

1. `static/chat.js` sincroniza primeiro (`window.finanSync()`), para o servidor ter os dados
   mais recentes, e envia a pergunta para `POST /chat/enviar`.
2. `web/chat.go` carrega o `Ledger` do banco, calcula os fatos (`advisor.BuildFacts`) e monta
   o retrato em texto (`advisor.Snapshot`): juros de cada dívida, sobra livre, reserva,
   pontos de atenção. **As contas vão prontas**, porque modelos pequenos erram aritmética.
3. `advisor.Ask` monta a conversa (system prompt + histórico recente + pergunta com o retrato)
   e chama o `llm.Provider` configurado, com **ferramentas** (`advisor/tools.go`):
   - `buscar_lancamentos`: filtra lançamentos por texto, categoria, tipo e período e devolve
     os totais (calculados em Go) e os 30 mais recentes, com o `[id]` de cada um;
   - `propor_lancamento`, `alterar_lancamento`, `apagar_lancamento`, `criar_meta`,
     `criar_divida`, `registrar_aporte`, `registrar_pagamento`: montam e validam o registro como vai ficar (`domain.DecodeRecord`), mas
     **não gravam**: viram uma `advisor.Proposal` (ação, tipo, registro, texto do cartão) que vai
     para a tela (evento SSE `proposal`) com um botão. O botão grava pelo cliente offline
     (`notes-save.js`, mesma validação em WebAssembly) e sincroniza.
   O laço pede ferramentas até 4 vezes por pergunta. Ferramentas usam o formato nativo de cada
   provedor (Ollama, OpenAI-compatível, Anthropic); se o modelo não aceitar ferramentas, o
   consultor pergunta de novo sem elas.
   Quando a IA configurada é de nuvem (`llm.IsLocal` falso), `advisor.Answer` anonimiza antes
   (`advisor/anon.go`): lojas, dívidas, credores e metas viram apelidos, as notas ficam de fora,
   e a resposta e as propostas voltam com os nomes verdadeiros. O app do celular sem computador
   usa o mesmo caminho em WebAssembly (`internal/askapi`, `cmd/finanadvisor`).
4. **"Treinamento" em finanças** (sem mexer nos pesos do modelo, o que os planos grátis não
   permitem): as instruções (`advisor.SystemPrompt`) pedem respostas personalizadas (números
   do usuário, conclusão primeiro, plano com R$ e prazo, próximo passo no app) e seguem a
   ordem de prioridades; a pergunta leva um bloco `<conhecimento>` com até 2 temas da base
   `advisor/knowledge.go` (dívidas, reserva, investimentos, orçamento, cartão, compras
   grandes, dinheiro extra, golpes; regras do Brasil, sem taxas do dia). A avaliação
   `advisor/eval_test.go` faz 10 perguntas típicas e confere critérios automáticos; roda só
   com chave: `FINAN_EVAL_PRESET=gemini FINAN_EVAL_KEY=... go test ./internal/advisor -run TestEval -v`.
   Em outubro de 2026, com `gemini-flash-lite-latest`: 58% dos critérios antes, 86–94% depois
   (varia um pouco entre rodadas).
4. A resposta volta em streaming (Server-Sent Events) e é gravada em `chat_messages`, com as
   propostas (coluna `proposals`, JSON). Ao mostrar a conversa, `advisor.Applied` compara cada
   proposta com os dados atuais para marcar o cartão como feito.

### 5. Acesso pelo celular e login

- Por padrão o servidor escuta só em `127.0.0.1` (este computador).
- Em Ajustes → *Acesso pelo celular*, o usuário cria uma senha; o servidor reabre a porta em
  todas as interfaces (`main.go` recebe o aviso por `Options.OnNetworkChange`).
- `web.requireAuth`: pedidos de `127.0.0.1` entram direto; os da rede precisam de sessão
  (cookie). Pedidos repassados por proxy (`X-Forwarded-For`, `Tailscale-User-Login`) contam
  como "da rede". Com `FINAN_PASSWORD` (modo servidor), todos precisam de login.
- O **app do celular** não usa cookie: no pareamento troca um código curto por um token
  (`Authorization: Bearer`). Com esse cabeçalho, vale só o token, mesmo vindo de
  `127.0.0.1`, e só para `/api/*` (sincronizar e consultor): com o token não se abre
  Ajustes nem se baixa o banco. Detalhes em [MOBILE.md](MOBILE.md).
- **Outros sites** abertos no mesmo navegador não usam o acesso local: sem token, pedido de
  outra origem (`Sec-Fetch-Site`/`Origin`) é recusado, menos a navegação (abrir um link).
  E, sem senha, só valem endereços por IP, `localhost`, `*.local`, `*.ts.net` ou os nomes de
  `FINAN_HOSTS`: um domínio de fora que aponte para `127.0.0.1` (DNS rebinding) recebe 403.
- **Senha e pareamento** têm limite de 5 tentativas erradas a cada 10 minutos por endereço
  (IPv6: por /64). A tentativa é contada antes de conferir a senha, então pedidos em paralelo
  não passam do limite. O servidor corta corpo enviado devagar (30 s) e conexões paradas
  (2 min); sem limite de escrita, por causa da resposta do chat aos poucos. Erro interno vai
  só para o log: quem pediu recebe uma mensagem genérica.
- **Wi-Fi sem cifra**: na rede de casa o tráfego é HTTP. A tela avisa para usar só em rede de
  confiança e, fora de casa, o Tailscale (HTTPS).
- **Dependências**: o CI roda `govulncheck` e `npm audit`; o Dependabot abre PR com versões
  novas. A versão do Go fica fixa no `toolchain` do `go.mod`.
- **Proxy reverso (nginx, Caddy) não é necessário** e pode ser perigoso: o gargalo medido é o
  SQLite (uma conexão, de propósito), não o HTTP; e um proxy que não mande `X-Forwarded-For`
  faz todo pedido parecer vir de `127.0.0.1`, liberando o acesso sem senha. Para acessar de
  fora, use Tailscale (HTTPS pronto). Se mesmo assim usar um proxy, rode com `FINAN_PASSWORD`
  (todo acesso pede login).
- **Limites medidos** (outubro de 2026, 4 CPUs): 500 aparelhos sincronizando juntos sem erro
  nem perda (p99 5 s; com 10 aparelhos, 61 ms); banco com 126 mil lançamentos baixa inteiro em
  1,7 s; até 10 MB por envio (o app manda a fila em lotes de 2.000; acima disso, 413). Os
  arquivos de `/static` saem com gzip e ETag (`finance.wasm`: 5,3 MB → 1,5 MB).
- Em Ajustes, trocar o endereço da IA sem colar a chave de novo apaga a chave salva (ela não
  vai para outro servidor), e passar de uma IA local para a nuvem começa conversa nova.

---

## Pastas e arquivos

### Raiz

| Arquivo | Função |
|---|---|
| `go.mod`, `go.sum` | Módulo Go e versões das dependências. |
| `Makefile` | Tarefas: `wasm`, `run`, `test`, `test-wasm`, `e2e`, `build`, `docker`, `mobile`, `apk`. |
| `Dockerfile` | Imagem do app: gera o `.wasm`, compila sem CGO, roda em `distroless`. |
| `docker-compose.yml` | Sobe o app (e opcionalmente o Ollama) em containers. |
| `.goreleaser.yaml` | Gera os executáveis de cada versão (Windows, macOS, Linux). |
| `.env.example` | Variáveis de ambiente opcionais (`FINAN_*`). |
| `.gitignore` | Ignora o `.wasm` gerado, `dist/`, bancos locais, `node_modules`. |
| `.dockerignore` | Deixa fora da imagem Docker o que não é código Go (`mobile/`, `node_modules`). |
| `mobile/` | App Android (Capacitor). Explicado em [MOBILE.md](MOBILE.md). |
| `LICENSE` | Licença MIT. |
| `README.md` | Como instalar, usar e desenvolver. |
| `docs/` | Esta documentação e o roadmap. |

### `cmd/` — programas

| Arquivo | Função |
|---|---|
| `cmd/finanrich/main.go` | Ponto de entrada do app: lê a config, abre o banco, cria o `web.Server`, escuta na porta (só local, ou na rede se o acesso pelo celular estiver ligado) e reabre a porta quando essa opção muda. Detecta se o app já está aberto e só abre o navegador. |
| `cmd/finanrich/browser.go` | Abre o navegador padrão em cada sistema; no Windows segura a janela aberta em caso de erro. |
| `cmd/finanwasm/main.go` | Compilado para WebAssembly: registra o objeto global `finan` no navegador, com as funções de `wasmapi`. |

### `internal/domain/` — o modelo de dados (Go puro)

| Arquivo | Função |
|---|---|
| `domain.go` | Tipos `Category`, `Transaction`, `Debt`, `Pref`, `Split`, `Note`, `Goal`, `Recurring`; campos de sincronização em `Meta` (`id`, `updatedAt`, `deleted`); constantes (buckets, tipos); **validação** de cada tipo. |
| `record.go` | `DecodeRecord`: lê um registro em JSON pelo tipo, valida e devolve a versão normalizada. Usado pelo servidor ao receber e pelo navegador antes de salvar. |
| `ledger.go` | `Ledger`: todos os dados juntos; filtros de ativos, índice de categorias, divisão do salário salva. |
| `month.go` | `Month`: navegação e rótulos em português ("outubro de 2026"). |
| `domain_test.go` | Validação, decodificação, ledger, mês. |

### `internal/finance/` — regras de negócio (Go puro)

| Arquivo | Função |
|---|---|
| `summary.go` | `Summarize` (resumo do mês e divisão planejado × realizado), `YearTotals`, `RecentTotals`. |
| `debts.go` | `Payoff` (simulação mês a mês), `PlanDebts` (totais, juros mensais, ordem avalanche e bola de neve), `PayoffSchedule` (todas juntas: mínimos + valor a mais, o que sobra de uma quitada vai para a próxima; saldo mês a mês) e `MinimumOnly` (cada uma só no mínimo, para comparar). |
| `emergency.go` | `PlanEmergencyFund`: reserva de 3 e 6 meses e prazo com a sobra livre. |
| `insights.go` | `Insights`: tela Análise — ritmo do mês, projeção, pode gastar por dia, gastos por lugar e por categoria contra a média, assinaturas e onde economizar ([ANALISE.md](ANALISE.md)). |
| `goals.go` | `Goals`: progresso das metas (já guardado + aportes na categoria), quanto guardar por mês, atrasada ou atingida. |
| `recurring.go` | `RecurringDue`: lançamentos que as recorrências já deveriam ter gerado (ID fixo `rec-<id>-<AAAA-MM>`, sem duplicar entre aparelhos). |
| `alerts.go` | `Alerts`: avisos do painel (vencimento em até 3 dias, dívida que cresce, grupo acima da meta, gasto fora do padrão, metas, orçamento da categoria aos 80% e estourado). |
| `accounts.go` | `Accounts`: saldo de cada conta e, nos cartões, a fatura atual e a próxima (pelo dia de fechamento); `NetWorth`: patrimônio mês a mês nos últimos 12 meses. |
| `installments.go` | `Installments`: divide uma compra em parcelas mensais sem perder centavo; o ID de cada parcela vem do da compra, então não duplica entre aparelhos. |
| `budgets.go` | `Budgets`: quanto já foi usado do teto mensal de cada categoria (`budgetCents`). |
| `search.go` | `SearchTransactions`: busca da tela Lançamentos (texto sem acento nem maiúscula, categoria, tipo, faixa de valor, mês) e os totais do que achou. |
| `merchants.go` | `TopMerchants`: gastos por estabelecimento nos últimos 12 meses (para o consultor). |
| `monthplan.go` | `PlanMonth`: parcelas mínimas pendentes, sobra livre, quanto pagar na dívida de maior juros. |
| `view.go` | `Dashboard` e `Debts`: o que cada tela mostra, já calculado, em structs com tags JSON (o que o navegador recebe). |
| `finance_test.go` | Testes de todas as regras com casos de valores conhecidos. |

### `internal/money/` — dinheiro

| Arquivo | Função |
|---|---|
| `money.go` | Valores em centavos (`int64`). `Parse` entende "1.234,56", "1234.56" e "3.500" (três mil e quinhentos); `Format` gera "R$ 1.234,56". |

### `internal/report/` — exportação e backup (Go puro)

Gera os arquivos que o usuário leva para fora do app. Sem dependências além da biblioteca
padrão, então também roda no aparelho (WebAssembly): exportar funciona sem conexão.

| Arquivo | Função |
|---|---|
| `report.go` | `Export(ledger, Options, agora)`: escolhe o formato (`xlsx`, `csv`, `pdf`, `json`, ou `txt`/`pdf` de uma nota), monta o nome do arquivo e o relatório em PDF. |
| `tables.go` | Os dados viram tabelas genéricas (`Table`, com colunas de tipo `Text`, `Money`, `Date`, `Percent`...): lançamentos, resumo por mês, por categoria, divisão do salário, dívidas, notas, categorias. Cada formato só sabe desenhar tabelas. |
| `period.go` | Período do relatório: mês, ano ou todo o histórico. |
| `xlsx.go` | Planilha do Excel escrita à mão (zip + XML): uma aba por tabela, valores e datas como número, formato R$, cabeçalho fixo e filtro. |
| `csv.go` | CSV no padrão do Excel em português: `;`, vírgula decimal, UTF-8 com BOM. |
| `pdf.go` | PDF escrito à mão: A4, Helvetica (WinAnsi, cobre os acentos), texto, linhas e retângulos; tabelas com cabeçalho repetido a cada página, barras e quebra de linha. |
| `backup.go` | `Backup` (todos os registros num `.json`) e `ReadBackup` (confere e valida antes de restaurar). |
| `report_test.go` | Abre os arquivos gerados: XML bem formado, tabela xref do PDF, CSV, backup de ida e volta. Roda também como WebAssembly (`make test-wasm`). |

### `internal/importer/` — importação de extratos (Go puro)

Lê o extrato no aparelho (WebAssembly): funciona offline e o arquivo não sai do aparelho.

| Arquivo | Função |
|---|---|
| `parse.go` | `Parse`: descobre o formato pelo conteúdo; datas (`05/10/2026`, `2026-10-05`, `20261005`...) e valores (`-1.234,56`, `1234.56`, `(87,50)`, `87,50 D`...). |
| `ofx.go` | OFX 1 (SGML, tags sem fechamento) e 2 (XML), conta e cartão; junta NAME e MEMO sem repetir. |
| `pdftext.go` | `ParseText`: compras de uma fatura em PDF a partir do texto que o pdf.js extrai no aparelho (data com ou sem ano, mês abreviado, duas colunas, ano pelo vencimento, créditos com sinal ou "C"). |
| `csv.go` | Descobre o separador e acha o cabeçalho pelo nome das colunas (data, descrição/histórico, valor ou crédito e débito, tipo, categoria), mesmo depois de linhas de título. Fatura de cartão (quase tudo positivo) vem marcada para inverter o sinal. |
| `text.go` | Converte arquivos em Windows-1252 para UTF-8; normaliza textos para comparar. |
| `preview.go` | `Build`: monta a revisão. ID estável (`imp-` + hash do FITID ou de data/valor/descrição), então reimportar não duplica; marca o que parece digitado à mão (mesmo dia, tipo e valor) e o pagamento de fatura. |
| `categories.go` | Categoria sugerida: do arquivo, do histórico (descrições parecidas que a pessoa já classificou; aprende com as correções) ou de palavras-chave de lojas e serviços. |
| `importer_test.go`, `testdata/` | Arquivos no formato de Itaú, Nubank (conta e cartão, OFX e CSV), Inter, BB e do próprio finan_rich. |

**Open Finance** (`internal/pluggy`): `Fetch` pega a chave na Pluggy (`POST /auth`) com o
Client ID e o Secret da própria pessoa, lista as contas de cada Item ID (`GET /accounts`) e os
lançamentos já caídos (`GET /v2/transactions`, por cursor, até 50 páginas e 8 MB por resposta).
O sentido vem do campo `type` (DEBIT/CREDIT), não do sinal. Cada conta vira um
`importer.Statement` (`Account` = `pluggy:<id>`, FITID = id do lançamento) e segue a revisão
de sempre (`wasmapi.ImportPreviewStatement`): buscar de novo não duplica. No computador a busca
é `POST /api/openfinance` (`internal/web/openfinance.go`, credenciais na tabela `settings`, o
segredo nunca volta para a tela); no app com o Open Finance configurado, roda no
`advisor.wasm` (`askapi.OpenFinance`) e o Client Secret fica no cofre do sistema
(`js/openfinance.js`). Nada passa por servidor do finan_rich.

A IA (opcional) fica no servidor: `internal/advisor/categorize.go` pergunta em lotes de 40 e
só aceita respostas que sejam uma das categorias do tipo certo; `internal/web/categorize.go`
expõe `POST /api/categorizar`, que recebe só descrições e tipos (sem valores).

### `internal/wasmapi/` — ponte com o navegador

| Arquivo | Função |
|---|---|
| `wasmapi.go` | Funções texto → texto (JSON) chamadas pelo JavaScript: `Dashboard`, `Debts`, `Validate`, `ParseMoney`, `FormatMoney`, `MonthLabel`, `Export` (arquivo em base64), `ReadBackup` e `ImportPreview`. Respostas no formato `{"ok": ...}` ou `{"error": "..."}`. Separado do `cmd/finanwasm` para ser testável sem navegador. |

### `internal/store/` — banco de dados (SQLite)

| Arquivo | Função |
|---|---|
| `store.go` | Abre o banco (WAL, chaves estrangeiras, uma conexão), aplica as migrations em ordem, utilitários `inTx` e `collect`. |
| `records.go` | Registros sincronizáveis: `ApplyChanges` (grava com "vence o mais recente" e numera com `rev`), `ChangesSince` (paginado), `Ledger` (carrega tudo para o servidor). |
| `settings.go` | Chave-valor de configurações do servidor (IA, acesso). **Não** sincroniza com os aparelhos (tem chave de API e hash de senha). |
| `chat.go`, `chatmodel.go` | Histórico do Consultor IA (fica só no servidor). |
| `sessions.go` | Sessões de login (só o hash SHA-256 do token). |
| `devices.go` | Aparelhos pareados (app do celular): criar, achar pelo hash do token, listar, remover. |
| `migrations/001_init.sql` | Schema da v1.0 e dados iniciais (categorias, 50/30/20). |
| `migrations/002_sessions.sql` | Tabela de sessões. |
| `migrations/003_sync_records.sql` | v1.1: cria `records` e `sync_state`, **converte** os dados da v1.0 e remove as tabelas antigas. |
| `migrations/004_devices.sql` | Tabela `devices` (app do celular). |
| `migrations/005_notes.sql` | v1.3: recria `records` aceitando o tipo `note` (o SQLite não altera um `CHECK`). |
| `migrations/006_open_types.sql` | v1.5: `records` sem lista fixa de tipos (quem valida é o `domain.DecodeRecord`); vieram `goal` e `recurring`. |
| `store_test.go` | Sincronização, validação, paginação e a conversão de um banco v1.0 real. |

Tabelas atuais:

| Tabela | Conteúdo |
|---|---|
| `records(type, id, data, updated_at, deleted, rev)` | Categorias, lançamentos, dívidas, preferências, notas, metas, recorrentes e contas como documentos JSON. |
| `sync_state(rev)` | Contador global de revisões. |
| `settings(key, value)` | `ai_*` (provedor de IA) e `access_*` (senha, acesso pela rede). |
| `chat_messages` | Conversa com o consultor. |
| `sessions` | Logins ativos. |
| `devices` | Apps do celular pareados (nome, hash do token, última sincronização). |
| `schema_migrations` | Quais migrations já rodaram. |

### `internal/llm/` — provedores de IA

| Arquivo | Função |
|---|---|
| `llm.go` | Interface `Provider` (`Chat` com streaming), `Config`, `New` (escolhe a implementação), cliente HTTP compartilhado. |
| `ollama.go` | Ollama pela API nativa: contexto de 8192 tokens, `think: false`, temperatura 0,7; lista e baixa modelos; distingue "não instalado", "lento" e "precisa de login" (modelos `:cloud`). |
| `openai.go` | Qualquer API no formato `/v1/chat/completions` (Groq, OpenRouter, DeepSeek, LM Studio). |
| `anthropic.go` | Claude pelo SDK oficial, com fallback automático em recusas. |
| `stream.go` | Leitura de NDJSON/SSE e o filtro que remove `<think>…</think>` mesmo partido entre pedaços. |
| `presets.go` | Opções prontas da tela de Ajustes (Ollama, Ollama Cloud, Groq, OpenRouter, DeepSeek…). |
| `*_test.go` | Cada provedor contra um servidor falso que imita a API real. |

### `internal/advisor/` — o consultor

| Arquivo | Função |
|---|---|
| `advisor.go` | System prompt, `BuildRequest` (histórico das últimas 12 mensagens + pergunta com o retrato) e `Ask`. |
| `snapshot.go` | `BuildFacts` (calcula tudo com o `finance`) e `Snapshot` (texto com as contas prontas e os pontos de atenção). |

### `internal/auth/` — senha e sessões

| Arquivo | Função |
|---|---|
| `password.go` | Hash PBKDF2-SHA256 (600 mil iterações) e verificação em tempo constante. |
| `session.go` | Tokens aleatórios de 256 bits e o hash guardado no banco. |
| `limiter.go` | Bloqueia um endereço após 5 senhas erradas em 10 minutos. |

### `internal/config/` — configuração

| Arquivo | Função |
|---|---|
| `config.go` | Flags (`-addr`, `-data`, `-no-browser`) e variáveis `FINAN_*`; pasta de dados padrão de cada sistema. |

### `internal/web/` — servidor HTTP e o app do aparelho

Código Go:

| Arquivo | Função |
|---|---|
| `server.go` | `New(store, Options)`, rotas, renderização de templates, middlewares (segurança, CSRF, log), service worker com a impressão digital dos arquivos. |
| `api.go` | `POST /api/sync`. |
| `access.go` | Login, sessões, `requireAuth` (cookie ou token do app), ligar/desligar o acesso pelo celular. |
| `backup.go` | `GET /configuracoes/backup.db`: cópia completa do banco (`VACUUM INTO`), com configurações e chat. |
| `pairing.go` | Pareamento do app do celular: código, página `/parear`, `POST /api/parear`, token do aparelho, CORS de `/api/`. |
| `export.go` | `ExportApp`: grava as telas do app (casco + `static/`) para o Capacitor. |
| `lan.go` | Endereços da rede local para o celular e QR code. |
| `tailscale.go` | Detecta o Tailscale e o endereço `https://` deste computador. |
| `aiconfig.go` | Lê e grava a configuração da IA; verifica se o Ollama e o modelo estão prontos. |
| `settings.go` | Tela de Ajustes, testar conexão, baixar modelo. |
| `chat.go` | Tela do consultor e envio de perguntas (SSE); `GET/POST /api/chat` e `/api/chat/limpar` para o app do celular (token). |
| `sse.go` | Envio de Server-Sent Events. |
| `redirect.go` | Redirecionamento com mensagem (padrão POST/redirect/GET). |
| `generate.go` | `//go:generate` que cria `static/finance.wasm`. |
| `*_test.go` | Integração: páginas, sincronização, login, acesso pela rede, chat com IA falsa. |

Templates (`templates/`, renderizados no servidor; arquivos com `_` são parciais):

| Arquivo | Função |
|---|---|
| `_layout.html` | Estrutura comum: metas do PWA, topo com indicador de sincronização, menu (abas no rodapé no celular), ícones. |
| `_aistatus.html` | Avisos de Ollama ausente ou modelo não baixado, com o botão de baixar. |
| `app.html` | Casco das telas de dados (`<div id="app">`), preenchido pelo JavaScript. |
| `chat.html`, `settings.html`, `login.html` | Telas que dependem do servidor. |
| `pair.html` | Página aberta pelo QR code de pareamento: leva ao app ou explica como instalá-lo. |

Arquivos do navegador (`static/`):

| Arquivo | Função |
|---|---|
| `js/main.js` | Entrada do app: carrega dados e regras, monta a tela, salva alterações, navega sem recarregar, mostra o status da sincronização, apaga os dados do aparelho ao sair. |
| `js/db.js` | IndexedDB: `records` (cópia dos dados), `outbox` (fila), `meta` (cursor). |
| `js/sync.js` | Ciclo de sincronização e seus gatilhos. |
| `js/device.js` | Para onde sincronizar: o próprio servidor (navegador) ou o computador pareado (app do celular, com token). |
| `js/finance.js` | Carrega o `finance.wasm` e expõe as funções em JavaScript. |
| `js/ui.js` | `html` com escape automático (evita injeção), UUID, datas, mensagens. |
| `js/pages/dashboard.js` | Painel do mês: totais, divisão do salário, gastos por categoria, visão anual. Também exporta o seletor de mês e os cartões de totais usados pelos lançamentos. |
| `js/pages/transactions.js` | Lançamentos: formulário (categorias filtradas por entrada/saída), lista do mês, busca e filtros (no endereço, então voltar e avançar funcionam), apagar, "copiar do mês anterior". |
| `js/pages/accounts.js` | Contas e cartões: cadastro, saldos, faturas e o painel de patrimônio (também usado na Análise). |
| `js/pages/welcome.js` | Primeiro uso guiado, mostrado no Painel até a pessoa terminar ou pular. |
| `js/pages/debts.js` | Dívidas: o plano numa frase (o que pagar primeiro, em quantos meses, juros economizados), valor a mais por mês (`?extra=`), gráfico plano × só o mínimo, faturas abertas dos cartões, cadastro e edição. |
| `js/pages/categories.js` | Divisão do salário (percentuais) e categorias (criar, apagar se nada a usa: regra em Go). |
| `js/pages/notes.js` | Notas: criar, editar, fixar, buscar, apagar e exportar cada uma em PDF ou texto. |
| `js/pages/export.js` | Exportar (Excel, PDF, CSV por período), backup e restauração. |
| `js/pages/import.js` | Importar extrato: enviar o arquivo, revisar (marcar, editar descrição, escolher categoria), pedir à IA e importar. |
| `js/pdftext.js` | Extrai o texto de PDFs com o pdf.js (carregado só quando preciso), linha por linha, com suporte a senha. |
| `vendor/pdfjs/` | pdf.js 6.3 (Mozilla, Apache-2.0), build legacy, embutido para funcionar offline. |
| `js/pages/goals.js` | Metas: criar, editar, apagar; cartões de progresso (também usados no painel). |
| `js/notes-save.js` | Salva uma resposta do consultor como nota (usado pelo chat do navegador e do app). |
| `js/native.js` | Só no app do celular: leitor de QR e lembretes de vencimento. |
| `js/exporter.js` | Entrega o arquivo gerado: download no navegador; no app do celular, salva e abre o menu de compartilhar (plugins Filesystem e Share do Capacitor). |
| `js/pages/device.js`, `js/pages/advisor.js` | Só no app do celular: Ajustes (pareamento, QR, lembretes) e Consultor (conversa pela `/api/chat`). |

Cada tela exporta `render(ctx)` (devolve o HTML) e, se tiver formulário, `bind(root, ctx)`
(liga os eventos). O `ctx` traz os dados (`ledger`), o mês, os parâmetros da URL e as ações
`save`, `remove`, `refresh`, `go` e `error`, criadas pelo `main.js`.

| Arquivo | Função |
|---|---|
| `js/wasm_exec.js` | Gerado: o "runtime" do Go para WebAssembly (copiado do Go instalado). |
| `finance.wasm` | Gerado: as regras em Go compiladas. |
| `app.js` | Utilitários das telas do servidor: SSE, markdown do chat, botão de baixar modelo, botão "Instalar app", registro do service worker. |
| `chat.js`, `settings.js` | Comportamento das telas de chat e de Ajustes. |
| `sw.js` | Service worker: guarda casco, scripts e `.wasm`; abre offline. |
| `manifest.webmanifest`, `icons/`, `favicon.svg` | Instalação como app (PWA). |
| `offline.html` | Mostrada quando uma tela que precisa do servidor é aberta sem conexão. |
| `app.css` | Estilos, tema claro/escuro (tokens em `:root` e `:root[data-theme="dark"]`), layout do celular. |
| `js/theme.js` | Tema automático, claro ou escuro (escolha em Ajustes, guardada só no aparelho); roda no `<head>` para a tela não piscar. |

### `tools/`, `e2e/`, `scripts/`, `.github/`

| Arquivo | Função |
|---|---|
| `tools/wasmbuild/main.go` | Compila `cmd/finanwasm` para `static/finance.wasm` e copia o `wasm_exec.js` (funciona em qualquer sistema). |
| `tools/mobilebuild/main.go` | Grava `mobile/www` (as telas do app do celular) com `web.ExportApp`. |
| `e2e/package.json` | Dependência dos testes no navegador (Playwright) e o comando `npm test`. |
| `e2e/offline.test.mjs` | Teste no Chromium: online, desligar o servidor, reabrir offline, lançar, religar, sincronizar, segundo aparelho. |
| `e2e/app.test.mjs` | Teste do app do celular no Chromium: pareamento, sync com token, offline, aparelho removido. |
| `e2e/export.test.mjs` | Notas, exportação (xlsx, PDF, CSV, inclusive offline), backup e restauração no Chromium. |
| `e2e/import.test.mjs` | Importação de fatura CSV, extrato OFX e fatura em PDF, sugestões, reimportação sem duplicar e aprendizado pelo histórico. |
| `e2e/plans.test.mjs` | Recorrentes (gera os meses, não duplica, apagado não volta), aviso de vencimento e metas no painel. |
| `scripts/install.sh` | Instalador de uma linha para macOS e Linux. |
| `.github/workflows/ci.yml` | A cada PR/push na main: gera o `.wasm`, formatação, vet, testes com race, regras como WebAssembly, compilação para todos os sistemas, os testes no navegador, o APK de teste do Android aberto num emulador e o app iOS compilado para o simulador. |
| `.github/workflows/release.yml` | Publica uma versão (por tag ou pelo botão *Run workflow*) e anexa o APK do Android. |

---

## Formato dos dados sincronizados

```json
{ "type": "transaction",
  "record": { "id": "6f1c…", "updatedAt": 1791760000000, "kind": "saida",
              "description": "Mercado", "amountCents": 9000,
              "categoryId": "cat-5", "date": "2026-10-12" } }
```

- Valores em **centavos** (`int64`); juros em **pontos-base** (1,99% = 199).
- Datas `AAAA-MM-DD`; `updatedAt` em milissegundos desde 1970 (UTC).
- IDs da v1.0 viraram `cat-3`, `tx-10`, `debt-2`; os novos são UUID.

## Testes

| Comando | O que cobre |
|---|---|
| `make test` | `gofmt`, `go vet`, todos os testes Go e as regras rodando como WebAssembly no Node. |
| `go test -race ./...` | Os mesmos testes com o detector de race conditions (o CI roda assim). |
| `make e2e` | O ciclo offline completo num navegador de verdade, e o app do celular (pareamento e sync com token). |

Nada usa a rede: o banco é SQLite em memória e os provedores de IA são servidores falsos que
imitam o Ollama, a OpenAI e a Anthropic.

## Como estender

- **Nova regra de cálculo:** escreva em `internal/finance` com teste; se uma tela precisar,
  inclua na view (`view.go`) e, se for nova, exponha em `wasmapi` + `cmd/finanwasm`.
- **Novo tipo de registro** (ex.: metas): tipo com `Meta` e `Validate` em `domain`; caso em
  `domain.DecodeRecord`; o tipo no `CHECK` de `records` (nova migration); lista no `Ledger` e
  em `db.loadLedger`; tela em `static/js/pages/`.
- **Nova tela de dados:** módulo em `static/js/pages/`, rota em `main.js` (`ROUTES`) e em
  `server.go` (`appShell`), e o arquivo na lista `PRECACHE` do `sw.js`.
- **Novo provedor de IA:** implemente `llm.Provider`, registre em `llm.New` e adicione um preset.
- **Mudar o banco:** nova migration `00N_*.sql`; nunca edite uma já publicada.
