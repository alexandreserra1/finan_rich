# Redesign da Rich: clareza com personalidade

## Auditoria e escopo

A Rich é um app de finanças offline-first: Go e WebAssembly calculam os valores;
JavaScript sem framework monta as telas; CSS e fontes locais definem a marca.
Não haverá migração para React, troca de banco, mudança nos cálculos ou instalação
de bibliotecas de animação. O mapa estrutural anterior não encontrou ciclos.

Foram revistos o roteador, os componentes compartilhados, os temas, a landing,
o login e retorno OAuth, o Painel, Movimentos, Contas, Dívidas, Metas, Categorias,
Análise, Consultor, Notas, Importar, Exportar e Ajustes.

Problemas comprovados:

- A demonstração pública usa fundo claro, mas herda a tinta clara do tema escuro.
  Botões claros sofrem o mesmo problema. Os testes anteriores não mediam contraste.
- A anotação do cartão é absoluta e não reserva espaço para o cartão rotacionado.
- O Painel dá peso parecido ao saldo, entradas e saídas; o detalhe mais importante
  deve liderar, com o fluxo de caixa agrupado e menos caixas concorrentes.
- A lista de insights reserva três colunas mesmo quando só há um achado.
- A página pública ainda descreve o código como aberto, mas o LICENSE atual é
  proprietário. Esse texto precisa refletir a licença real.
- A captura do login confirmou "The provided client secret is invalid": a chave
  da Vercel precisa corresponder ao cliente OAuth Web do Google. Isso exige ajuste
  privado da credencial e Redeploy de Production; nenhuma alteração visual pode
  tornar uma chave inválida válida. Separadamente, foram reproduzidas corridas de
  renovação e uma falha de sincronização tratada como sucesso.

## Referências e decisões

As decisões abaixo são uma interpretação para a Rich, não cópias de layouts.

