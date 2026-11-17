## ADDED Requirements

### Requirement: Extração local limitada
O Rich MUST ler somente formatos suportados, identificar conteúdo e impor limites de bytes,
expansão ZIP, páginas, pixels e tempo. Documentos MUST permanecer no dispositivo.

#### Scenario: Documento exige OCR
- **WHEN** uma imagem ou página PDF não tem texto aproveitável
- **THEN** o app reconhece localmente, informa progresso e permite cancelar sem gravar dados

### Requirement: Revisão e confirmação
Resultados MUST apresentar tipo, datas, descrição, sentido, valor e categoria reconhecidos.
OCR MUST exigir revisão explícita e não inventar dados ausentes ou ilegíveis.

#### Scenario: Comprovante incompleto
- **WHEN** não é possível identificar data, valor ou direção com segurança
- **THEN** o app recusa a interpretação e orienta como corrigir a origem

### Requirement: Importação idempotente
Reimportar o mesmo documento MUST evitar duplicação. Correspondências entre fontes MUST
ser desmarcadas para revisão sem descartar silenciosamente compras legítimas iguais.

#### Scenario: Notificação e extrato
- **WHEN** uma movimentação semelhante já existe em outra origem
- **THEN** a prévia indica possível duplicação e a pessoa decide antes de confirmar

### Requirement: Dados confirmados
Somente a confirmação MUST persistir lançamentos e recalcular os painéis e insights existentes.

#### Scenario: Cancelar prévia
- **WHEN** a pessoa cancela a leitura ou a revisão
- **THEN** nenhum lançamento é salvo nem enviado para IA
