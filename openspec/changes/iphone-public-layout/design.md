## Context

O Safari escolhe apresentação emoji para caracteres Unicode usados como ilustrações.
Elementos absolutos da composição dependiam de alturas fixas. O foco do leitor de tela
no título acionava o mesmo contorno destinado a controles interativos.

## Decisions

Reutilizar os traços do favicon em um sprite SVG local. Manter cores e fontes existentes.
Abaixo de 900 px, usar fluxo normal para os cartões, sem rotação e sem movimento contínuo.
Abaixo de 600 px, exibir cada critério da tabela com seus dois valores identificados.
Preservar foco programático no título para tecnologia assistiva e foco visível nos controles.

## Validation

Testar quatro larguras e dois temas com dados fictícios, interações, console e capturas.
Inspecionar imagens; não aprovar baselines gerados nesta mesma alteração. O Safari físico,
suas barras dinâmicas e o VoiceOver ainda exigem verificação no aparelho.
