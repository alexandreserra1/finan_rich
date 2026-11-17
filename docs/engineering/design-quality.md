# Critérios de qualidade visual

Preserve a identidade existente: cobalto, papel quente, amarelo, fontes locais Manrope e
DM Sans, tokens em app.css e padrões de movimento já implementados. Não instale React,
outro design system ou biblioteca de efeitos para cumprir esta revisão.

Para cada tela alterada, abra o site local com dados fictícios, nos tamanhos 390, 768,
1440 e 1920 pixels. Examine temas claro e escuro quando a tela os suporta. Registre:

- Hierarquia clara, valores legíveis, alinhamento e espaço entre texto, cartão e ações.
- Ausência de overflow, corte, sobreposição e saltos de layout.
- Navegação, formulários, menus, modal e estados vazio, erro, carregando e sucesso.
- Teclado, foco visível, labels, contraste, área de toque e movimento reduzido.
- Console, requests com falha e persistência após navegação ou recarga.
- Peso e tempo de carregamento medidos quando houver mudança relevante; não invente LCP,
  CLS, INP ou cobertura de acessibilidade com base apenas em screenshots.

O E2E existente `site.test.mjs` exercita abas da demonstração, contraste, separação da
legenda do cartão e capturas nas quatro larguras e dois temas. Os demais E2E verificam
o app por dentro. A prova MCP abre a landing real, captura DOM, rede e console e verifica
overflow; ela não substitui todos esses fluxos.

Ainda não há baselines de pixels aprovados por uma pessoa. Não gere e aprove um snapshot
na mesma tarefa para afirmar ausência de regressão. Primeiro entregue capturas para
aprovação; só então adote comparação de pixels com ambiente, fontes e tolerância fixos.
Avaliação estética subjetiva deve ser relatada com evidência e limitação, não como garantia.
