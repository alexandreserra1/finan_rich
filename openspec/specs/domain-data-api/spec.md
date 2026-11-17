## Purpose

Preservar fronteiras de domínio, dados e contratos existentes durante a evolução.

## Requirements

### Requirement: Domínio único
Regras financeiras MUST ficar em Go e usar centavos int64 e datas AAAA-MM-DD. Interfaces
HTTP, Wasm e Capacitor MUST reutilizar o domínio sem replicar regras em JavaScript.

#### Scenario: Nova regra financeira
- **WHEN** um cálculo ou validação financeira é criado
- **THEN** há teste Go e execução Wasm quando aplicável antes da ligação com a interface

### Requirement: Persistência segura
Migrações MUST ser testadas com dados fictícios e manter compatibilidade offline-first.
Testes de PostgreSQL MUST usar instância descartável, nunca banco real.

#### Scenario: Mudança de esquema
- **WHEN** uma tabela ou política de acesso muda
- **THEN** a revisão examina perda de dados, isolamento por usuário e compatibilidade de sincronização

### Requirement: Contratos explícitos
Mudanças de APIs MUST definir entrada, saída, autorização, erros e compatibilidade.

#### Scenario: Integração externa
- **WHEN** um provedor novo é considerado
- **THEN** licença, consentimento, limites, timeout, custos e idempotência são verificados em fontes oficiais
