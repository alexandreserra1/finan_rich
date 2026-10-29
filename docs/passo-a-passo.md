# Passo a passo: o que a auditoria apontou, o que é verdade e o que fazer

Uma auditoria externa (feita por outra IA lendo o repositório e o site, sem rodar o app)
listou riscos e melhorias. Em 29/10/2026 cada ponto foi conferido no código e nos testes. Aqui
fica só o que vale: o que já estava certo, o que foi corrigido (com o commit) e o que falta, em
ordem de prioridade, com os arquivos e os passos.

Legenda: **feito** (já estava certo ou foi corrigido) · **a fazer** (real, ainda aberto) ·
**não procede** (a auditoria se enganou ou ficou desatualizada).

O plano da IA (respostas sem IA, vários provedores, computador como provedor) e das telas que
faltam está em [PLANO.md](PLANO.md).

## 1. Não perder dados (prioridade máxima)

### 1.1 Sair da conta apagava antes de salvar · feito

- **O que era:** no programa do computador acessado pela rede (outro aparelho abre o finan_rich
  do PC no navegador, com senha), o botão Sair apagava o banco do navegador (`db.wipe()`) antes
  de qualquer conferência. Lançamento feito ali e ainda na fila (sem conexão com o PC) se perdia.
  Era real. O Sair do **site** (com Google) já salvava no Drive antes de apagar; a auditoria
  misturou os dois.
- **Corrigido em `4e075f2`:** `internal/web/static/js/main.js` chama `sync.flushBeforeLeaving()`
  (`internal/web/static/js/sync.js`), que sincroniza e devolve quantas alterações ainda estão
  na fila. Só apaga com zero; senão avisa quantas faltam e não apaga nada. Teste em
  `e2e/offline.test.mjs` (com o PC desligado e depois ligado).
- **Falta conferir:** o aviso na tela num acesso de verdade pela rede (o teste roda em
  127.0.0.1, onde esse Sair não aparece).

### 1.2 Sincronização e conflitos

| Caso | Situação | Onde |
|---|---|---|
| Alteração offline enviada depois | feito, com teste | fila `outbox` em `db.js`; `e2e/offline.test.mjs` |
| Mesmo registro mudado em dois aparelhos | feito: vence o `updatedAt` maior; empate, o conteúdo maior | `internal/store/records.go`, `db.applyRemote`; `TestApplyChangesLastWriteWins` |
| Edição antiga chegando depois de apagar | feito: não ressuscita (teste acrescentado em 29/10) | `internal/store/store_test.go` |
| Lote interrompido no meio | feito: a fila só é limpa depois que o computador confirma o lote | `sync.js` (`round`, `clearSent` após 200) |
| Dois aparelhos gravando o Drive juntos | feito: quem perdeu regrava na próxima sincronização | `internal/drivesync`, `gdrive.js`; `e2e/google.test.mjs` |
| Relógios diferentes, edição simultânea offline | correção no computador e no Drive; conferência em aparelhos reais pendente | ver 1.3 |

### 1.3 Relógio do aparelho em edições simultâneas · implementado no computador e no Drive

- **O problema, no tamanho certo:** o `updatedAt` vem do relógio de quem editou, mas o carimbo é
  `máximo(relógio, versão anterior + 1)`: com a versão mais nova recebida, a edição sempre vence.
  O relógio só decidia quando dois aparelhos sem conexão mudavam o **mesmo** registro antes de
  sincronizar (vencia o relógio maior, não quem editou por último).
- **Corrigido:** a resposta do `/api/sync` traz a hora do computador (`now`); o aparelho guarda a
  diferença (descontando metade da ida e volta; até 2 s é ruído) e soma no carimbo de toda
  edição, com `sync.stamp` (`main.js`, `notes-save.js`, restaurar backup). Testes em
  `internal/web/web_test.go` e `e2e/offline.test.mjs` (aparelho com o relógio 1 hora atrasado).
- **Drive (site):** `gdrive.js` pede `modifiedTime` ao criar ou atualizar o arquivo e passa a
  hora para `sync.learnClock`. O carimbo usa a mesma correção do computador. O teste em
  `e2e/google.test.mjs` simula um navegador uma hora atrasado; passou na execução do Claude
  em 29/10/2026. O ajuste é aprendido após uma gravação bem-sucedida, não antes da primeira
  sincronização. Não resolve a ordem real de duas edições feitas sem referência de hora.

