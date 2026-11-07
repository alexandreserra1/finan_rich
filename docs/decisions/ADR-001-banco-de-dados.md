# ADR-001: Manter o SQLite e o banco do navegador, com os registros em JSON

## Situação

Aceita. A parte dos aparelhos (IndexedDB) foi substituída pelo
[ADR-002](ADR-002-sqlite-nos-aparelhos.md): a web e o celular também passam a usar SQLite.

## Data

07/11/2026

## Contexto

Surgiu a ideia de trocar o banco de dados (DuckDB, ou um banco NoSQL) para "já ter o que
precisa quando escalar". O banco teria de guardar três tipos de dado:

- os dados financeiros (lançamentos, categorias, dívidas, metas);
- o que a pessoa escreve (notas, conversas, o que a IA aprende com ela);
- cache (resultados guardados por um tempo, que não precisam sincronizar).

Escalar, neste produto, quer dizer **mais pessoas, cada uma com os próprios dados no próprio
aparelho**. Não existe servidor do finan_rich com os dados de todos, e as regras da casa
(só o que é grátis; os dados não passam por servidor nosso) mantêm isso.

Como os dados ficam hoje:

- **Computador:** SQLite (`finan.db`) com o driver em Go puro `modernc.org/sqlite`, sem CGO,
  em modo WAL e com uma conexão só.
- **Celular e navegador:** IndexedDB (`js/db.js`), com os armazéns `records` (cópia dos dados),
  `outbox` (fila do que falta sincronizar) e `meta` (cursor e caches locais).
- **Formato:** desde a migração `006_open_types.sql`, todos os dados sincronizados ficam numa
  tabela só, `records (type, id, data, updated_at, deleted, rev)`, com `data` em JSON e sem
  lista fixa de tipos. Quem valida o tipo é o `domain.DecodeRecord`, no computador e no
  aparelho. No IndexedDB é igual: `{key: "tipo:id", type, record}`.
- **Sincronização:** vence o `updatedAt` maior; no empate, o conteúdo maior em ordem
  alfabética (`store` e `db.applyRemote`). A sincronização pelo Google Drive usa os mesmos
  registros (`internal/drivesync`).

## Decisão

Manter o SQLite no computador e o IndexedDB no aparelho, com os registros em JSON por tipo.

- **Dado novo que sincroniza** (o que a pessoa escreve, a memória da IA): vira um tipo novo
  de registro. A sincronização, o backup e o Drive já levam qualquer tipo.
- **Cache que não sincroniza:** fica na tabela `settings` do computador e no `meta` do
  aparelho, como o cache do Open Finance. Uma tabela de cache com prazo de validade só entra
  quando aparecer um caso concreto.
- **Busca por texto:** quando for preciso, usar o FTS5 do próprio SQLite. Ele já vem no driver
  atual (conferido em 07/11/2026), sem dependência nova.

## Alternativas consideradas

### DuckDB

- **A favor:** é muito rápido para análises sobre milhões de linhas; licença MIT.
- **Contra:**
  - É feito para análise, não para as muitas gravações pequenas de um app de finanças.
  - Só um processo pode abrir o arquivo para gravar.
  - A biblioteca para Go depende de C (CGO), o que acaba com o build simples para Windows,
    Linux e Mac. A biblioteca também é grande e aumentaria o programa em dezenas de MB.
  - Não ajuda o celular, cujos dados ficam no IndexedDB. Lá só daria com a versão WebAssembly
    do DuckDB, que é pesada.
  - Os relatórios são calculados em Go sobre os dados na memória, não em SQL. Trocar o banco
    não os deixaria mais rápidos.
- **Recusado.** Pode voltar como camada extra só de leitura, em cima do backup, se um dia
  houver análise pesada. Ele não substituiria o SQLite.

### MongoDB

- **Contra:** a licença (SSPL) não é de código aberto e a regra de licenças do projeto
  (`scripts/check-licenses.sh`) não aceita. Ele também precisa de um servidor rodando, o que
  não serve para um programa que a pessoa abre no próprio computador.
- **Recusado.**

### CouchDB com PouchDB

- **A favor:** é o NoSQL clássico para quem trabalha sem internet e sincroniza depois;
  licença Apache-2.0.
- **Contra:** obrigaria a reescrever a sincronização e a camada do navegador, que já
  funcionam e têm testes, sem resolver nenhum problema atual. O CouchDB também precisa de um
  servidor rodando.
- **Recusado.**

### Trocar o banco do computador por Postgres agora

- **Contra:** exige um servidor de banco em cada computador e não muda nada no celular. Ele
  só faz sentido num servidor central (veja abaixo).
- **Recusado.**

## Consequências

- **Mais pessoas não pesam em banco nenhum nosso:** cada uma tem um banco pequeno, e o custo
  para o finan_rich continua zero.
- **Um programa só:** continua sem dependências e compilando para qualquer sistema.
- **Tipo novo é barato:** basta o `domain.DecodeRecord`, sem migração de tabela.
- **Consultas entre tipos são mais trabalhosas:** consultar dentro do JSON pelo SQL é mais
  lento que colunas próprias. Hoje isso não pesa, porque as regras rodam em Go sobre os
  dados já carregados.
- **Volume:** os dados de uma pessoa, com anos de lançamentos, ficam em alguns MB, bem dentro
  do que o SQLite e o IndexedDB aguentam. Volumes muito maiores não foram medidos.

## Caminho de migração, se um dia houver servidor central

Se o produto passar a ter contas com os dados num servidor nosso (decisão de produto, que
exige um ADR novo), o banco certo é o Postgres (licença PostgreSQL, código aberto). O formato
de hoje já está pronto para isso:

1. Criar no Postgres a mesma tabela `records`, com `data` em `jsonb` e uma coluna a mais para
   a pessoa: `PRIMARY KEY (user_id, type, id)`.
2. O cursor da sincronização (`rev`) continua igual, só que contado por pessoa.
3. Os aparelhos não mudam: continuam com o IndexedDB e o mesmo formato `{type, record}`.
4. Os dados de quem já usa precisam subir uma vez para o servidor. Como o formato é o mesmo,
   isso é cópia, não conversão.

## Quando rever

- se o produto passar a guardar os dados de todos num servidor nosso;
- se um relatório ficar lento de verdade (medido) para uma pessoa;
- se aparecer uma análise que o Go sobre os dados na memória não resolva.
