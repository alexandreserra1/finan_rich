# Plano: SQLite no celular e na web (o mesmo banco em todo lugar)

## Resumo

Hoje o computador usa SQLite, e o celular e a web usam o IndexedDB (o banco do navegador).
O objetivo é ter um banco só, que sirva de 1 a 1.000 clientes (e além) sem precisar trocar de
novo:

- **Computador:** SQLite, como já é (`modernc.org/sqlite`, Go puro). Não muda.
- **Celular (app iOS e Android):** SQLite nativo do aparelho, pelo plugin
  `@capacitor-community/sqlite` 8.1.1 (MIT, para o Capacitor 8). Os dados saem do
  armazenamento do WebView, o que acaba com o bug conhecido do IndexedDB no iPhone.
- **Web (site e as telas no navegador do computador):** SQLite WebAssembly oficial,
  `@sqlite.org/sqlite-wasm` 3.54.1 (Apache-2.0; o SQLite em si é de domínio público). Roda
  num Worker e grava no OPFS (o armazenamento de arquivos do navegador), pelo modo
  `opfs-sahpool`, que não exige cabeçalhos especiais. É o mesmo modo que o Notion escolheu
  ao levar o SQLite para o navegador.

Se um dia existir um servidor central, ele usa Postgres (ADR-001). Isso é acréscimo, não
troca: os aparelhos continuam com o SQLite.

Por que isso serve de 1 a 1.000 clientes sem trocar: cada pessoa tem o próprio banco, com os
próprios dados. Mais clientes significa mais bancos pequenos, não um banco maior. Nenhum
servidor nosso cresce.

## Decisões

- **O formato dos dados não muda:** as três tabelas no aparelho espelham os armazéns de hoje.
  São elas `records (key, type, record)`, `outbox (key, type, record)` e `meta (key, value)`,
  com o registro em JSON. A sincronização, o Google Drive e as regras em Go (WebAssembly) não
  mudam.
- **A troca fica toda em `js/db.js`:** é o único arquivo que toca no IndexedDB. As funções que
  ele exporta (`loadLedger`, `allRecords`, `saveManyLocal`, `saveLocal`, `applyRemote`,
  `pending`, `clearSent`, `getMeta`, `setMeta`, `siteSession`, `wipe`) continuam com a mesma
  assinatura. Por dentro, ele fala com um de dois motores, os dois SQLite e com o mesmo SQL:
  o nativo (no app) ou o WebAssembly (na web).
- **Migração com backup:** na primeira abertura da versão nova, os dados do IndexedDB são
  copiados para o SQLite numa transação só. Depois a cópia é conferida (quantidade de
  registros, fila e meta) e marcada como feita. O IndexedDB fica intacto, como backup, por
  pelo menos duas versões. Se a cópia falhar, nada é marcado: a sessão continua no IndexedDB
  e a cópia tenta de novo na próxima abertura. Nenhum dado é apagado nesta mudança.
- **Uma aba de cada vez na web:** o modo `opfs-sahpool` só deixa uma aba abrir o banco. A aba
  nova pede o banco (Web Locks e BroadcastChannel). A antiga fecha o banco e mostra "A Rich
  está aberta em outra aba. Usar aqui", como faz o WhatsApp Web. O Notion teve corrupção com
  outro modo do OPFS e acesso concorrente; uma aba só com o banco evita isso.
- **Sem etapa de build de JS:** os arquivos do `sqlite-wasm` entram em `static/vendor/sqlite/`
  com LICENSE e README.txt, como o pdf.js. Os arquivos novos entram no `PRECACHE` do `sw.js`.
- **Fora deste plano** (cada um vem quando houver um caso concreto):
  - busca por significado para a IA (`sqlite-vec`);
  - tabela de cache com prazo de validade;
  - criptografia do banco do celular (SQLCipher, que o plugin oferece);
  - apagar o backup do IndexedDB.

## Tarefas

### Fase 0: provas antes de mexer nos dados (risco primeiro)

#### Tarefa 1: SQLite WebAssembly com OPFS funciona nos nossos navegadores

**Descrição:** pôr o `sqlite-wasm` em `static/vendor/sqlite/` e uma página de teste que cria
uma tabela, grava, recarrega e lê pelo `opfs-sahpool` num Worker. Medir o tamanho que ele
acrescenta e o tempo para abrir.

**Critérios de aceite:**
- [ ] grava e lê depois de recarregar no Chromium e no WebKit (Playwright), servido pelo
      programa do computador e pelo site;
- [ ] o FTS5 está presente nesse build;
- [ ] os números ficam anotados no plano: KB a mais (com gzip) e ms para abrir.

