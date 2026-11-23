## Context

O site sincroniza pelo Drive no navegador (`js/gdrive.js`): um arquivo, `finan_rich.json`,
na pasta de dados do app (`appDataFolder`), com todos os registros, inclusive os apagados.
Cada aparelho baixa o arquivo quando a versão muda e aplica o que for mais novo. Depois,
regrava o arquivo inteiro se tiver algo que falta lá. O Google exige o client secret para
trocar o código e renovar o token de um cliente Web, e o secret fica só no intermediário
(`/oauth/token`, `/oauth/refresh`). A página `/oauth` do site é o único endereço de retorno
cadastrado no Google e já repassa `pc.<porta>.*` para `http://127.0.0.1:<porta>/google/retorno`.

## Decisions

- **A sincronização roda no programa, em Go, não na tela.** O computador guarda tudo e
  sincroniza com o celular pareado mesmo com o navegador fechado (como o Open Finance). As
  telas continuam falando só com o programa (`/api/sync`).
- **Cliente do Drive num pacote próprio (`internal/gdrive`).** O `drivesync` também vai para o
  WebAssembly e não deve carregar `net/http`. As mensagens de erro do Drive são as mesmas do
  site, porque a pessoa vê os mesmos motivos (API desligada, permissão, espaço, limite).
- **"Precisa regravar?"**:
  - arquivo baixado: vale o `drivesync.NeedsUpload`, como no site;
  - arquivo igual ao da última rodada: regrava se a revisão do banco passou da última
    sincronizada. A revisão sobe com qualquer alteração vinda de aparelho, inclusive com
    `updatedAt` antigo. A revisão é lida antes dos registros, então nada fica para trás.
- **As duas rotas do login (`GET /google/entrar` e `GET /google/retorno`) valem só neste
  computador.** A volta vem por `127.0.0.1`, e começar o login de outro aparelho mandaria a
  volta para o `127.0.0.1` dele. `/google/entrar` é um link e não um formulário: um POST que
  redireciona para o Google bateria no `form-action 'self'` da CSP. Um pedido falso nessa rota
  só abre a tela de login do Google. A volta só vale com o state guardado na memória do
  programa (15 min, uso único), e o código não serve sem o verificador PKCE.
- **Token de renovação na tabela `settings`** (`google_refresh`), como o segredo da Pluggy, e
  apagado no `BackupTo`. Entrar de novo zera a versão e a revisão guardadas, e a primeira
  rodada mescla tudo.
- **Renovação recusada (`invalid_grant`) desliga a conta neste computador** e a tela pede para
  entrar de novo. Os dados ficam. Outras falhas do intermediário só adiam a rodada.

## Validation

Go, com um Google e um Drive falsos (`httptest`). Os testes conferem:
- o endereço do login, com state `pc.<porta>.*`, PKCE e o retorno do site;
- volta com state errado recusada;
- volta certa grava a conta;
- a primeira rodada traz o que o site gravou e manda o que só o computador tinha;
- sem mudança não regrava;
- alteração de aparelho regrava;
- renovação recusada desliga a conta;
- sair revoga o token e mantém os dados.
Também: as regras das rotas (`TestRouteTable`) e o backup sem o token.

## Risks

- **Duas gravações ao mesmo tempo** (computador e site): uma pode sobrescrever a outra no
  arquivo. A que perdeu regrava na rodada seguinte, porque o dado continua no aparelho dela
  (o mesmo risco do site).
- **Registro que o computador não conhece** (tipo novo de uma versão mais nova do app) faz a
  rodada falhar com a mensagem do registro, como no pareamento. O aviso fica em Ajustes até
  atualizar.
- **Porta não fixa**: a volta precisa da porta numérica. O programa usa a 8080 de fábrica.
