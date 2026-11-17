## ADDED Requirements

### Requirement: Recebimento restrito
O app MUST receber somente um arquivo por envio, por content URI externo, sem permissões
amplas, cópia persistente, leitura de URLs remotas ou acesso aos provedores do próprio UID.

#### Scenario: Envio abusivo
- **WHEN** o envio contém vários arquivos, URI inadequado ou conteúdo acima de 8 MiB
- **THEN** o app recusa sem persistir ou importar dados

### Requirement: Pendência temporária
A pendência MUST expirar em dez minutos, não sobrescrever outro envio e exigir seu
identificador para leitura ou descarte. O conteúdo MUST ser entregue no máximo uma vez.

#### Scenario: Novo envio durante revisão
- **WHEN** já existe um arquivo pendente ou em leitura
- **THEN** o app preserva o anterior e orienta a reenviar o novo depois

### Requirement: Revisão voluntária
O recebimento MUST mostrar um aviso sem navegar, gravar lançamentos ou trocar uma prévia.
O conteúdo MUST ser lido somente após ação da pessoa e sem bloqueio ativo do app.

#### Scenario: Prévia já aberta
- **WHEN** a pessoa pede para revisar o documento recebido
- **THEN** o app pede confirmação antes de descartar uma prévia existente

#### Scenario: App bloqueado
- **WHEN** um documento chega enquanto o bloqueio está ativo
- **THEN** nenhum conteúdo é lido até a pessoa desbloquear e pedir a revisão
