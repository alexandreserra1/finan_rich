## Context

Swift local funciona para macOS, mas não há SDK iPhoneSimulator. O job macos-latest
do GitHub já tem Xcode e compila o mesmo projeto Capacitor, sem credenciais da Apple.

## Goals / Non-Goals

Lançar o app em um iPhone simulado, conferir conteúdo visível e navegar para Ajustes.
Não distribuir IPA, entrar em conta real, ativar release ou afirmar teste em aparelho físico.

## Decisions

Alvo XCUITest isolado e scheme App compartilhado. Um teste espera o WebView, confere
o aviso de aparelho novo, toca em Conectar agora e confere o botão de uso local.
Anexos ficam no xcresult. O seletor considera somente iPhones disponíveis em runtimes iOS
e aceita apenas UUID válido; ausência de dispositivo reprova, sem pular o teste.
O workflow conserva timeout e código de saída do xcodebuild, e publica somente evidências.
vercel.json desabilita auto-deploy apenas em codex/validacao-ios-sem-xcode.

## Risks / Trade-offs

Sintaxe do projeto, XML e seleção podem ser conferidas localmente. Compilação e interação
nativa dependem da execução remota; resultados pendentes nunca contam como sucesso.
Acessibilidade do WebView pode variar entre SDKs e precisa ser verificada no runner.
