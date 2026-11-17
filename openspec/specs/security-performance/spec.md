## Purpose

Manter privacidade, segurança e desempenho como critérios verificáveis, não promessas.

## Requirements

### Requirement: Dados protegidos
Tokens, senhas e dados financeiros reais MUST ficar fora de logs, screenshots, fixtures e commits.
O agente MUST usar perfis de navegador isolados e não autenticar clientes reais nos testes.

#### Scenario: Diagnóstico de integração
- **WHEN** uma chamada externa falha
- **THEN** o diagnóstico distingue hipótese de causa confirmada e remove credenciais

### Requirement: Validação proporcional
Mudanças de backend MUST executar testes Go; mudanças de interface ou sincronização MUST
executar E2E. Alterações de dependência MUST conferir licença e vulnerabilidades.

#### Scenario: Mudança sem evidências atuais
- **WHEN** o conteúdo muda depois de uma aprovação
- **THEN** a aprovação anterior não satisfaz o gate de conclusão

### Requirement: Orçamentos existentes
O sistema MUST preservar limites de CI, tamanho de APK e medições existentes. Métricas de
desempenho não medidas MUST ser informadas como não verificadas.

#### Scenario: Revisão de desempenho
- **WHEN** há suspeita de regressão
- **THEN** o agente mede antes de otimizar e registra condições e resultados
