## Why

Capturas reais de iPhone mostram símbolos da marca e setas renderizados como emoji,
cartões sobrepostos e uma comparação difícil de ler. Os testes anteriores não cobriam
essas condições; emulação WebKit não equivale à homologação em um iPhone físico.

## What Changes

- Substituir glifos decorativos por vetores reutilizáveis, preservando a identidade.
- Colocar cartões da apresentação em fluxo no celular e tablet.
- Adaptar a comparação para leitura vertical e corrigir foco programático no título.
- Acrescentar verificações de regressão e capturas em WebKit e Chromium.

## Capabilities

### Modified Capabilities
- frontend: legibilidade, símbolos vetoriais e comportamento responsivo público.

## Impact

Landing, entrada, marca compartilhada, CSS e testes E2E. Sem mudanças de login, dados,
dependências ou publicação.
