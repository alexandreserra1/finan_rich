# Receber documentos de outros apps

## Android

1. No banco ou no gerenciador de arquivos, escolha um extrato ou uma imagem compatível.
2. Toque em **Compartilhar** e escolha **Rich**.
3. Desbloqueie a Rich, se necessário, e toque em **Revisar documento recebido**.
4. Confira a prévia e confirme somente os lançamentos corretos.

Receber o arquivo não navega sozinho nem altera o formulário em edição. Se já existir
uma prévia, o app pede confirmação antes de substituí-la. **Dispensar documento recebido**
remove a pendência sem apagar o arquivo no app de origem ou modificar a prévia aberta.

PDF, OFX/QFX, CSV, XLSX, PNG, JPEG e WebP passam pelo mesmo
[importador local](IMPORTACAO-DOCUMENTOS.md). MIME e nome não comprovam o formato:
a leitura confere o conteúdo. Alguns bancos enviam outro MIME; nesse caso, salve o arquivo
e escolha-o pelo seletor da Rich. Texto solto, links e envio de vários arquivos não entram.

## Privacidade e limites

- Um arquivo pendente por vez, por dez minutos. Um novo envio não substitui o anterior;
  a Rich orienta a compartilhar novamente depois.
- A fila nativa guarda somente URI temporário e metadados em memória. Não copia o
  documento para disco, não mantém permissão persistente e não pede acesso amplo aos arquivos.
- Nome e metadados podem ser consultados antes; o conteúdo só é aberto após a ação de revisão.
- Só aceita `content://` de provedor externo. Recusa URI de arquivo, URL remota e provedores
  do próprio UID, evitando acesso indireto aos arquivos privados da Rich.
- O limite real é 8 MiB. Um byte adicional detecta excesso, mesmo se o tamanho informado
  pelo provedor estiver errado. A prévia não é uma gravação financeira.
- Identificadores aleatórios, consumo único, expiração e descarte impedem reutilização e
  resultados antigos. Bloqueio e geração da sessão são conferidos após as esperas assíncronas.
- Apagar os dados nos Ajustes ou recuperar o PIN também limpa a revisão anterior.
- Logs automáticos da ponte Capacitor ficam desativados, inclusive em APK de depuração:
  eles podem imprimir respostas com conteúdo de arquivo. Mensagens do app continuam disponíveis.
- O watchdog responde em até 30 segundos e tenta fechar/cancelar o provedor. Um provedor
  hostil ainda pode prender o worker nativo; a fila e os executores continuam limitados.
  Se isso ocorrer, feche a Rich e use o seletor com uma cópia local do arquivo.

Se o Android encerrar o processo, a pendência se perde: compartilhe de novo. A prévia já
aberta fica na memória até ser descartada, a sessão terminar ou os dados serem apagados.
Os lançamentos confirmados seguem a sincronização que a pessoa configurou; o documento
original não é enviado a OCR, IA ou serviço remoto por esta integração.

## Verificações reproduzíveis

```sh
cd mobile/android
./gradlew :app:testDebugUnitTest :app:assembleDebug :app:assembleDebugAndroidTest
cd ../../e2e
node shared-documents.test.mjs
BROWSER=webkit node shared-documents.test.mjs
```

Os testes do navegador usam a ponte nativa simulada para conferir ação explícita, bloqueio,
cancelamento, confirmação de troca, campos preservados, descarte, limpeza dos dados e
respostas atrasadas tanto na leitura quanto na consulta final de metadados.
Capturas fictícias em quatro larguras e dois temas ficam em `.artifacts/android-share`.

O teste instrumentado `SharedDocumentsInstrumentedTest` usa um provedor exclusivo do APK
de testes. Confere chegada com app fechado e aberto, ausência de leitura antes do clique,
prévia preservada e excesso de tamanho com metadados falsos. Esse provedor não entra no APK
distribuído. Não rode os testes de instalação em um aparelho com dados reais.

No CI, a prova roda após o smoke existente em um emulador descartável. Só nesse ambiente
o APK anterior é desinstalado para trocar a assinatura de release pela de depuração.
O resultado JUnit é conferido, pois `am instrument` sozinho pode sair com código zero
mesmo quando um teste falha. O CI também confere que o console não contém o campo base64.

Fontes do protocolo: [recebimento no Android](https://developer.android.com/training/sharing/receive)
e [guia de plugins Capacitor](https://capacitorjs.com/docs/plugins/android).

## Limitações

Esta etapa é Android. No iPhone e na versão web, a entrada continua pelo seletor de arquivos.
A extensão de compartilhamento do iOS exige alvo e integração próprios; o Xcode completo
não está instalado neste Mac. Não foi declarado suporte nativo iOS sem compilar e validar.

O emulador local usa Android 14/API 34 em ARM64 e dados fictícios. Isso não valida aparelhos
reais, todos os bancos, Android 17/16 KiB local ou a execução futura do workflow no GitHub.

## Evidências locais desta etapa

- 18 testes JUnit novos passaram para fila, expiração, descarte, concorrência e limite do stream.
- A prova instrumentada passou no emulador isolado `rich-share-qa`, incluindo chegada fria
  e quente, conteúdo aberto somente após o clique e erro visível para arquivo acima do limite.
- A captura nativa está em `.artifacts/android-share/emulator-android34.png`.
- O teste de compartilhamento passou em Chromium e WebKit. A suíte completa WebKit,
  agora com 17 scripts, também passou. Formulários, cancelamento, bloqueio, reset e as
  duas janelas de resposta atrasada possuem regressões específicas.
- `make test`, as 23 verificações do controlador/recursos e as licenças foram aprovados.
- Revisões independentes de código e segurança tiveram seus bloqueadores corrigidos;
  não houve bloqueador restante na releitura. A mensagem de erro da importação agora
  permanece depois de redesenhar a tela.
- O APK local ficou com **18.766.227 bytes**, abaixo dos **18.874.368 bytes** permitidos.
  Não foram reduzidos o limite nem as verificações. A assinatura local é de desenvolvimento.
- Com `loggingBehavior: "none"`, a nova prova do logcat não registrou o payload do
  documento no console nativo. Nenhuma informação financeira real foi usada nos testes.

O gate consolidado dos arquivos finais é `node scripts/engineering.mjs check affected`;
o resultado fica em `.artifacts/engineering/verification.json`. Alterações posteriores
invalidam essa evidência. Não houve commit, push, merge ou deploy automático.
