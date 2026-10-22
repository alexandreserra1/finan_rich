# Como o app chega ao celular (sem pagar loja)

| Onde | Custo | Atualiza sozinho | Situação |
|---|---|---|---|
| APK no [GitHub Releases](https://github.com/alexandreserra1/finan_rich/releases) | grátis | não (baixa de novo) | ✅ já funciona |
| [IzzyOnDroid](https://apt.izzysoft.de/fdroid/) (no app F-Droid, Droid-ify, Neo Store) | grátis | sim | ⏳ falta a chave fixa e o pedido |
| [F-Droid](https://f-droid.org) (repositório principal) | grátis | sim | ⏳ falta a chave fixa e o pedido |
| iPhone pelo Safari (Tela de Início) | grátis | sim (é a versão do computador) | ✅ já funciona |
| Play Store | US$ 25 uma vez | sim | depois |
| App Store / TestFlight | US$ 99 por ano | sim | depois |

## Antes de tudo: a chave de assinatura fixa

As lojas abertas só aceitam o APK assinado sempre com a mesma chave (sem ela, uma versão
nova não instala por cima da antiga). Crie e cadastre nos secrets do GitHub uma vez só:
veja *Publicação* em [MOBILE.md](MOBILE.md). A partir daí, cada release sai como
`finan_rich_X.Y.Z_android.apk` (sem o `-teste`).

A versão do app fica em `mobile/android/app/version.properties`. **Publique sempre pelo
botão** (Actions → Release → Run workflow): ele atualiza esse arquivo, faz o commit na
main e cria a tag. As lojas leem a versão desse arquivo na tag.

## IzzyOnDroid

Ele pega o APK direto do GitHub Releases, então cada release nova chega aos usuários em
cerca de um dia. Os textos e as capturas de tela vêm de `fastlane/metadata/android`
(pt-BR e en-US).

1. Publique uma release com a chave fixa (passo acima).
2. Abra um pedido em <https://gitlab.com/IzzyOnDroid/repo/-/issues> (conta grátis no
   GitLab), com o modelo de *inclusion request*. Texto sugerido:

   > **App:** finan_rich (`io.github.alexandreserra1.finanrich`)
   > **Source:** https://github.com/alexandreserra1/finan_rich (MIT)
   > **APK:** GitHub Releases, `finan_rich_<version>_android.apk`
   > **Description:** Offline-first personal finance app for Brazil (Portuguese UI):
   > expenses, debts, goals, statement import (OFX/CSV/PDF) and a local AI advisor. Data
   > stays on the phone and syncs only with the user's own computer (LAN or Tailscale).
   > **Libraries:** Capacitor (MIT), AndroidX (Apache-2.0), pdf.js and jsQR
   > (Apache-2.0), capacitor-native-biometric (MPL-2.0, AndroidX Biometric), wllama/llama.cpp (MIT; the
   > optional on-device AI model is downloaded on demand by the user); no Google Play Services, no trackers, no ads. License check runs in CI
   > (`scripts/check-licenses.sh`).
   > **Metadata:** Fastlane structure in `fastlane/metadata/android` (pt-BR, en-US).
   > **Permissions:** INTERNET (sync with the user's computer), CAMERA (scan the pairing
   > QR code), and the notification permissions added by @capacitor/local-notifications
   > (offline debt due-date reminders).

3. Responda às perguntas que aparecerem no pedido (costuma levar de dias a poucas semanas).

## F-Droid

O F-Droid compila o app a partir do código, no servidor deles, e assina com a chave
deles. A receita está em [`fdroid/io.github.alexandreserra1.finanrich.yml`](../fdroid/io.github.alexandreserra1.finanrich.yml)
e o workflow **F-Droid** (`.github/workflows/fdroid.yml`) roda os mesmos comandos a cada
mudança, para a gente saber antes deles se a receita quebrou.

1. Publique a versão que está na receita (`versionName`/`commit`); se for outra, ajuste os
   quatro campos de versão da receita.
2. Faça um fork de <https://gitlab.com/fdroid/fdroiddata>, copie a receita para
   `metadata/io.github.alexandreserra1.finanrich.yml` e abra o merge request (o modelo
   deles pede para confirmar que o app é livre e sem rastreamento).
3. Depois de aceito, o F-Droid confere as tags novas (`UpdateCheckMode: Tags`) e publica
   as versões seguintes sozinho.

Observação: como o F-Droid assina com a chave deles, quem instalou pelo F-Droid não
atualiza com o APK do GitHub (e vice-versa). Para unificar, dá para tornar o build
reproduzível e o F-Droid passa a publicar o nosso APK (campo `Binaries:` da receita).

## Computador: Homebrew, Scoop e winget

Todos gratuitos e de código aberto. Este repositório serve de *tap* do Homebrew
(`Casks/`) e de *bucket* do Scoop (`bucket/`): o GoReleaser atualiza os dois a cada release
e o workflow **Pacotes** instala, abre e desinstala nos três sistemas para conferir.

**winget** (o gerenciador que já vem no Windows 10/11): os pacotes ficam no repositório
`microsoft/winget-pkgs`, e cada versão nova vira um pedido (pull request) revisado pela
Microsoft, de graça. Para o GoReleaser abrir esse pedido sozinho:

1. Faça um fork de <https://github.com/microsoft/winget-pkgs> na sua conta.
2. Crie um token em GitHub → Settings → Developer settings → *Fine-grained tokens*, com
   acesso ao fork (`Contents` e `Pull requests`: leitura e escrita).
3. Cadastre o token como secret `WINGET_GITHUB_TOKEN` neste repositório.

Na release seguinte, o pedido é aberto; depois de aprovado, funciona
`winget install alexandreserra1.finan_rich`. Sem o secret, essa etapa é pulada.

## iPhone sem App Store

A Apple não deixa instalar apps de fora da App Store sem a conta paga. O caminho grátis é
usar o finan_rich pelo **Safari**, que vira um ícone na tela de início, com a cara de um
app.

**Para os clientes (recomendado):**

1. No computador, em Ajustes, ligue o acesso pela rede (com senha).
2. No iPhone, leia com a câmera o QR code dos Ajustes (ou abra o endereço no Safari).
   A página de pareamento reconhece o iPhone e mostra os passos.
3. Entre com a senha e toque em **Compartilhar → Adicionar à Tela de Início**.

Pelo Wi-Fi de casa (endereço `http://`), o app funciona enquanto o computador está
ligado e na mesma rede. Para **abrir sem internet e sincronizar de qualquer lugar**, o
iPhone precisa de um endereço `https://`, e o jeito grátis é o **Tailscale** (no
computador: Ajustes → *Usar no celular sem conexão e fora de casa*).

O que muda em relação ao app nativo: os lembretes de vencimento (notificações agendadas)
não existem no Safari; o resto (lançamentos, importação, metas, dívidas, notas,
exportação e consultor) é igual.

**Para você testar o app nativo no seu iPhone, sem pagar:**

- **Xcode com Apple ID grátis** (precisa de um Mac): `make mobile`, depois
  `cd mobile && npx cap open ios`, escolha sua conta em *Signing & Capabilities* e rode
  no iPhone ligado no cabo. O app vale 7 dias; depois é só rodar de novo.
- **SideStore / AltStore** (código aberto, Apple ID grátis): instalam um `.ipa` e renovam
  a assinatura a cada 7 dias (no máximo 3 apps). Serve para alguns testadores, mas é
  trabalhoso demais para clientes.

**TestFlight** (convite por link, até 10 mil testadores) e App Store exigem a conta de
desenvolvedor (US$ 99 por ano). O projeto iOS já compila no CI, então quando a conta
existir falta só configurar a assinatura.