**Resultado (07/11/2026):** feito. `sqlite3.js` tem 657 KB (179 KB com gzip) e `sqlite3.wasm`
879 KB (407 KB com gzip): uns 586 KB a mais com gzip, perto de um décimo do `finance.wasm`. O
banco abre e grava em 55 a 178 ms, no Chromium e no WebKit, servido pelo computador e pelo
site. O FTS5 está no build. Achado: na janela temporária do Playwright, o WebKit se comporta
como a navegação anônima do Safari e não tem OPFS (erro "unknown transient reason"). Com um
perfil normal funciona. Por isso o IndexedDB fica como reserva quando o OPFS falhar (responde
à pergunta em aberto).

**Verificação:** e2e novo `e2e/sqlite.test.mjs` nos dois motores; `scripts/check-licenses.sh`.

**Depende de:** nada. **Arquivos:** `internal/web/static/vendor/sqlite/*`,
`e2e/sqlite.test.mjs`, `internal/web/static/sw.js`. **Tamanho:** S.

#### Tarefa 2: o plugin nativo compila no Android e no iOS

**Descrição:** adicionar `@capacitor-community/sqlite` 8.1.1 ao `mobile/package.json`, rodar
`make mobile`/`make apk` e o CI do iOS.

**Critérios de aceite:**
- [ ] o APK de teste compila;
- [ ] o job `ios` do CI fica verde;
- [ ] a verificação de licenças passa;
- [ ] o aumento do APK fica anotado.

**Verificação:** `make apk`, CI (`android`, `ios`), `scripts/check-licenses.sh`.

**Depende de:** nada. **Arquivos:** `mobile/package.json`, `mobile/package-lock.json` e os
arquivos versionados do `cap sync`. **Tamanho:** S.

**Resultado (07/11/2026):** o plugin entrou em `mobile/package.json` e no lock (só acrescenta
pacotes) e `capacitor.config.json` ganhou `"CapacitorSQLite": { "androidIsEncryption": false }`.
Com o padrão `true`, o plugin cria uma chave no Keystore a cada abertura; se o backup do Android
restaurar o arquivo dela em outro aparelho, o plugin não inicia e o app fica sem banco. A
verificação de licenças passa. O CI roda `npm ci` e `cap sync` antes de compilar, então os
arquivos nativos gerados não precisam ir para o commit (os três da lista de não commitar
continuam fora). Esta máquina não tem Java nem Xcode: o APK não foi compilado aqui.

**Achado que pede decisão:** no Android o plugin traz o SQLCipher, com bibliotecas nativas de
7,6 MB (quatro arquiteturas). Pela estimativa, o APK passa de 8 MB para uns 16 MB, acima do
limite de 12 MB do CI. Opções: tirar a arquitetura x86 (só emuladores antigos de 32 bits; o
x86_64 fica para o emulador do CI), comprimir as bibliotecas no APK (`useLegacyPackaging`), um
APK por arquitetura, ou subir o limite. Até a decisão, o plugin não vai para o commit, e o app
continua no IndexedDB (o `db.js` só usa o motor nativo quando o plugin existe).

### Ponto de controle A
- [ ] números das tarefas 1 e 2 conferidos com o Alexandre (segue ou para).

### Fase 1: web

As tarefas 3, 4 e 5 saíram juntas: o motor SQLite sem a migração apagaria da tela os dados de
quem já usa, e sem "uma aba de cada vez" a segunda aba ficaria sem banco. Assim não precisou da
chave para ligar e desligar.

#### Tarefa 3: `db.js` com o motor SQLite da web, desligado por padrão

**Descrição:** `db.js` ganha o motor SQLite WebAssembly (Worker, `opfs-sahpool`, as três
tabelas), mantendo as mesmas funções exportadas. Uma chave interna escolhe o motor; os testes
rodam com o SQLite ligado.

**Critérios de aceite:**
- [ ] com o SQLite ligado, todos os e2e de hoje passam no Chromium e no WebKit;
- [ ] com ele desligado, nada muda para quem usa.

**Verificação:** `make test`, `make e2e`, `cd e2e && npm run webkit`.

**Depende de:** tarefa 1. **Arquivos:** `js/db.js`, `js/sqlite-worker.js` (novo),
`sw.js`. **Tamanho:** M.

#### Tarefa 4: migração do IndexedDB para o SQLite e motor ligado

**Descrição:** na abertura, copiar o IndexedDB para o SQLite (uma transação, conferência,
marca de feito). O IndexedDB fica intacto. O SQLite passa a ser o padrão na web.

**Critérios de aceite:**
- [ ] e2e: dados criados pela versão anterior (IndexedDB) aparecem iguais depois da troca,
      incluindo a fila de envio e o cursor da sincronização;
- [ ] e2e: uma falha no meio da cópia não perde nada (continua no IndexedDB e tenta de novo);
- [ ] o IndexedDB continua lá depois da migração.

