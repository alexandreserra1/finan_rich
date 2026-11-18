## ADDED Requirements

### Requirement: Mapa do código e memória para os dois clientes
O `.mcp.json` e o `.codex/config.toml` SHALL declarar os mesmos MCPs, incluindo o mapa do
código (graphify) e a memória longa (ai-memory), e o mapa SHALL poder ser gravado na memória
sem mandar código para fora do computador.

#### Scenario: Servidor novo só num cliente
- **WHEN** alguém acrescenta um MCP ao `.mcp.json` sem pôr no `.codex/config.toml`
- **THEN** o teste de paridade de `scripts/engineering.test.mjs` falha

#### Scenario: Atualizar o mapa
- **WHEN** a pessoa roda `scripts/graphify-memoria.sh`
- **THEN** o grafo é refeito só com o código, sem IA, e a página `concepts/mapa-do-codigo` do ai-memory é trocada pela nova
