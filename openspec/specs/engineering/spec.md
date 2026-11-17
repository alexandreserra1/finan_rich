## Purpose

Garantir uma engenharia verificável e proporcional ao risco para Claude e Codex.

## Requirements

### Requirement: Fluxo rastreável
O agente MUST investigar, especificar, planejar, implementar, testar, revisar, corrigir,
validar e registrar mudanças relevantes. Correções triviais dispensam proposta extensa.

#### Scenario: Mudança relevante
- **WHEN** uma mudança altera contrato, segurança, persistência ou mais de um módulo
- **THEN** existe uma mudança OpenSpec com tarefas e critérios verificáveis antes do código

### Requirement: Autocorreção limitada
O controlador MUST limitar cada problema a cinco pedidos de correção e quinze minutos.
Uma falha idêntica sem alteração MUST interromper a continuação automática sem afirmar sucesso.

#### Scenario: Falha sem progresso
- **WHEN** o mesmo conteúdo e a mesma falha reaparecem
- **THEN** o hook encerra a insistência e informa que a tarefa continua reprovada

### Requirement: Evidência e preservação
O sistema MUST preservar mudanças locais, credenciais e configurações globais. O gate MUST
reprovar comandos com erro. Push, merge, deploy e migrações destrutivas exigem autorização específica.

#### Scenario: Teste falha
- **WHEN** um gate retorna erro ou excede o tempo
- **THEN** nenhuma evidência de aprovação é emitida para aquele conteúdo

### Requirement: Sessões dos dois clientes
Claude e Codex MUST compartilhar instruções, especificações, testes e MCPs locais.

#### Scenario: Cliente confiável inicia sessão
- **WHEN** o cliente carrega os hooks revisados do projeto
- **THEN** SessionStart informa contratos, alterações pendentes e gates sem executar suítes pesadas
