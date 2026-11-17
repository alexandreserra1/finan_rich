# Correções da apresentação no iPhone

## Diagnóstico confirmado

Os símbolos Unicode da marca e das setas dependiam da fonte do sistema. No Safari do
iPhone, viravam emojis coloridos. Cartões posicionados absolutamente cobriam o texto
da conversa. A tabela de três colunas forçava quebras excessivas. O foco programático
do título da entrada herdava o contorno destinado a controles.

## Correções

- Sprite SVG local, com os mesmos traços do favicon, usado no site e dentro do app.
- Cartões em fluxo normal abaixo de 900 px; conteúdo preservado, sem sobreposição.
- Comparação vertical abaixo de 600 px, com valores identificados.
- Título continua recebendo foco acessível, sem contorno; links e botões mantêm o foco.
- Espaçamento considera áreas seguras do aparelho e a tipografia da entrada é fluida.

## Evidências e limites

`e2e/site.test.mjs` testa 360, 390, 600, 768, 1440 e 1920 px, claro e escuro, em Chromium
e WebKit. Verifica símbolos de texto e pseudo-elementos, tabela, texto dos cartões,
contraste da demonstração, overflow, foco, abas e erros de JavaScript.

As capturas locais de inspeção ficam em `.artifacts/iphone-public-layout/`. Os logs de
WebKit e Chromium ficam em `.artifacts/iphone-public-webkit-pass.log` e
`.artifacts/iphone-full-e2e.log`. São evidências locais, não baselines aprovados.

No WebKit para macOS, Option+Tab inclui links na navegação; o teste respeita esse
comportamento do Safari. No Chromium, usa Tab. Isso não valida VoiceOver.

WebKit emulado não é iPhone físico. As barras dinâmicas do Safari, os recortes reais
de tela e o VoiceOver ainda precisam ser conferidos no aparelho após publicação.
O teste local não faz login em contas reais nem comprova a configuração Google de produção.
