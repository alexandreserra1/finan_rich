# ADR-002: SQLite também nos aparelhos (web e celular)

## Situação

Aceita. Substitui a parte dos aparelhos do [ADR-001](ADR-001-banco-de-dados.md), que mantinha o
IndexedDB. A web já usa o SQLite; o app do celular passa a usar quando o motor nativo for
publicado (plano em `tasks/plan.md`).

## Data

07/11/2026

## Contexto

O Alexandre quer o mesmo banco no celular e na web, grátis, com espaço para os dados da IA e
que sirva de 1 a 1.000 clientes sem precisar trocar de novo. Também pediu o mais robusto a
longo prazo.

- Só dois bancos rodam no celular e na web: SQLite (nativo no celular, WebAssembly na web) e
  Postgres (pelo PGlite, WebAssembly nos dois). As opções "NoSQL" para navegador (RxDB,
  PouchDB) são camadas por cima do próprio IndexedDB.
- No Safari, o PGlite só grava no IndexedDB (o modo OPFS não funciona lá) e ainda está na
  versão 0.x.
- No iPhone há um bug conhecido do IndexedDB dentro do app: a conexão cai depois de horas com
  o app em segundo plano (WebKit 277615).
- No celular, o padrão do mercado é o SQLite nativo. O Notion usa SQLite no navegador
  (WebAssembly com OPFS, modo `opfs-sahpool`) e no app, com Postgres no servidor.
- Servidores de banco (Postgres, Aurora, Cassandra, Redis) não rodam dentro do aparelho. Eles
  só fazem sentido se um dia houver servidor central (ADR-001).

## Decisão

SQLite em todo lugar:

- **Computador:** SQLite (`modernc.org/sqlite`), como já era.
- **Web:** SQLite WebAssembly oficial (`vendor/sqlite`, 3.54.1), num Worker, gravando no OPFS
  pelo modo `opfs-sahpool`.
- **App do celular:** SQLite nativo pelo plugin `@capacitor-community/sqlite` 8.1.1.

O SQL fica num lugar só (`js/db-sqlite.js`), igual na web e no app. As tabelas espelham o
formato de antes (`records`, `outbox`, `meta`, com JSON), então a sincronização, o Google Drive e
as regras em Go não mudaram.

- **Migração:** na primeira abertura, o IndexedDB é copiado numa transação só, conferido
  (contagens dentro da transação) e marcado nas duas pontas. O IndexedDB fica intacto, como
  backup. Se a cópia falhar, a sessão segue no IndexedDB e tenta de novo na próxima abertura.
  Depois da cópia, nunca volta para o IndexedDB (estaria desatualizado): se o SQLite não abrir,
  avisa.
- **Reserva:** onde o SQLite não abre e a cópia nunca aconteceu (navegação anônima do Safari,
  que não tem OPFS), fica no IndexedDB.
- **Uma aba de cada vez:** o `opfs-sahpool` só deixa uma aba abrir o banco. A aba nova pede
  (Web Locks e BroadcastChannel); a antiga termina o que estava gravando, solta o banco e
  mostra "Usar aqui".
- **O que não vira JSON:**
  - bytes (`Uint8Array` e `ArrayBuffer`) viram `{ $u8 }` e `{ $ab }` e voltam a ser bytes;
  - chaves de criptografia (`CryptoKey`, a que cifra a chave da IA no navegador) continuam no
    IndexedDB, que as guarda sem expor, e o SQLite guarda só uma marca.
- **Cache:** fica na memória do programa e no `meta`. Uma tabela com prazo de validade entra
  quando houver um caso concreto. A IA pode usar a busca por texto (FTS5, presente nos dois
  builds) e, no futuro, a busca por significado (`sqlite-vec`).

## Alternativas consideradas

- **Postgres (PGlite) no celular e na web:** é o mesmo banco do servidor do Pierre. Mas está na
  versão 0.x, são 3 MB a mais, grava no IndexedDB no iPhone (o bug continua) e o computador
  precisaria de um Postgres embutido. Recusado.
