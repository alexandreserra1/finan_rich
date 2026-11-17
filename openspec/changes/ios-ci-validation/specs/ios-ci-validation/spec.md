## ADDED Requirements

### Requirement: Validação nativa remota
O CI MUST compilar e executar o alvo UI no simulador iPhone sem credenciais ou assinatura
de distribuição. O teste MUST exigir conteúdo visível e navegação, não somente ausência de crash.

#### Scenario: Aparelho novo
- **WHEN** o app abre no simulador limpo
- **THEN** o teste confere o aviso inicial, navega para Ajustes e guarda capturas como anexos

### Requirement: Falha explícita
O seletor MUST ignorar dispositivos indisponíveis ou de outra plataforma. Falta de iPhone,
erro de build ou falha de teste MUST reprovar o job sem diminuir os critérios.

#### Scenario: Nenhum iPhone disponível
- **WHEN** o runner não oferece um simulador iPhone elegível
- **THEN** a validação falha com diagnóstico, sem declarar homologação

### Requirement: Sem publicação
A branch autorizada MUST impedir deploy automático da Vercel. O fluxo MUST preservar
produção, mudanças locais de terceiros e a exigência de autorização para merge.

#### Scenario: Abrir PR
- **WHEN** a branch de validação é publicada
- **THEN** somente verificações são solicitadas, sem merge ou release
