## ADDED Requirements

### Requirement: Verificação em duas etapas no login pela rede
Com a verificação em duas etapas ligada, o login pela rede SHALL pedir a senha e depois o código
de 6 dígitos do app autenticador (ou um código de recuperação de uso único), e SHALL NOT criar
sessão só com a senha nem aceitar o mesmo código duas vezes.

#### Scenario: Senha certa sem o código
- **WHEN** alguém acerta a senha pela rede e erra o código
- **THEN** não há sessão e o registro de acesso mostra "Código de verificação errado"

#### Scenario: Tentativas de vários endereços
- **WHEN** o código é errado 10 vezes em 10 minutos, de endereços diferentes
- **THEN** a tentativa seguinte é recusada, mesmo de outro endereço

### Requirement: Reautenticação para mudar a segurança
Pela rede, trocar a senha, desligar o acesso e ligar ou desligar o código SHALL pedir a senha
atual e, se ligado, o código. Neste computador SHALL NOT pedir.

#### Scenario: Sessão aberta por outra pessoa
- **WHEN** alguém com uma sessão aberta pela rede tenta trocar a senha sem a senha atual
- **THEN** a senha continua a mesma

### Requirement: Autorização por rota
Toda rota do computador SHALL declarar quem pode usá-la, e uma rota sem nível SHALL negar tudo.
O token do app pareado SHALL valer só nas rotas da API.

#### Scenario: Rota nova sem nível
- **WHEN** uma rota entra na tabela sem nível de acesso
- **THEN** o `TestRouteTable` falha e a rota responde 403

### Requirement: Proteção do celular desde o começo
O primeiro uso do app SHALL oferecer o bloqueio do app, e o backup do Google no Android SHALL
levar os dados só quando for cifrado de ponta a ponta.

#### Scenario: Primeiro uso
- **WHEN** a pessoa abre o app pela primeira vez, sem bloqueio
- **THEN** o painel convida a proteger o app, e o link abre o formulário da senha do app
