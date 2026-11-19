## Why

A IA grátis da Rich (intermediário na Vercel, sem chave para a pessoa) já existia, mas vinha
desligada. No app e no site, o Consultor abria com "O consultor precisa de uma IA para
funcionar" e mandava a pessoa aos Ajustes escolher e salvar. No computador, o padrão é o Ollama
local, que quase ninguém tem instalado: o Consultor respondia sem IA e a única saída era os
Ajustes. O dono pediu a IA conversando de primeira, de graça e sem chave.

## What Changes

- App e site: sem nada guardado, vale a IA grátis da Rich. O Consultor conversa de fábrica,
  avisando que usa a IA grátis com apelidos. Desligar nos Ajustes fica guardado (`{ off: true }`);
  desligada, o Consultor oferece "Ligar a IA grátis" num toque.
- Computador: não liga sozinho (a promessa ali é "nada sai daqui sem você escolher"). Sem a IA
  local pronta, o aviso do Consultor traz "Usar a IA grátis da Rich (sem chave)", um clique,
  com o mesmo formulário dos Ajustes.
- O nome da IA na tela vira "IA grátis da Rich" (antes "finan · auto").
- A política de privacidade diz que a IA grátis vem ligada no app e no site, o que vai junto
  (identificador aleatório do aparelho, só para o limite) e como desligar.

## Capabilities

### Modified Capabilities
- advisor: a IA grátis vem ligada no app e no site, e a um clique no computador.

## Impact

`cloud-ai.js`, `pages/advisor.js`, `pages/device.js`, `internal/web/chat.go`,
`site/privacidade.html`, docs. Sem dependência nova, sem mudança no intermediário. Mais gente
usando o grátis consome mais da cota compartilhada: veja os riscos.
