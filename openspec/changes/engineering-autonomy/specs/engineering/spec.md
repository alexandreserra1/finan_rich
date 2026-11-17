## ADDED Requirements

### Requirement: Engenharia compartilhada
Os dois clientes MUST carregar o fluxo local com gates, contexto, MCPs isolados e correção limitada.

#### Scenario: Prova reversível
- **WHEN** uma fixture temporária falha e é corrigida
- **THEN** a verificação seguinte aprova a fixture e o código de produção não é alterado
