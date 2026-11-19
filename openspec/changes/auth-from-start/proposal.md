## Why

Os dados da Rich são sensíveis, e o dono pediu autenticação e autorização desde o começo, não
depois. Pela rede, o computador pedia só a senha; trocar a senha ou desligar o acesso não pedia
a senha atual; quem podia usar cada rota estava espalhado (um prefixo `/api/`, uma checagem
dentro do "fechar"); o app do celular só oferecia o bloqueio escondido nos Ajustes; e o Android
copiava o banco do app para o backup do Google mesmo sem cifra.

## What Changes

- Verificação em duas etapas (TOTP, RFC 6238) no login pela rede ao computador: o código de 6
  dígitos do app autenticador depois da senha, 8 códigos de recuperação de uso único, o mesmo
  código não vale duas vezes, e um limite de tentativas do código somando todos os endereços.
  Ligar o acesso pelo celular já oferece o código, marcado por padrão.
- Reautenticação: pela rede, trocar a senha, desligar o acesso e ligar ou desligar o código
  pedem a senha atual e o código. Neste computador não pedem (quem perdeu o celular resolve aqui).
- Autorização numa tabela única (`routes` em `internal/web/server.go`): cada rota declara quem
  pode usá-la (público, a pessoa, a pessoa ou o app pareado, só este computador); o nível zero
  nega tudo, e um teste confere as regras da tabela e a matriz de acesso.
- O primeiro uso do app do celular e do site oferece o bloqueio do app; o link já abre o
  formulário da senha do app.
- Android: o backup do Google só leva os dados quando é cifrado de ponta a ponta (Android 9+);
  no Android 7 e 8, os dados ficam fora do backup.
- O backup `.db` do computador sai sem o segredo do código (o código vem desligado).

## Capabilities

### Modified Capabilities
- security-performance: autenticação em duas etapas, reautenticação, autorização por rota,
  bloqueio oferecido no primeiro uso e backup do Android só cifrado.

## Impact

`internal/auth` (TOTP e códigos de recuperação), `internal/web` (tabela de rotas, `authorize`,
login com código, Ajustes), `internal/store` (backup), telas de login e Ajustes do computador,
primeiro uso e Ajustes do app, manifesto e regras de backup do Android. Sem dependência nova.
Quem já usa o acesso pela rede continua só com a senha até ligar o código.
