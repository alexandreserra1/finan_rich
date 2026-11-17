## ADDED Requirements

### Requirement: Ilustrações consistentes no Safari
O site SHALL desenhar a marca e as setas decorativas com SVG, sem depender de fontes emoji.

#### Scenario: Abrir landing e entrada no iPhone
- **WHEN** a pessoa abre as páginas públicas
- **THEN** a marca e as setas usam os vetores locais e respeitam o tema

### Requirement: Conteúdo público legível no celular
O site SHALL manter o texto dos cartões separado e identificar os valores da comparação.

#### Scenario: Ler em tela estreita
- **WHEN** a largura é de 390 px
- **THEN** os cartões da apresentação não se sobrepõem
- **AND** cada valor da comparação identifica Rich ou outros apps

### Requirement: Foco programático discreto
O site SHALL manter o título acessível sem contorno decorativo após a navegação.

#### Scenario: Navegar por teclado
- **WHEN** a página de entrada recebe foco programático no título
- **THEN** o título não ganha contorno
- **AND** controles alcançados com Tab continuam com foco visível