| Fonte | Referência consultada | Princípio aproveitado |
|---|---|---|
| Taste | [Documentação](https://www.tasteskill.dev/docs) e skill instalada 2.5.1 | Revisar composição, preservar a stack e conferir o resultado real. A página de docs ainda se apresenta como 2.1.5; não confundir com a versão instalada. |
| Awwwards | [Letter, SOTD](https://www.awwwards.com/sites/letter) | Um objeto da marca como protagonista, sem deixar a ilustração competir com a leitura. Não usar WebGL ou copiar o cartão. |
| Awwwards | [Reown, SOTD](https://www.awwwards.com/sites/reown) | Sistema visual consistente e poucos elementos para explicar uma interface complexa. |
| Awwwards | [BlueYard, SOTD](https://www.awwwards.com/sites/blueyard) | Um motivo visual reconhecível e entrada intencional. Aproveitar a consistência do motivo, não copiar o cenário WebGL. |
| Awwwards | [Finom, indicado](https://www.awwwards.com/sites/finom) | Tom acessível e produto financeiro apresentado com personalidade. É indicação, não vitória SOTD. |
| One Page Love | [Fynza](https://onepagelove.com/fynza) | Sequência problema, solução e prova; demonstração legível antes do CTA. |
| One Page Love | [Depo Budget App](https://onepagelove.com/depo-budget-app) | Produto de orçamento como referência de contexto, sem copiar cores ou telas. |
| Transitions | [Biblioteca](https://transitions.dev/) | Feedback local e movimento com finalidade; sem efeitos repetitivos sobre valores financeiros. |
| React Bits | [Catálogo](https://reactbits.dev/) | Interatividade pontual, não cenário decorativo permanente. Como não usamos React, nenhum componente ou pacote foi instalado no app. |

Algumas páginas tentadas no Awwwards não responderam. As referências listadas
acima foram abertas; não afirmamos ter inspecionado os exemplos inacessíveis.

## Direção artística

"Um lugar tranquilo para entender o dinheiro e escolher o próximo passo."

Manter cobalto, papel quente, amarelo, DM Sans, Manrope e os detalhes serifados.
No site público, composição editorial e um produto de exemplo legível. Dentro
do app, saldo em primeiro plano e registros organizados como um resumo de conta.
Sem fotos extras, dados inventados, carrossel automático ou nova linguagem visual.

Tokens existentes continuam a fonte das cores e fontes. Superfícies públicas
claras recebem tinta própria, independente do tema da página. O respiro do cartão
vem do fluxo do layout, não de coordenadas que falham em outra largura.

## Plano e validação

1. Reproduzir os dois defeitos públicos em testes antes de corrigir CSS.
2. Corrigir contraste, composição do cartão e clareza do conteúdo público.
3. Reorganizar o resumo financeiro usando os componentes e os valores existentes.
4. Corrigir as corridas de autenticação com regressões, sem relaxar a segurança.
5. Capturar site e telas internas em 390, 768, 1440 e 1920 pixels, nos dois temas.
6. Conferir abas, teclado, estados vazios, console, overflow e movimento reduzido.
7. Rodar testes Go/Wasm e navegador; medir tamanho e desempenho com o script do projeto.
8. Revisar o diff, publicar somente arquivos desta tarefa e registrar limitações.

Sem RUM, não é possível declarar INP real nem Core Web Vitals de toda a população.
Medições locais são evidência de laboratório, com ambiente e limites explícitos.
Login real, se depender de uma conta pessoal, permanece com a pessoa; testes usam
Google e Drive simulados e dados fictícios.

## Implementação

| Arquivo | Alteração |
|---|---|
| `internal/web/static/app.css` | Tinta própria nas superfícies claras, espaço real para a anotação do cartão, navegação alcançável no tablet, resumo financeiro e estado OAuth. |
| `internal/web/static/js/pages/landing.js` | Composição do cartão, descrição correta da licença e troca suave das demonstrações. |
| `internal/web/static/js/pages/dashboard.js` | Saldo como informação principal; entradas e saídas agrupadas, sem alterar os cálculos Go. |
| `internal/web/static/js/ui.js` | Reuso da entrada de conteúdo com Web Animations, tokens existentes e respeito a movimento reduzido. |
| `internal/web/static/js/google.js` | Isolamento das respostas de autenticação por geração; renovação antiga não pode apagar um login novo. |
| `internal/web/static/js/pages/oauth.js` | Erro compreensível e recuperação da sincronização sem repetir a troca do código de login. |
| `e2e/site.test.mjs` | Contraste calculado, overflow, posição da anotação e screenshots das demonstrações em quatro larguras e dois temas. |
| `e2e/simple.test.mjs` | Mesma matriz de larguras e temas nas telas internas, mantendo testes de teclado e movimento reduzido. |
| `e2e/app.test.mjs` | Espera pelo saldo principal do novo resumo após concluir o primeiro uso. |
| `e2e/session.test.mjs`, `e2e/google.test.mjs` | Regressões de concorrência, falha de configuração e recuperação do Drive. |
| `e2e/performance.mjs` | Observadores nativos de LCP, CLS por janela e Event Timing para uma interação específica. |

Nenhuma dependência de execução foi adicionada. Taste foi usado também para captura
real no Chrome. Os princípios de Transitions foram aplicados com código nativo
próprio; receitas com licença fora da lista permitida não foram incorporadas.
O MCP de Transitions exige autenticação externa e não foi usado nesta validação.

## Medição de laboratório

Ambiente: macOS arm64, Apple M1 Pro, Chromium 156.0.8078.4, sem limitação artificial
de CPU ou rede. A base local fictícia tem 5 mil lançamentos. Foram feitas cinco
aberturas frias e cinco recargas em cada versão.

| Medida | Antes | Depois |
|---|---:|---:|
| Mediana até o Painel, abertura fria | 1.257 ms | 1.323 ms |
| Mediana até o Painel, recarga | 958 ms | 872 ms |
| Maior CLS observado | 0,0297 | 0,0297 |
| Maior duração registrada ao abrir o formulário | 336 ms | 320 ms |

LCP depois: de 840 a 2.140 ms nas dez amostras. Os tempos variam numa máquina
compartilhada; a abertura fria ficou um pouco mais lenta. Não há evidência para
prometer ganho geral de velocidade. O Event Timing acima mede uma interação
sintética, não INP de campo. O caso de 5 mil lançamentos ainda merece otimização
própria da abertura da lista; não foi incluída uma refatoração de dados neste redesign.

Os 61 arquivos do precache somam 10.074.328 bytes sem compressão e aproximadamente
3.145.204 bytes com gzip calculado localmente. A soma gzip dos seis arquivos de
execução alterados cresceu 1.290 bytes frente ao commit anterior. Isso não é o
download HTTP medido: cabeçalhos, compressão e cache variam no servidor.

## Evidências de validação

- `make test`: passou, incluindo formatação Go, vet, testes Go e regras em Wasm.
- `make e2e`: passou nas 16 suítes Chromium, incluindo app offline e Google simulado.
- `scripts/check-licenses.sh`: passou; nenhuma biblioteca nova no app.
- `cd e2e && PORT_OFFSET=600 npm run webkit`: passou nas 15 suítes do núcleo.
- Os 58 casos de sessão também passaram no WebKit após a última correção de retomada.
- A matriz pública cobre quatro larguras, dois temas e três abas: 24 combinações,
  com contraste calculado, ausência de overflow e distância mínima de 20 pixels
  entre a anotação e o cartão. Foram salvas capturas completas e recortes.
- As telas internas foram abertas nas mesmas quatro larguras e dois temas,
  incluindo navegação por teclado, modais, console e movimento reduzido.
- A inspeção real encontrou uma falha adicional em 768 pixels: abas centralizadas
  escondiam links no começo da faixa rolável. O alinhamento inicial foi corrigido,
  sem forçar cliques nos testes.
- A revisão independente aprovou os diffs visual e de autenticação após correção
  de dois bloqueios encontrados na revisão. Não houve relaxamento de PKCE ou TTL.
- Testes direcionados: 58 casos de sessão e 151 verificações do Google simulado
  passaram. Incluem renovação atrasada, logout concorrente, retomada após recarregar
  e recuperação automática do Drive sem clique. O erro `invalid_client` foi aberto
  em 390 e 1440 pixels, nos dois temas, sem overflow.

As capturas, logs e bancos fictícios ficam fora do Git. A suíte Google usa serviços
simulados; nenhuma chave pessoal foi necessária ou publicada.

## Configuração ainda necessária

O erro capturado foi `The provided client secret is invalid.`. Na Vercel, substituir
`GOOGLE_CLIENT_SECRET` pela chave do mesmo cliente OAuth Web de `GOOGLE_CLIENT_ID`,
no ambiente Production, e fazer Redeploy da `main` em Production. Não colar a chave
em conversas, documentos ou commits. As correções de concorrência não substituem
esse ajuste privado.

Referência: [credenciais OAuth Web do Google](https://developers.google.com/identity/protocols/oauth2/web-server#creatingcred).
Se a chave não foi guardada quando criada, gerar outra no mesmo cliente, em vez
de copiar o ID do cliente para o campo de segredo.

Não foram verificados login Google real, Android físico, Safari de aparelho real
nem métricas de usuários em produção. WebKit automatizado não substitui esses testes.