## 2. Segurança

### 2.1 Site só com login · feito

A auditoria achou a contradição entre "usar sem conta" e "precisa de login". Decidido: o site
só funciona com Entrar com Google (`2398954`): sem login, nenhuma tela do app aparece, nem
digitando o endereço; sessão vence após 30 minutos parado; sair salva no Drive e apaga o
navegador. README corrigido em `6f57882`; regras em [GOOGLE.md](GOOGLE.md). O computador e o
app do celular continuam sem conta (o aparelho é da pessoa).

### 2.2 Token de renovação do Google no navegador · feito

- **Era verdade:** no site, o token de renovação ficava no IndexedDB, ao alcance de qualquer
  script da página.
- **Corrigido:** o intermediário (`internal/proxy/oauth.go`) guarda o token num cookie
  `__Host-finan_rt` (`HttpOnly`, `Secure`, `SameSite=Strict`, 6 meses) quando a página pede com
  `X-Finan-Cookie: 1`; a resposta do login não traz mais o token; `/oauth/refresh` lê o cookie;
  `/oauth/logout` apaga o cookie e revoga no Google. O `google.js` não guarda mais o token no
  site, e quem entrou antes passa o token antigo para o cookie na primeira renovação. Testes em
  `internal/proxy/oauth_test.go` e `e2e/google.test.mjs` (o token não aparece no banco nem em
  `document.cookie`). O app do celular continua com o cofre do sistema.
- **Já protegia antes:** CSP só com scripts do próprio site; `html\`\`` escapa tudo; o markdown
  do Consultor escapa `& < >` e não gera atributos.

### 2.3 Outras conferências · feito

Tokens e chaves nunca vão para log nem URL (registro de tokens do intermediário guarda só
números; ver [IA-GRATIS.md](IA-GRATIS.md)). Rotas OAuth com limite por IP. Upload do extrato com
tamanho máximo. Chave do computador local fora do repositório (`scripts/finanlocal.sh`, e
`.gitignore` com `*.key`).

## 3. Testes dos fluxos de verdade

A lista da auditoria, conferida nos e2e:

| Fluxo | Situação |
|---|---|
| Começar, lançar receitas e despesas, ver o mês | feito: `budget.test`, `plans.test` |
| Importar o mesmo extrato duas vezes | feito: `import.test` ("reimportar não duplica"; Open Finance também) |
| Fatura com parcelas e saldo do cartão | feito: `accounts.test` |
| Mudar offline e sincronizar depois | feito: `offline.test` |
| Sair com alterações pendentes | feito: `offline.test` (desde `4e075f2`) e `google.test` (Drive fora do ar) |
| Trocar de aparelho e recuperar os dados | feito: `google.test` (segundo navegador pelo Drive) e pareamento em `offline.test` |
| Exportar e restaurar o backup | feito: `export.test` (apaga tudo e restaura) |
| IA, Drive ou computador fora do ar | feito: `app.test` (IA no limite, internet caindo, resposta sem IA), `google.test`, `offline.test` |
| Relógio errado com edição simultânea offline | feito: `offline.test` (relógio 1 hora atrasado) |

## 4. Produto e telas

| Ponto da auditoria | Situação |
|---|---|
| Painel como centro de decisão (situação, atenção, ação, evolução) | feito no redesign: avisos assinados pelo time, "O que o time viu", números do mês no topo |
| Avisos no sininho | feito em 29/10/2026: os avisos saíram do corpo do Painel para um sininho no alto de toda tela (app, site e computador), com o número de avisos. Tocar num leva à tela dele; "Limpar tudo" esvazia a lista em todos os aparelhos; o aviso volta se a situação mudar (`finance.Alert.ID`, `PendingAlerts`). Testes em `internal/finance/plans_test.go` e `e2e/budget.test.mjs` |
| Do aviso para a conversa | feito em 29/10/2026: cada aviso e cada resumo do time tem "Perguntar para ...", que abre o Consultor com quem avisou e a pergunta já escrita (app, site e computador). Antes, o aviso só levava à tela do assunto e o "Conversar" abria com o último personagem escolhido. Testes em `e2e/budget.test.mjs` e `e2e/app.test.mjs` |
| Lançamento rápido, com desfazer | feito: janela curta, Desfazer depois de apagar |
| Dívidas com cenários (mínimo, valor a mais, ordem) | feito: tela Dívidas e o Bento (`simular_dividas`) |
| Consultor que separa conta do Go e texto da IA | feito: cartão "Os números do app" (Go) na hora, a IA explica embaixo; sem IA, o cartão é a resposta |
| Ajustes mostrando onde ficam os dados e o que vai para a IA | feito: Ajustes redesenhado, com a escolha "IA do finan_rich" (apelidos, limite por dia) ou "minha própria chave" |
| Estados vazios, carregamento, erro padronizados | feito em todas as telas redesenhadas |
| Acessibilidade e contraste | feito: teste de contraste dos temas; gráficos com tabela |

