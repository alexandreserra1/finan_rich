# Problemas conhecidos

Consulte antes de corrigir a mesma área. Atualize o registro existente em vez de duplicá-lo.
Cada incidente deve conter esperado, observado, status, hipótese ou causa confirmada,
arquivos, correção, testes, regressões e prevenção. Nunca inclua credenciais.

## ENG-001 — Credenciais reais e consentimento bancário

- Esperado: importar transações mediante autorização, sem expor credenciais.
- Observado: a integração Pluggy pessoal existe; não há gratuidade comercial multicliente
  confirmada nem provedor substituto contratado.
- Status: pendência externa; não é defeito corrigido.
- Causa: requisitos comerciais e regulatórios não equivalem a uma API pública gratuita.
- Fontes e próximos passos: [OPEN-FINANCE.md](../OPEN-FINANCE.md).
- Teste possível: fixtures e importação OFX/CSV; não provar contas reais sem consentimento.

## ENG-002 — Login/Drive e cadastro sem Google

- Esperado: entrar e sincronizar sem perder acesso nem dados.
- Observado histórico: erro 403 do Drive e configuração inválida de Client Secret.
- Status: há diagnóstico e recuperação no código; causa do 403 pessoal não confirmada.
- Não confundir: corrigir secret informado pela pessoa não comprova escopo, consentimento
  ou API do Drive habilitada. Testes locais usam Google simulado.
- Cadastro Supabase: adaptador e esquema existem, mas projeto, credenciais e SMTP ainda
  precisam ser configurados. Não anunciar cadastro em produção com base no adaptador.
- Próximo passo: obter somente código de erro sanitizado e verificar configuração com a
  pessoa; nunca pedir que envie senha ou secret ao chat.

## ENG-003 — Aprovação desatualizada no novo gate

- Esperado: uma nova falha invalida qualquer aprovação anterior, mesmo sem editar o código.
- Observado na revisão inicial: a evidência era gravada somente no sucesso.
- Causa confirmada: faltava estado de execução e invalidação antes dos testes.
- Correção: `scripts/engineering.mjs` grava running, passed ou failed; o Stop exige passed
  e o hash atual, incluindo o índice e o conteúdo de trabalho.
- Teste: `aprovação real é invalidada por nova falha com o mesmo hash` simula uma dependência
  externa indisponível depois da aprovação, reprova e confirma que Stop pede correção.
- Regressões examinadas: índice parcialmente preparado, remoções e compactação de sessão.
- Status: corrigido e coberto por teste local; CI nova ainda não foi publicada.

## ENG-004 — Risco de processos órfãos ao exceder prazo

- Esperado: uma verificação com limite não deixa seu servidor e subprocessos rodando.
- Causa identificada na revisão: timeout somente no filho direto não cobre seus descendentes.
- Correção: gates longos usam grupo de processos próprio e encerram o grupo ao exceder prazo,
  saída ou receber interrupção. A prova MCP tem encerramento com prazo e escalonamento.
- Teste: filho e neto ignoram SIGTERM em fixture; timeout encerra ambos e confirma o PID do neto.
- Limite: o controle completo de grupos foi validado no macOS e é usado no Linux da CI;
  a mesma garantia para árvores de processos no Windows não foi validada.
- Status: corrigido localmente; não altera nem encerra processos iniciados pela pessoa.

## ENG-005 — Emojis ❌ e ✅ nas mensagens viram figura colorida no iPhone

