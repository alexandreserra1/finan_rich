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
- **Corrigido em `55f29af`:** `internal/web/static/js/main.js` chama `sync.flushBeforeLeaving()`
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
| Relógios diferentes, edição simultânea offline | feito com o computador; falta no Drive | ver 1.3 |

### 1.3 Relógio do aparelho em edições simultâneas · feito (com o computador)

- **O problema, no tamanho certo:** o `updatedAt` vem do relógio de quem editou, mas o carimbo é
  `máximo(relógio, versão anterior + 1)`: com a versão mais nova recebida, a edição sempre vence.
  O relógio só decidia quando dois aparelhos sem conexão mudavam o **mesmo** registro antes de
  sincronizar (vencia o relógio maior, não quem editou por último).
- **Corrigido:** a resposta do `/api/sync` traz a hora do computador (`now`); o aparelho guarda a
  diferença (descontando metade da ida e volta; até 2 s é ruído) e soma no carimbo de toda
  edição, com `sync.stamp` (`main.js`, `notes-save.js`, restaurar backup). Testes em
  `internal/web/web_test.go` e `e2e/offline.test.mjs` (aparelho com o relógio 1 hora atrasado).
- **Falta:** o mesmo no Drive (site): lá não há resposta com a hora do servidor ainda; dá para
  usar o `modifiedTime` que o Drive devolve ao gravar o arquivo.

## 2. Segurança

### 2.1 Site só com login · feito

A auditoria achou a contradição entre "usar sem conta" e "precisa de login". Decidido: o site
só funciona com Entrar com Google (`2398954`): sem login, nenhuma tela do app aparece, nem
digitando o endereço; sessão vence após 30 minutos parado; sair salva no Drive e apaga o
navegador. README corrigido em `c9ecd2b`; regras em [GOOGLE.md](GOOGLE.md). O computador e o
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
| Sair com alterações pendentes | feito: `offline.test` (desde `55f29af`) e `google.test` (Drive fora do ar) |
| Trocar de aparelho e recuperar os dados | feito: `google.test` (segundo navegador pelo Drive) e pareamento em `offline.test` |
| Exportar e restaurar o backup | feito: `export.test` (apaga tudo e restaura) |
| IA, Drive ou computador fora do ar | feito: `app.test` (IA no limite, internet caindo, resposta sem IA), `google.test`, `offline.test` |
| Relógio errado com edição simultânea offline | feito: `offline.test` (relógio 1 hora atrasado) |

## 4. Produto e telas

| Ponto da auditoria | Situação |
|---|---|
| Painel como centro de decisão (situação, atenção, ação, evolução) | feito no redesign: avisos assinados pelo time, "O que o time viu", números do mês no topo |
| Lançamento rápido, com desfazer | feito: janela curta, Desfazer depois de apagar |
| Dívidas com cenários (mínimo, valor a mais, ordem) | feito: tela Dívidas e o Bento (`simular_dividas`) |
| Consultor que separa conta do Go e texto da IA | feito em parte: contas prontas do Go (`prefetch`) e resposta sem IA; **em andamento:** cartão "Os números do app" na hora, antes da IA |
| Ajustes mostrando onde ficam os dados e o que vai para a IA | **em andamento:** redesign de Ajustes com a escolha "IA do finan_rich" ou "minha chave" |
| Estados vazios, carregamento, erro padronizados | feito nas telas redesenhadas; faltam Ajustes, Consultor e Boas-vindas (em andamento) |
| Acessibilidade e contraste | feito: teste de contraste dos temas; gráficos com tabela |

## 5. Medir antes de otimizar · a fazer (média)

A auditoria não mediu desempenho. Passos: medir o tempo até o Painel aparecer (com o
`finance.wasm`), com 5 mil lançamentos, e o tamanho do que o site baixa; só então decidir se
vale otimizar. Já medido: consultor no computador local, de 13 a 35 s por pergunta (ver
[IA-GRATIS.md](IA-GRATIS.md)).

## Ordem de execução

1. **1.3** O acerto do relógio também no Drive (site).
2. Terminar as telas em andamento (Consultor, Ajustes, Boas-vindas) e conferir em navegador real.
3. **5** Medir o desempenho e só então otimizar.
4. Conferir o Sair do acesso pela rede num aparelho de verdade (1.1).