## 5. Medir antes de otimizar · medido e otimizado no computador

A auditoria não mediu desempenho. Passos: medir o tempo até o Painel aparecer (com o
`finance.wasm`), com 5 mil lançamentos, e o tamanho do que o site baixa; só então decidir se
vale otimizar. Já medido: consultor no computador local, de 13 a 35 s por pergunta (ver
[IA-GRATIS.md](IA-GRATIS.md)).

Em 29/10/2026, o levantamento local dos 55 arquivos estáticos do precache deu
**8.414.189 bytes sem compressão** e **2.553.917 bytes com gzip estimado**. O `finance.wasm`
responde por 5.502.497 bytes (1.507.092 com gzip). Não inclui HTML, dados nem cabeçalhos;
não é uma medição da transferência da Vercel. O precache também baixa bibliotecas de PDF
que não são necessárias para desenhar o Painel.

Medição reproduzível, a partir da raiz, após instalar as dependências de `e2e/` e o Chromium:

```bash
make wasm
go build -o dist/finanrich ./cmd/finanrich
cd e2e && npm run performance
```

O script cria um banco temporário, confere 5 mil lançamentos e mede cinco aberturas e cinco
recargas do Painel, com as regras em WebAssembly. Mostra as amostras, a mediana e o ambiente.
Usa o computador local, sem limitar CPU/rede e com service worker bloqueado; a recarga mantém
IndexedDB e cache HTTP. Não mede login/Drive, acesso à Vercel nem aparelho real. Os tamanhos
são calculados antes de abrir o navegador, a partir do precache em `sw.js`.

Após autorizar a execução fora do sandbox, a medição terminou em 29/10/2026 no Apple M1 Pro
(macOS arm64, Chromium 141.0.7390.37), sem outras suítes rodando em paralelo:

| Amostra | Primeira abertura | Recarga |
|---|---:|---:|
| 1 | 5.391 ms | 2.617 ms |
| 2 | 2.729 ms | 2.045 ms |
| 3 | 2.761 ms | 2.346 ms |
| 4 | 2.768 ms | 2.533 ms |
| 5 | 2.761 ms | 2.330 ms |
| **Mediana** | **2.761 ms** | **2.346 ms** |

Cada primeira abertura usa um contexto novo, com IndexedDB vazio; a recarga mantém os
dados e o cache HTTP. A medição espera os 5 mil registros, o Painel visível e dois quadros
de animação. Não é LCP nem INP. O navegador registrou 2.792.392 bytes de recursos na primeira
abertura e 13.262 bytes na recarga; essa soma não inclui o documento principal nem o
precache do service worker, que fica bloqueado nesse cenário. A primeira amostra inclui
aquecimento do processo; por isso foram preservadas todas as amostras e usada a mediana.

**Otimização (29/10/2026).** O perfil mostrou o gargalo: cada soma do Painel chamava
`ActiveTransactions()`, que copia e ordena todos os lançamentos, umas 20 vezes por abertura.
As somas em que a ordem não importa (`summary.go`, `budgets.go`, `goals.go` e
`placeCategories` em `insights.go`) passaram a percorrer a lista direto, pulando os apagados.
Resultado, com os mesmos 5 mil lançamentos:

| Medida | Antes | Depois |
|---|---:|---:|
| `go test ./internal/finance -bench Painel` (nativo) | 53 ms | 13 ms |
| Painel, primeira abertura (mediana, `npm run performance`) | 2.761 ms | 1.036 ms |
| Painel, recarga (mediana) | 2.346 ms | 781 ms |