**Verificação:** e2e novo de migração nos dois motores; `make e2e`.

**Depende de:** tarefa 3. **Arquivos:** `js/db.js`, `e2e/migrate.test.mjs` (novo).
**Tamanho:** M.

#### Tarefa 5: uma aba de cada vez

**Descrição:** Web Locks e BroadcastChannel. A aba nova assume o banco e a antiga mostra
"A Rich está aberta em outra aba. Usar aqui".

**Critérios de aceite:**
- [ ] e2e com duas abas: a nova grava, a antiga mostra o aviso, "Usar aqui" volta o banco
      para ela, e nada se perde nem corrompe.

**Verificação:** e2e nos dois motores.

**Depende de:** tarefa 4. **Arquivos:** `js/db.js`, `js/main.js`, `app.css`.
**Tamanho:** S.

### Ponto de controle B
- [ ] `make test` e `make e2e` passam (Chromium e WebKit); CI verde;
- [ ] conferência à mão no navegador do computador e no site; commit e push.

### Fase 2: celular

#### Tarefa 6: motor SQLite nativo no app e migração do IndexedDB do WebView

**Descrição:** no app, `db.js` usa o plugin nativo (mesmo SQL e mesmas tabelas) e migra o
IndexedDB do WebView do mesmo jeito da tarefa 4.

**Critérios de aceite:**
- [ ] teste do motor nativo com um plugin falso (mesmas operações da web);
- [ ] o e2e do app (`e2e/app.test.mjs`, no navegador) continua verde;
- [ ] o APK e o iOS compilam.

**Verificação:** `make e2e`, `make apk`, CI.

**Depende de:** tarefas 2 e 4. **Arquivos:** `js/db.js`, `js/native.js` e um teste novo.
**Tamanho:** M.

**Resultado (07/11/2026):** `js/sqlite-native.js` implementa o mesmo `run(ops)` sobre o plugin,
com as chamadas numa fila. O `db.js` usa ele no app quando o plugin existe. O teste
`e2e/native.test.mjs` usa um plugin falso (SQLite WebAssembly na memória, com as manias do
plugin: `ios_columns`, conexão que sobrevive a recarregar) e confere o contrato e o banco por
cima. Ele falha se tirar a fila, o descarte do `ios_columns` ou o desfazer. Falta o plugin
entrar no app (decisão do tamanho do APK, tarefa 2) e o teste num aparelho (tarefa 7).

#### Tarefa 7: teste num aparelho de verdade (Alexandre)

**Critérios de aceite:**
- [ ] no Android e no iPhone, com dados da versão anterior: depois de atualizar, tudo
      aparece, sincroniza com o computador e continua lá depois de horas com o app em
      segundo plano.

**Depende de:** tarefa 6.

### Ponto de controle C
- [ ] tarefa 7 confirmada pelo Alexandre antes de publicar a versão.

### Fase 3: documentação e versão

#### Tarefa 8: docs, ADR-002 e novidades

**Descrição:** ADR-002 (aparelhos passam ao SQLite; substitui a parte de aparelhos do
ADR-001), `ARCHITECTURE.md`, `MOBILE.md`, `README.md` e as novidades da versão.

**Depende de:** tarefa 7. **Tamanho:** S.

#### Tarefa 9 (pelo menos duas versões depois): apagar o backup do IndexedDB

**Descrição:** remover a leitura do IndexedDB e apagar o banco antigo, com aviso nas
novidades.

## Riscos

| Risco | Impacto | O que fazer |
|---|---|---|
| A migração perde dados | Alto | O IndexedDB fica intacto; a cópia é conferida antes de ser marcada; e2e com dados da versão anterior e com falha no meio; o computador e o Drive guardam outra cópia |
| OPFS indisponível em algum navegador (Safari antigo, janela anônima) | Médio | A tarefa 1 mede; se faltar, a sessão continua no IndexedDB (o código atual), sem perda |
| Duas abas no mesmo banco | Médio | Uma aba com o banco de cada vez (tarefa 5) |
| O motor nativo não roda no Playwright | Médio | Plugin falso no teste, builds no CI e teste em aparelho antes de publicar (tarefa 7) |
| A abertura fica mais lenta (o Notion viu isso) | Baixo | Carregar o SQLite no Worker em paralelo com o `finance.wasm`; a tarefa 1 mede |
| Plugin da comunidade sem manutenção | Baixo | É o plugin padrão de SQLite do Capacitor, MIT, com versão de outubro de 2026; o SQL é o mesmo, então trocar de plugin mexe só no motor nativo |

## Perguntas em aberto

- Se a tarefa 1 mostrar navegador sem OPFS: manter o IndexedDB como reserva para sempre
  nesses casos ou mostrar "navegador sem suporte"? Recomendo manter a reserva.
