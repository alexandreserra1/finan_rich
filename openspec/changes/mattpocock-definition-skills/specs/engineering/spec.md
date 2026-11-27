## ADDED Requirements

### Requirement: Skills de definição compartilhadas
O projeto SHALL disponibilizar to-spec e grill-with-docs com suas dependências nos dois clientes.

#### Scenario: Descobrir as skills após carregar o projeto
- **WHEN** Claude ou Codex consulta as skills locais
- **THEN** os quatro nomes resolvem arquivos presentes com a mesma origem e conteúdo
- **AND** grill-with-docs pode carregar grilling e domain-modeling

### Requirement: Definição sem publicar ou duplicar contratos
O fluxo SHALL carregar as skills desde o início, permitir invocação automática conforme
o contexto e preservar o OpenSpec como fonte única, sem autorizar publicação externa.

#### Scenario: Iniciar uma sessão
- **WHEN** o hook apresenta o fluxo de definição
- **THEN** orienta carregar as duas skills e aplicar entrevista ou síntese conforme o contexto, sem comando manual
- **AND** não cria issues externas nem contratos paralelos

#### Scenario: Executar uma correção trivial
- **WHEN** a tarefa não tem decisões relevantes abertas nem precisa de especificação nova
- **THEN** o fluxo curto é preservado, sem iniciar uma entrevista desnecessária