O que ainda pesa: cada chamada ao WebAssembly decodifica o JSON de todos os lançamentos
(uns 45 ms com 5 mil) e o `finance.wasm` tem 5,5 MB (1,5 MB com gzip). Rede móvel, Vercel,
login/Drive e aparelhos reais ainda precisam de medição própria.

## Conferência da retomada (29/10/2026)

- **Safari:** alternativa de crescimento da caixa de mensagem em `chat.js` e
  `js/pages/advisor.js`. O teste de `/chat` no Chromium, simulando ausência de `field-sizing`,
  passou também na execução do Codex (38 → 100 px). Safari real e o fluxo do app continuam pendentes.
- **Validação no Codex:** formatação, `go vet` e os seis pacotes de regras em WebAssembly
  passaram. Fora do sandbox, as dez suítes de `make e2e` passaram, incluindo o relógio do
  Drive e a alternativa para `field-sizing`. `make test` também passou após corrigir
  `TestPairingNeedsAddress`: o teste consultava o Tailscale real do Mac, embora esperasse um
  cenário sem endereço. A detecção agora é substituída por servidor nos testes de acesso,
  com cobertura separada para Tailscale indisponível e conectado sem acesso pelo Wi-Fi.
  No programa, a detecção continua automática e o comportamento de pareamento é o mesmo.
- **Consultor · feito:** o cartão "Os números do app" fica salvo com a resposta no banco do
  computador (migração `009_chat_numbers.sql`) e volta ao recarregar: recolhido em cima da
  resposta da IA, ou aberto quando a resposta foi do app, sem IA. O app do celular recebe os
  mesmos números pelo `/api/chat`, e o cartão que a pessoa abriu ou fechou continua assim
  quando a tela é redesenhada. Testes em `internal/web/web_test.go` e `e2e/app.test.mjs`.
- **Configuração externa:** o Tailscale está instalado e conectado no Mac (29/10/2026). O
  `scripts/finanlocal.sh` agora acha o comando dentro do app do Tailscale. Faltam: rodar o
  script no Terminal, aprovar o HTTPS e o Funnel no link que ele mostra, pôr
  `FINAN_LOCAL_URL`/`FINAN_LOCAL_KEY` na Vercel, e configurar `GOOGLE_*` (ver [GOOGLE.md](GOOGLE.md)). A correção da identidade dos
  commits já foi publicada, conforme o registro da sessão anterior; não repetir o force push.

## Safari (motor WebKit), 29/10/2026

Os testes no navegador agora rodam também no WebKit, o motor do Safari (`cd e2e && npm run
webkit`; versão 26.0). Resultado:

- **Erro real, corrigido:** a fatura em PDF não era lida no Safari ("undefined is not a
  function"). O pdf.js 6 percorre `ReadableStream` com `for await`, que o Safari 26 não tem;
  `js/pdftext.js` completa isso antes de carregar o pdf.js. O `e2e/import.test.mjs` simula o
  Safari no Chromium para a falha não voltar.
- **Corrida no teste, corrigida:** a troca de personagem no Consultor (`e2e/app.test.mjs`)
  esperava a mensagem pendente, que já tem o nome.
- **A conferir no iPhone:** no WebKit do Playwright, a página trava ("Page crashed") ao
  recarregar o Painel logo depois que ele aparece (offline, app e budget). Não é memória
  (o WebAssembly usa 6 MB), nem o gráfico, nem as animações, nem o service worker; outras
  telas recarregam sem problema. Pode ser defeito do WebKit do Playwright: conferir no Safari
  do iPhone abrindo o Painel e recarregando algumas vezes seguidas.
- **Limite do Playwright:** no WebKit, o Google falso não pode responder com redirecionamento
  (302), então `google.test` só roda no Chromium.

## Ordem de execução

1. Manter `make test` e `make e2e` como verificação das próximas alterações.
2. Conferir as telas novas (Consultor, Ajustes, Boas-vindas) num celular e num Safari de verdade.
3. **5** Medir também em rede móvel e no site publicado.
4. Conferir o Sair do acesso pela rede num aparelho de verdade (1.1).
