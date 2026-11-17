## Why

Quem recebe um extrato no banco ou no gerenciador de arquivos deve poder enviá-lo à Rich
sem procurar o arquivo novamente. O importador local já existe; falta receber esse envio.

## What Changes

- Acrescentar recebimento de um arquivo pelo Compartilhar do Android.
- Manter uma pendência temporária em memória, sem ler o conteúdo antes da ação da pessoa.
- Levar o arquivo ao mesmo importador, preservando limites, cancelamento e confirmação.
- Não interromper formulários ou sobrescrever uma prévia sem confirmação.

## Capabilities

### New Capabilities
- `android-document-share`: recebimento nativo limitado e revisão explícita.

### Modified Capabilities

## Impact

Plugin local Capacitor, manifesto Android, ponte JavaScript e tela de importação.
Sem nova dependência, migração, provedor externo ou publicação. iOS continua com o seletor
de arquivos; a extensão de compartilhamento exige alvo próprio e Xcode, indisponível aqui.
