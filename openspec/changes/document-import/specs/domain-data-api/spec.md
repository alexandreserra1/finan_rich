## MODIFIED Requirements

### Requirement: Contratos explícitos
Mudanças de APIs MUST definir entrada, saída, autorização, erros e compatibilidade.
A importação MUST aceitar XLSX e texto de documentos sem avaliar fórmulas ou executar conteúdo.

#### Scenario: Integração externa
- **WHEN** um provedor novo é considerado
- **THEN** licença, consentimento, limites, timeout, custos e idempotência são verificados em fontes oficiais

#### Scenario: Documento ilegível
- **WHEN** não há dados suficientes para preparar lançamentos
- **THEN** o contrato devolve erro legível sem persistir nem fabricar transações
