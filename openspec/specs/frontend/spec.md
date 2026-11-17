## Purpose

Preservar a identidade e a qualidade verificável das telas públicas e do app.

## Requirements

### Requirement: Design existente e inspeção real
As telas MUST reutilizar app.css, tokens, fontes locais e componentes existentes. Mudanças
visuais MUST ser inspecionadas em 390, 768, 1440 e 1920 pixels, claro e escuro quando aplicável.

#### Scenario: Revisão visual
- **WHEN** uma tela muda
- **THEN** o agente abre a tela real, interage, examina console e rede e registra screenshots

### Requirement: Acessibilidade e movimento
O frontend MUST preservar teclado, foco, labels, contraste e prefers-reduced-motion.

#### Scenario: Conteúdo essencial
- **WHEN** animações estão reduzidas ou indisponíveis
- **THEN** o conteúdo e as ações essenciais continuam acessíveis

### Requirement: Baselines honestos
Capturas novas MUST ser tratadas como evidência, não como aprovação visual automática.

#### Scenario: Não existe baseline aprovado
- **WHEN** a captura foi gerada pela primeira vez
- **THEN** a revisão informa a necessidade de aprovação antes de adotar comparação pixel a pixel
