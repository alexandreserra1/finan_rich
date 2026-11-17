## Why

O Mac local não tem SDK iOS e a pessoa não consegue instalar Xcode. O CI já compila
o app, mas falta abrir a interface nativa e obter evidências do simulador remoto.

## What Changes

- Acrescentar um alvo XCUITest ao app existente, sem reescrever a interface.
- Selecionar um iPhone disponível no runner e testar abertura e navegação para Ajustes.
- Guardar logs e anexos do teste em artefatos, sem assinar para distribuição.
- Bloquear deploy da Vercel somente na branch autorizada de validação.

## Capabilities

### New Capabilities
- `ios-ci-validation`: teste nativo e evidências no simulador do CI.

### Modified Capabilities

## Impact

Projeto e scheme iOS, workflow existente, seletor de simulador testável e documentação.
A pessoa autorizou branch e PR, não merge nem deploy. Arquivos locais de outros trabalhos
ficam fora dos commits. A extensão Compartilhar do iOS não faz parte desta etapa.