- **Manter o IndexedDB:** não é o mesmo banco do computador e tem o bug do iPhone. Recusado.
- **SQLite WebAssembly também dentro do app:** seria o mesmo código da web, mas o OPFS do
  iPhone passa pelo mesmo armazenamento do WebView que tem o bug. Recusado em favor do nativo.

## Consequências

- **Mesmo banco:** computador, web e celular rodam o mesmo banco e o mesmo SQL.
- **Tamanho na web:** são uns 586 KB a mais com gzip para baixar (uma vez, depois fica no
  cache). O banco abre em 55 a 178 ms nos testes.
- **Tamanho no Android:** o plugin traz o SQLCipher, com bibliotecas nativas de 7,6 MB nas
  quatro arquiteturas. Pela estimativa, o APK passa de uns 8 MB para uns 16 MB. O limite do CI
  subiu de 12 MB para 18 MB: ainda é um app pequeno, e as outras saídas (tirar a arquitetura
  x86, um APK por arquitetura) ou não bastavam ou mudariam a publicação e o F-Droid.
- **iOS:** o SQLCipher conta como criptografia para a regra de exportação da Apple. É preciso
  ver o `ITSAppUsesNonExemptEncryption` antes da App Store.
- **Testes:**
  - os testes no navegador cobrem os dois caminhos: o Chromium usa o SQLite, e o WebKit, na
    janela temporária, usa a reserva no IndexedDB;
  - `e2e/sqlite.test.mjs` e `e2e/migrate.test.mjs` usam perfis persistentes, para o WebKit
    também testar o SQLite;
  - o motor nativo foi conferido num emulador Android 14 (07/11/2026): a v1.10.0 com dados
    no IndexedDB, atualizada pelo APK de release (com o R8), passou tudo para o SQLite nativo,
    sincronizou e manteve os dados depois de fechar e de o Android matar o app em segundo
    plano. Falta o aparelho de verdade, e o iPhone (sem Xcode nesta máquina).
- **Backup:** o IndexedDB de antes fica por pelo menos duas versões, só com os registros, a
  fila, a marca e as chaves de criptografia. O `meta` antigo (tokens de pareamento e do Google,
  o hash da senha do app) sai logo depois da cópia, para não ficar segredo esquecido no disco.
  Apagá-lo inteiro é uma tarefa separada.
- **"Apagar os dados do aparelho":** apaga tudo, menos a marca da migração. Sem ela, a próxima
  abertura tentaria copiar de novo e cairia no IndexedDB, escondendo o que foi gravado depois
  (achado na revisão e coberto por teste).
- **Aba na versão anterior durante a atualização:** a versão anterior abria o IndexedDB na
  versão 1 e nunca fechava a conexão. A nova abre na versão 2, e o navegador só deixa abrir
  depois que todas as conexões da versão 1 fecham. Até lá, a aba nova mostra "A Rich está
  aberta em outra aba" e pede para recarregar ou fechar a outra. Depois disso, a versão
  anterior não abre mais o banco (`VersionError`). Por isso, a cópia para o SQLite nunca deixa
  para trás o que uma aba antiga gravou (coberto por teste).
- **Abrir o SQLite logo depois de recarregar:** o Worker da página anterior pode ainda segurar
  os arquivos do banco por um instante. O Worker novo tenta de novo, sem prazo, só nesse caso (erro
  `NoModificationAllowedError` no Chromium, `InvalidStateError` no WebKit), e a tela de espera
  aparece depois de 3 segundos. No WebKit do CI, o Worker de uma aba fechada segurou o arquivo
  por mais de 30 segundos; desistir só faria a pessoa recarregar e esperar de novo.
  Abre com `preserveOnInitFailure`: no padrão do pacote, uma falha ao abrir apaga a pasta do
  banco (achado ao investigar uma falha do CI, coberto por teste).