- Esperado: mensagem de erro em texto, com a cor vinda do CSS, igual em qualquer aparelho.
- Observado: prints do iPhone mostraram a marca e as setas como emojis (já corrigidos pelo
  Codex no PR #24, com SVG). Uma busca depois achou ❌ e ✅ em `import.js`, `settings.js` e
  `internal/web/settings.go`.
- Causa confirmada: o Safari troca esses caracteres por emoji colorido; a marca de erro
  também era lida com `startsWith("❌")`, ligando o estilo ao emoji.
- Correção: texto puro; `state.aiError` decide a classe `neg`; teste e2e usa `p.neg`.
- Testes: `make test`, `e2e/import.test.mjs` e `node scripts/engineering.mjs check affected`.
- Falhou no caminho: o seletor `has-text('Sem conexão')` expirou, porque a mensagem depende
  do erro (TypeError ou texto do servidor); o teste agora confere só a classe.
- Risco restante: não conferido no iPhone físico. Símbolos →, ← e ↑ ficam como texto.

## ENG-006 — Item atual do menu lateral some no tema escuro (celular)

- Esperado: no menu aberto, a página atual aparece em amarelo com texto escuro.
- Observado (print do iPhone): faixa amarela vazia, sem texto nem ícone.
- Causa confirmada: `:root[data-theme="dark"] .tabs a[aria-current]` (cor amarela, para a barra
  de baixo) tem mais peso que `body.menu-open .tabs a[aria-current]`; texto e fundo ficaram
  `rgb(245, 239, 171)`. Medido pelo teste antes da correção.
- Correção: regra do menu aberto repetida com o prefixo do tema escuro (`app.css`).
- Teste: `e2e/app.test.mjs` compara cor e fundo do item atual nos temas claro e escuro.
- Armadilha: o e2e do app lê `mobile/www`, que é gerado; sem `go run ./tools/mobilebuild
  mobile/www` o teste usa o CSS antigo e engana.
- Desktop: a barra lateral usa a regra sem a sobreposição do escuro (só no celular); não
  conferido no navegador.

## ENG-007 — Auditoria das ferramentas barrada por aviso sem correção (picomatch)

- Observado: o job `engineering` do CI falhou no PR #25 com `npm audit --audit-level=high`:
  `picomatch` (GHSA-fc5q-39xc-vjpf, ReDoS), puxado por `@fission-ai/openspec` 2.0.0.
- Causa confirmada: o aviso vale para `picomatch <= 4.0.7`, e a 4.0.7 é a versão mais nova em
  17/11/2026. `npm audit fix` não resolve; `--force` só rebaixaria o OpenSpec.
- Decisão (autorizada pelo Alexandre em 17/11/2026): liberar só esse aviso até 17/12/2026, em
  `scripts/audit-tools.mjs`. Qualquer outro aviso alto ou crítico continua barrando, e a
  liberação vencida volta a barrar. O app (`mobile/`) segue com `npm audit` puro.
- Risco aceito: o picomatch roda só no CI e na máquina de quem desenvolve, com padrões do
  próprio repositório; não vai para o app nem para o site.
- Teste: `scripts/audit-tools.test.mjs` (dentro do prazo, vencido, aviso novo, aviso moderado).
- Ao vencer: conferir se saiu versão corrigida do picomatch ou do OpenSpec e tirar a liberação.

## Modelo para novos incidentes

Use identificador estável, data e status. Separe hipótese de causa confirmada. Informe
comando reproduzível, evidência, correção, testes executados, possíveis regressões e risco
restante. Referencie a mudança OpenSpec e arquivos relevantes, sem copiar a arquitetura.

## ENG-008 — WebKit do CI preso no apt e iOS sem máquina macOS

- Esperado: os jobs `e2e (webkit)` e `ios` terminam em minutos.
- Observado em 16/11 e 18/11/2026: o WebKit caía com `exit code 124` ao baixar 125 MB de
  bibliotecas pelo apt (espelhos do Ubuntu a poucos KB/s nas duas tentativas de 5 min). O
  iOS era cancelado com 15 min de fila, sem `runner_name` e sem nenhum passo.
- Causa confirmada do WebKit: depender do apt. Um cache dos `.deb` não resolvia: a chave
  saía sem hash (o `package-lock` do e2e não está no git) e o `actions/cache` só grava quando
  o job passa.
- Correção do WebKit: o job roda na imagem `mcr.microsoft.com/playwright:v1.64.1-noble`, na
  mesma versão do e2e (`.github/workflows/ci.yml`). No contêiner, o git precisa de
  `safe.directory`, senão o `go generate` cai em "error obtaining VCS status". Ao subir o
  Playwright do e2e, suba também a tag da imagem.
- iOS: fila do GitHub para `macos-latest` (`macos-26-arm64`); nas rodadas boas a espera foi de
  5 a 10 min. Não é defeito do código: confira `runner_name` vazio e rode de novo só o job.
  Se virar rotina, fixar uma imagem com mais máquinas (ex.: `macos-15`) exige conferir o
  Xcode.
- Também visto em 18/11: um push no PR não gerou o evento `pull_request` (mergeabilidade
  `UNKNOWN`); `gh workflow run ci.yml --ref <branch>` roda o mesmo CI no mesmo commit.
- Visto em 22/11 (main, `2cc6f54`): o `testAbreConfiguracoesSemConectarConta` falhou porque
  o toque em "Conectar agora" não navegou. No `.xcresult` (artefato `rich-ios-validacao`), a
  árvore de acessibilidade e o vídeo mostram a primeira tela parada por 30 s, sem alerta nem
  redesenho. O mesmo código tinha passado nos três commits do PR, e rodar de novo só o job
  passou. Para ler o `.xcresult` sem o Xcode, os blocos de `Data/` são zstd: `zstd -d` e
  `file`. Se repetir, o teste deve tocar de novo quando a tela seguinte não aparecer.

- Em 27/11/2026, o Dependabot subiu o `playwright` do e2e para 1.65.0 (#37) sem a imagem, e o
  job do WebKit parou com `Executable doesn't exist ... required: v1.65.0-noble`. A imagem foi para
  `v1.65.0-noble`, e `scripts/engineering.test.mjs` passou a conferir que ela acompanha a versão do
  `e2e/package.json`: o próximo PR do Dependabot falha no job `engineering` até a imagem subir junto.

## ENG-009 — No celular, link com âncora caía no topo e o teclado cobria o formulário

Achados em 22/11/2026, com a v1.11.0 oficial instalada pelo site num emulador Android 14.
- Âncora. Esperado: "usar só neste celular" (`/configuracoes#sozinho`) abre os Ajustes no
  botão. Observado: abria no topo, com o botão sete telas abaixo. Causa confirmada: o
  `navigate()` do `main.js` sempre rolava para o topo depois de desenhar a tela e ignorava o
  `#`. Correção: rolar até o elemento do `#`; o `scroll-padding-top` do `html` faz ele parar
  abaixo do cabeçalho fixo.
- Teclado. Observado: a barra de baixo (fixa) subia com o teclado e cobria o botão "Ligar o
  bloqueio". Causa confirmada: o WebView do app encolhe a página quando o teclado abre.
  Correção: o `main.js` marca `body.typing` quando há campo de texto em foco e a tela está 20%
  mais baixa que a maior já vista; o `app.css` esconde a barra. Só o foco não basta: o
  "voltar" do Android fecha o teclado e o campo continua em foco.
- Teste: o `e2e/app.test.mjs` confere o botão à vista depois do link. Também confere a barra
  sumindo com a tela encolhida e voltando quando ela volta ao tamanho. Os dois falharam
  antes da correção.
- Armadilha: o Chrome 113 do emulador responde `pointer: fine` mesmo só com tela de toque
  (`any-pointer: coarse` falso, medido pelo DevTools remoto). Regra só para celular vai pela
  largura, não por `pointer`.
- Não conferido: celular de verdade (o emulador sem Play Store fica no WebView 113).

## ENG-010 — No celular, as taxas do Banco Central nunca chegavam ao Consultor

Achado em 23/11/2026, num teste de uso com um APK de teste da main no emulador Android 14.
- Esperado: "o que é CDB?" traz o CDI do dia, e a IA do app recebe Selic, CDI e IPCA. Observado:
  no celular, sempre "Confira a taxa do dia"; no computador, a taxa aparecia.
- Causa confirmada pelo DevTools remoto do WebView: o `fetch` nativo do Capacitor
  (CapacitorHttp) entrega o cabeçalho `Content-Length` repetido (`"0, 39"`). O `net/http` do Go
  no `advisor.wasm` recusa a resposta (`ill-formed Content-Length header`). Pelo `fetch` do
  WebView (`CapacitorWebFetch`) o cabeçalho vem certo (`"39"`), e o Banco Central aceita CORS.
  Os pedidos à IA já iam pelo WebView (para a resposta chegar aos poucos), por isso funcionavam.
- Correção: o `cloud-ai.js` manda também os pedidos do Banco Central pelo `fetch` do WebView
  (`streamFromWebView`), com a volta para o nativo se o WebView falhar.
- Teste: o `e2e/app.test.mjs` imita o `fetch` nativo com o cabeçalho repetido, apaga as taxas
  guardadas e confere a taxa na explicação do CDI. Falhou antes da correção. No emulador, com o
  APK corrigido, a taxa aparece e a IA a usa no "Perguntar à IA".
- Conferido também: o "Testar chave" dos Ajustes e o "IA sugere" da importação funcionam pelo
  nativo (as respostas da IA não vêm com o cabeçalho repetido). Em 24/11/2026, no emulador, o Go do
  `advisor.wasm` leu certo pelo nativo uma resposta de erro do Pluggy (Open Finance: "a Pluggy
  respondeu 400", com credenciais de teste) e a de chave errada da IA (401). O `Content-Length`
  repetido apareceu só com o Banco Central, que manda um cabeçalho só pelo `curl`; não é de todo
  pedido nativo, então não há remendo geral no `fetch`.
- 24/11/2026, depois da v1.12.0: no APK oficial, as taxas voltaram a faltar ("Confira a taxa do
  dia"), até numa instalação do zero. Pelo DevTools, o Go recebia `304 Not Modified`, sem corpo.
  O Banco Central manda `Cache-Control: public, max-age=900` e `private` repetidos e um `ETag`.
- 1ª hipótese (#50, v1.12.1): o WebView 113 revalidava a resposta guardada e entregava o 304 ao
  JavaScript. Os pedidos pelo `fetch` do WebView passaram a sair com `cache: "no-store"`. Não
  bastou: a v1.12.1 continuou recebendo 304.
- Causa confirmada: o **próprio servidor**, de forma intermitente, responde 304 sem corpo a pedido
  sem condição. Pelo `curl` no Mac, sem cabeçalho nenhum, a série do CDI (4389) voltou 304 quatro
  vezes seguidas; um minuto depois, 45 de 45 pedidos das três séries voltaram 200. No app, com
  `no-store`, o pedido saía sem `If-None-Match` e o 304 vinha da rede, não do cache. Chromium 156 e
  WebKit atuais devolveram 200 nos testes do site, que também pode pegar um surto de 304.
- Correção: `internal/rates` pede de novo, até 3 vezes, quando recebe 304, cada vez com um
  parâmetro único no endereço (`&_=…`; o Banco Central ignora parâmetros a mais). Vale para o
  app, o site e o computador. O `no-store` ficou (não atrapalha e evita o cache do WebView).
- Teste: `TestFetchRetries304` (Go: um e dois 304 seguidos dão certo; sempre 304 desiste no 3º
  pedido) e o e2e do ENG-010, em que o 1º pedido de cada série volta 304 mesmo sem cache. Os dois
  falharam antes da correção. No emulador, com um APK de teste, a taxa aparece e cinco rodadas
  seguidas das três séries voltam 200; não deu para forçar um surto de 304 do servidor ao vivo.
- Não conferido: celular de verdade.

## ENG-011 — No celular, o lançamento ia para a primeira categoria e a permissão de notificação voltava a cada gasto

Achados em 23/11/2026, num teste de uso com um APK de teste da main no emulador Android 14.
- Categoria. Esperado: "Uber" em Transporte. Observado: em "Contas da casa", a primeira da
  lista. Causa confirmada: o formulário de lançamento não sugeria nada; a regra existia só na
  importação e no Consultor (`importer.SuggestCategory`). O lançamento errado também ensina o
  histórico: o próximo "Uber" seria sugerido em "Contas da casa".
- Permissão. Observado: "Permitir notificações?" depois do 1º, do 2º e do 3º gasto salvo.
  Causa confirmada: o `scheduleReminders` reagenda a cada mudança e, sem permissão, saía sem
  guardar nada, então pedia de novo no gasto seguinte.
- Alarme exato. Observado: depois de liberar a notificação, o app abria a tela de sistema
  "Alarmes e lembretes". Causa confirmada no código do plugin (`@capacitor/local-notifications`
  8.3.1): `isExactNotification` vale `true` por padrão e, no Android 12+, sem permissão de
  alarme exato, o `schedule` abre essa tela.
- Correção: o formulário usa `importer.SuggestCategory` pelo `finance.wasm` até a pessoa
  escolher; sem a permissão, os lembretes ficam desligados (ligar nos Ajustes pede de novo e
  avisa); os lembretes vão com `isExactNotification: false` (às 9h, sem hora exata).
- Teste: `e2e/offline.test.mjs` (categoria) e `e2e/app.test.mjs` (permissão pedida uma vez,
  Ajustes, lembretes sem alarme exato); falharam antes da correção. No emulador, de uma
  instalação limpa: "Uber" em Transporte, "iFood" em Restaurantes, uma pergunta só, o aviso dos
  Ajustes e os alarmes agendados sem abrir a tela de sistema.
- Não conferido: celular de verdade.

## ENG-012 — Job Android do CI caindo com 429 do Maven Central

Em 24/11/2026, o job `android` dos PRs #45 e #46 falhou ao configurar o Gradle: o Maven Central
respondeu `429 Too Many Requests` para várias dependências (`kotlin-stdlib`, `httpmime`, `asm`).
Não tinha relação com o código; rodar de novo o job passou no #45.
- Causa provável: sem cache, cada rodada baixa todas as dependências do Gradle do Maven Central,
  e os endereços dos executores do GitHub são compartilhados.
- Correção: `cache: gradle` no `actions/setup-java` do job `android` (`.github/workflows/ci.yml`).
  O workflow de release, que publica o APK assinado, ficou sem cache de propósito; se cair por
  429, basta rodar de novo.
- Verificação: no CI do PR, o passo do Java restaura ou grava o cache do Gradle; a rodada
  seguinte baixa só o que mudou.

## ENG-013 — Números do app que a revisão do corpus da IA achou errados

Achado em 26/11/2026, na terceira revisão do corpus (três revisores independentes,
`.artifacts/ai-lab/corpus-review-a3q0ph26/`, fora do git). Mudança OpenSpec:
`openspec/changes/advisor-review-fixes/`.
- Ritmo do mês. Observado: "Gastando mais rápido que no mês passado ... R$ 206,32 a mais" com
  o mercado comprado no dia 8 nos dois meses e só R$ 10,00 a mais até o dia 22. Causa
  confirmada: a projeção multiplicava o gasto variável pelos dias do mês, e o aviso comparava a
  projeção, com fixos, com o total do mês passado. Correção: o resto do mês segue o que os meses
  anteriores gastaram depois deste dia (sem histórico, o ritmo de até hoje), e o aviso compara só
  o gasto variável até o mesmo dia. As parcelas mínimas que faltam pagar entram na projeção e
  saem do que ainda cabe no mês, como no Plano do mês. Teste: `TestInsightsMonthlyPurchasesAreNotPace`
  e `TestInsightsPendingDebtMinimum` falharam antes da correção.
- Onde economizar. Observado: "3 cobranças todo mês somam R$ 559,90 ... Vale revisar as que você
  não usa mais", com o mercado do mês e a Internet na conta, e "Pagamento de dívidas ... Voltar à
  média economiza R$ 500,00" quando a pessoa pagou a mais para quitar antes. Causa confirmada: a
  detecção de cobranças repetidas aceitava lugar de necessidade, a sugestão somava todas as
  cobranças e a sugestão por categoria não excluía o pagamento de dívida. Teste:
  `TestInsightsSavingsSkipCommitments` falhou antes da correção.
- Diagnóstico. Observado: "quitação em 1 mês" pondo nas dívidas a sobra de setembro, que ainda
  continha a parcela de R$ 300,00 não lançada, por cima do mínimo que o plano já paga. Correção:
  a sobra desconta as parcelas mínimas sem pagamento lançado no mês (com as dívidas de hoje).
  Teste: `TestDiagnoseUnpaidMinimumIsNotSurplus` falhou antes da correção.
- Vencimentos. Observado: com a parcela do dia 10 sem pagamento lançado e hoje dia 22, "o que
  vence?" só mostrava a do mês seguinte. Correção: `finance.LateDebts`, usado pela ferramenta e
  pelo novo aviso `debt-late` (#59). As contas fixas não entram: o app as lança sozinho no dia.
- Textos que a IA copia: "1 lançamentos", "1 compra(s)", "1 meses", datas `2026-09-02` e, no #52,
  uma ressalva longa como primeiro item da resposta sem IA "Plano das dívidas" (#57).
- Risco restante: uma compra do mês feita em dia diferente do mês passado (dia 8 agora, dia 25
  antes) ainda pode acusar ritmo até o dia em que a do mês passado caiu.
