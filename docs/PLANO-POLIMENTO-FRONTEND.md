# Polimento da experiência da Rich

## Objetivo

Manter a identidade da Rich e fazer a pessoa se sentir no controle do dinheiro,
sem esforço para encontrar uma ação ou entender uma troca de tela. Este trabalho
não troca o design por uma landing page nem altera regras financeiras.

## O que preservar

- Papel quente, cobalto, amarelo e os temas claro e escuro.
- DM Sans e Manrope locais, navegação e organização atual das telas.
- Valores e indicadores calculados em Go, sem animações que inventem saldos intermediários.
- Funcionamento offline, sincronização, privacidade e formulários em andamento.

## Etapas

1. Instalar as skills Taste, Transitions e Transitions Polish e conferir suas licenças.
2. Revisar os componentes compartilhados antes de alterar cada tela separadamente.
3. Unificar tempos e curvas das interações, sem efeitos caros ou movimento obrigatório.
4. Anunciar trocas de tela pelo título e manter o teclado dentro de janelas abertas.
5. Conferir navegação, histórico, modais, redução de movimento e tamanhos de tela no navegador.
6. Rodar testes, revisar segurança e publicar somente os arquivos desta tarefa.

## Arquitetura e limites

Os pontos comuns são `static/app.css`, `static/js/ui.js` e o roteador de
`static/js/main.js`. Melhorias nesses pontos atendem Painel, Lançamentos, Contas,
Dívidas, Metas, Análise, Consultor e Ajustes sem duplicar componentes por tela.
As regras, persistência e sincronização não mudam. Redesenhos provocados pela
sincronização não devem mover o foco nem reiniciar a animação de navegação.

Taste foi instalado como ferramenta de trabalho, fora do aplicativo. A versão
atual tem regras voltadas a sites comerciais: heróis em tela inteira, fotos e
momentos decorativos não se aplicam às telas de dados da Rich. Aproveitamos a
disciplina de revisão visual, hierarquia e preservação da marca, não esses layouts.

Transitions oferece skills gratuitas e uma licença própria para as receitas,
com restrição de redistribuição da biblioteca. Ela não corresponde à lista de
licenças abertas permitidas pelo projeto. Por isso não incorporamos a biblioteca
nem copiamos suas receitas para o aplicativo: o código usa CSS e APIs nativas,
mantendo as animações já existentes. O MCP foi cadastrado no Codex, mas exige
autenticação e plano Pro/Business para ser usado. Não contratamos nenhum plano e
não enviamos dados financeiros para esse serviço.

## Critérios de aceite

- Navegação por mouse, teclado e histórico tem foco previsível e título identificável.
- Janelas mantêm Tab e Shift+Tab dentro delas; Escape devolve o foco ao disparador.
- A preferência por menos movimento desliga os efeitos, sem esconder conteúdo.
- A identidade, as funções e os números existentes permanecem.
- O sistema compartilhado não depende de serviço externo para funcionar.
- `make test` e os testes de navegador passam; limitações ficam registradas.

## Fora desta etapa

Não colocamos dados financeiros essenciais em carrosséis nem acrescentamos
autoplay. O endereço separado citado para carrosséis não foi enviado; a mensagem
repetiu `transitions.dev`. Não afirmamos integração com um serviço não identificado.

## Primeira entrega

- Skills `design-taste-frontend`, `transitions-dev` e `transitions-polish` instaladas
  no ambiente local e disponíveis para Codex e os dois perfis do Claude.
- Tokens de movimento compartilhados para botões, janelas, avisos e entrada de tela.
- Foco e título na navegação, inclusive quando o Consultor termina de carregar.
- Perguntas já preparadas continuam com foco no campo de mensagem.
- Janelas mantêm a navegação por teclado, respeitam grupos de rádio e liberam
  listeners e rolagem quando a pessoa volta no histórico.
- Gráficos não ultrapassam a largura do celular antes do carregamento do Chart.js.
- Capturas com dados fictícios, em 390 e 1200 pixels, nos dois temas; o gráfico
  termina de montar antes da captura. O teste de Início em 320 pixels permanece.

O mapa estrutural examinou 603 arquivos e não encontrou ciclos de importação.
Isso não substitui auditoria funcional de cada arquivo; as alterações ficam na
camada de apresentação e não mudam regras financeiras ou sincronização.

As limitações continuam explícitas: autenticação do MCP pago depende da conta da
pessoa; nenhum plano foi contratado. As capturas e testes locais não substituem
a conferência do aplicativo instalado em um Android ou iPhone de verdade.

### Validação desta entrega

- `make test`: passou, incluindo regras em WebAssembly.
- `make e2e`: passou em Chromium, incluindo login e sincronização simulados com Google.
- `npm run webkit` em `e2e`: passou nas suítes locais do motor do Safari.
- A cobertura ampliada das onze rotas SPA, mais Ajustes do servidor, passou
  separadamente em Chromium e WebKit, nos dois temas e nas duas larguras.
- `scripts/check-licenses.sh`: passou; nenhuma dependência foi adicionada ao app.
- Revisão independente apontou um listener de modal que sobrevivia à navegação;
  corrigimos a causa, acrescentamos regressão e a nova revisão aprovou a mudança.

Referências: [Taste Skill](https://www.tasteskill.dev/docs),
[Transitions Skill](https://transitions.dev/skill.html) e
[Transitions MCP](https://transitions.dev/mcp.html).
