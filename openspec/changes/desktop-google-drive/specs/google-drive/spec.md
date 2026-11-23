## ADDED Requirements

### Requirement: Entrar com Google no computador
O programa do computador SHALL oferecer "Entrar com Google" nos Ajustes, com o login no
navegador do próprio computador. A volta SHALL valer só com o state que o programa gerou, uma
vez, por até 15 minutos, e o código SHALL ser trocado com o verificador PKCE pelo intermediário.

#### Scenario: Login completo
- **WHEN** a pessoa toca em "Entrar com Google" e aceita no Google
- **THEN** os Ajustes mostram a conta ligada, e o token de renovação fica só neste computador

#### Scenario: Volta forjada
- **WHEN** chega uma volta com um state que o programa não gerou
- **THEN** o programa recusa e nada é gravado

### Requirement: Sincronização pelo Drive em segundo plano
Com a conta ligada, o programa SHALL sincronizar com o arquivo da pasta de dados do app no
Drive enquanto estiver aberto, com a mesma regra do site: vence o `updatedAt` maior. Ele SHALL
regravar o arquivo só quando tiver algo que falta lá.

#### Scenario: Dados do site chegam ao computador
- **WHEN** o arquivo no Drive tem um lançamento feito no site
- **THEN** o lançamento aparece no computador e vai para o celular pareado

#### Scenario: Nada mudou
- **WHEN** nem o arquivo nem o banco mudaram desde a última rodada
- **THEN** o programa não regrava o arquivo

### Requirement: Sair e segredos
Sair SHALL revogar o acesso no Google e apagar o token deste computador, mantendo os dados. O
backup do banco SHALL NOT levar o token de renovação.

#### Scenario: Sair da conta
- **WHEN** a pessoa toca em "Sair da conta Google"
- **THEN** o token é revogado e apagado, e os lançamentos continuam no computador e no Drive
