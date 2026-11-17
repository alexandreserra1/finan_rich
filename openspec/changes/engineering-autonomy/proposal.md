## Why

O projeto já tem regras, skills e testes fortes, mas faltam uma especificação verificável,
gates locais compartilhados e inspeção MCP isolada. A configuração deve servir aos dois agentes.

## What Changes

- Instalar OpenSpec e os dois MCPs como ferramentas locais com versões fixas.
- Reusar Superpowers, skills e testes existentes, sem substituir o fluxo atual.
- Conectar hooks de sessão, verificação de saída, pre-commit e CI.
- Limitar autocorreção a cinco tentativas e quinze minutos, sem execução após a sessão.
- Registrar evidências e a viabilidade real do Open Finance gratuito.

## Capabilities

### New Capabilities

- `engineering`: especificação, gates, inspeção e correção limitada.

### Modified Capabilities

Nenhum contrato de negócio será alterado nesta implantação.

## Impact

Somente ferramentas de desenvolvimento, instruções, CI e documentação. Não há nova
dependência no app nem mudança de autenticação, banco real ou integração bancária.
