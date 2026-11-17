## Why

A entrada de dados precisa funcionar no site e nos aplicativos sem configurar APIs.
O Rich já importa OFX, CSV e faturas PDF; faltam planilhas XLSX, imagens e PDFs digitalizados,
identificação conservadora de documentos e uma revisão que destaque dúvidas.

## What Changes

- Acrescentar XLSX em Go, sem executar fórmulas ou acessar referências externas.
- Extrair texto de imagens e páginas digitalizadas por OCR local, com limites e cancelamento.
- Reconhecer fatura, extrato e comprovante, exigindo campos financeiros identificáveis.
- Mostrar origem, limites de reconhecimento, revisão e possíveis duplicações entre fontes.
- Recalcular os resumos existentes somente depois da confirmação e gravação.

## Capabilities

### New Capabilities
- `document-import`: documentos financeiros suportados, extração e revisão segura.

### Modified Capabilities
- `domain-data-api`: o contrato existente de importação aceita XLSX e texto de documentos.

## Impact

Importadores Go/Wasm e interface de importação. OCR Apache-2.0 servido pelo próprio app,
sem enviar documentos para terceiros. Não altera autenticação, nuvem, bancos ou design system.
Não promete qualquer documento, leitura infalível ou saldo bancário completo por um comprovante.
