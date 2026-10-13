# App do celular (Capacitor)

O app do finan_rich para o celular usa as **mesmas telas** do app no navegador
(`internal/web/static/js`) e as **mesmas regras em Go** (`finance.wasm`), empacotadas
num app Android pelo [Capacitor](https://capacitorjs.com). A diferença é que, no app,
as telas vêm de dentro do próprio app e o computador fica em outro endereço, combinado
no **pareamento**.

```
 celular (app Capacitor)                         computador (finanrich)
 ┌──────────────────────────────┐  /api/sync     ┌──────────────────────────┐
 │ telas + finance.wasm         │  Authorization │ devices: hash do token   │
 │ (dentro do app: abre sempre) │  Bearer <token>│ records (SQLite)         │
 │ IndexedDB: dados + fila      │ ─────────────▶ │                          │
 │ meta "server": url + token   │ ◀───────────── │ Consultor IA             │
 └──────────────────────────────┘                └──────────────────────────┘
```

## Como usar (para quem vai instalar)

1. Baixe o arquivo `.apk` da [última versão](https://github.com/alexandreserra1/finan_rich/releases/latest)
   no celular Android e abra-o (o Android pede para permitir a instalação de apps
   fora da Play Store).
2. No computador, abra o finan_rich → **Ajustes → Acesso pelo celular** e ligue o acesso
   (com uma senha), se ainda não estiver ligado.
3. Em **Ajustes → App do celular**, clique em **Parear um celular**. Aparece um QR code e
   um código como `ABCD-EFGH` (vale 10 minutos e serve para um aparelho só).
4. No celular, aponte a câmera para o QR code e toque em **Abrir no app**, confira o
   endereço e toque em **Conectar**. Ou abra o app → **Ajustes** e digite o endereço e o
   código.

Pronto: o app abre sem internet, guarda tudo no celular e sincroniza sempre que o
computador estiver ao alcance (mesmo Wi-Fi, finan_rich aberto). Em **Ajustes → App do
celular**, no computador, dá para ver os aparelhos pareados e desconectar qualquer um.
Trocar a senha de acesso ou desligar o acesso pelo celular desconecta todos.

O **Consultor IA** continua rodando no computador: no app, a aba abre a tela do consultor
no navegador do celular (pede a senha de acesso na primeira vez).

## Pareamento e segurança

- O código de pareamento é aleatório (8 caracteres, ~40 bits), vale 10 minutos, é de uso
  único e fica só na memória do servidor. Erros contam no mesmo limite do login
  (5 tentativas em 10 minutos por endereço).
- Ao parear, o app recebe um **token** de 256 bits. O servidor guarda só o hash SHA-256
  (tabela `devices`), como faz com as sessões.
- O app manda `Authorization: Bearer <token>`. Com esse cabeçalho, o servidor aceita
  **apenas** o token, mesmo vindo do próprio computador: assim uma página de outro site
  aberta no computador não consegue usar o acesso local para ler os dados.
- `/api/` responde CORS (`Access-Control-Allow-Origin: *`) porque o app roda em outra
  origem. Não há risco de CSRF: o navegador nunca envia o token (nem o código) sozinho,
  como faria com um cookie.
- O link do QR code abre `/parear` no navegador, que leva a `finanrich://parear?url=…&codigo=…`.
  O app **não conecta sozinho** ao abrir esse link: mostra o endereço e pede confirmação,
  para que um link malicioso não troque o computador com que o app sincroniza.

## Como o código se organiza

| Arquivo | Função |
|---|---|
| `internal/web/pairing.go` | Código de pareamento, `/parear`, `POST /api/parear`, token do aparelho, CORS de `/api/`, lista de aparelhos. |
| `internal/store/devices.go`, `migrations/004_devices.sql` | Aparelhos pareados. |
| `internal/web/export.go` | `ExportApp(dir)`: grava `index.html` (o casco com `data-app="mobile"`) e `static/`. |
| `tools/mobilebuild/main.go` | `go run ./tools/mobilebuild mobile/www` chama o `ExportApp`. |
| `static/js/device.js` | Onde sincronizar: no navegador, o mesmo endereço (cookie); no app, o computador pareado (token). Pareamento, nome do aparelho, leitura do link `finanrich://`. |
| `static/js/pages/device.js` | Tela de Ajustes do app (parear, sincronizar, desconectar, apagar dados). |
| `static/js/pages/advisor.js` | Aba Consultor no app: conversa com a IA do computador (`/api/chat`, com o token). |
| `static/js/native.js` | Leitor de QR code e lembretes de vencimento (plugins do Capacitor). |
| `mobile/package.json`, `capacitor.config.json` | Projeto Capacitor (Android). `CapacitorHttp` faz as chamadas pela rede nativa do celular, sem as travas de `http://` do navegador. Os plugins `Filesystem` e `Share` salvam os arquivos exportados (Excel, PDF, CSV, backup) e abrem o menu de compartilhar do Android. |
| `mobile/android/` | Projeto Android (gerado pelo `cap add android`, versionado). Mudanças nossas: `AndroidManifest.xml` (link `finanrich://` e `usesCleartextTraffic` para o `http://` do Wi-Fi), `app/build.gradle` (versão e assinatura pelo ambiente), ícones e tela de abertura. |
| `mobile/scripts/icons.py` | Gera os ícones e as telas de abertura (Pillow). |
| `e2e/app.test.mjs` | Teste no Chromium com as telas do app servidas em outra origem: pareamento, sync com token, offline, aparelho removido. |

`mobile/www/` e `mobile/node_modules/` são gerados e não ficam no git.

## Desenvolvimento

O app roda em Android 8.0 ou mais novo e iOS 15 ou mais novo.

Precisa de Go, Node 22, Java 21 e o Android SDK (o jeito mais simples é instalar o
[Android Studio](https://developer.android.com/studio)).

```bash
make mobile              # gera mobile/www e sincroniza com o projeto Android
cd mobile && npx cap open android   # abre no Android Studio (rodar no emulador/celular)
make apk                 # ou compila o APK de teste pela linha de comando
```

Depois de mudar qualquer coisa em `internal/web/static` ou nos templates, rode
`make mobile` de novo para o app receber a mudança.

Testar o fluxo do app sem Android: `make e2e` (inclui o `e2e/app.test.mjs`).

## Publicação

O CI compila um **APK de teste** a cada push (artefato `finan_rich-android-teste` na
execução do workflow). A release (`release.yml`) anexa o APK à página da versão:

- **Com a chave do projeto** (recomendado): o APK sai assinado sempre com a mesma chave, e
  cada versão nova instala **por cima** da anterior, mantendo os dados do celular.
- **Sem a chave**: sai um APK de teste (`…_android-teste.apk`), assinado com uma chave
  temporária. Para instalar uma versão nova é preciso desinstalar a anterior (os dados
  voltam do computador ao parear de novo; só se perde o que ainda não tinha sincronizado).

Para criar a chave (uma vez só; **guarde o arquivo e as senhas**, sem eles não dá para
publicar atualizações):

```bash
keytool -genkeypair -v -keystore finan_rich.jks -alias finan_rich \
  -keyalg RSA -keysize 4096 -validity 10000
base64 -w0 finan_rich.jks > finan_rich.jks.b64   # no macOS: base64 -i finan_rich.jks -o finan_rich.jks.b64
```

E cadastrar em **GitHub → Settings → Secrets and variables → Actions**:

| Secret | Valor |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | conteúdo de `finan_rich.jks.b64` |
| `ANDROID_KEYSTORE_PASSWORD` | senha do keystore |
| `ANDROID_KEY_ALIAS` | `finan_rich` |
| `ANDROID_KEY_PASSWORD` | senha da chave |

A versão do app vem da tag (`v1.2.0` → `versionName 1.2.0`, `versionCode 10200`).

## Recursos do celular

- **Ler QR code** (Ajustes do app → *Ler QR code*): `@capacitor/barcode-scanner`. Lê o QR dos
  Ajustes do computador (`http://…/parear?codigo=…`) e conecta.
- **Lembretes de vencimento**: `@capacitor/local-notifications` agenda, para cada dívida com dia
  de vencimento, um aviso mensal 2 dias antes, às 9h. É o próprio Android/iOS que avisa,
  então funciona sem internet. Liga e desliga em Ajustes do app (`js/native.js`).
- **Consultor dentro do app**: `js/pages/advisor.js` usa `GET/POST /api/chat` com o token do
  aparelho; a resposta vem em SSE (no app, chega inteira ao final). Respostas podem ser
  salvas como nota.

## iOS

O projeto está em `mobile/ios` (Capacitor 8 com Swift Package Manager). Já vem com o esquema
`finanrich://` (`CFBundleURLTypes`), `http://` liberado só na rede local
(`NSAllowsLocalNetworking`), as descrições de rede local e câmera, ícone e tela de abertura.
O CI compila para o simulador (sem assinatura) a cada execução.

Para instalar num iPhone: num Mac com Xcode, `make mobile`, depois
`cd mobile && npx cap open ios`, escolha sua conta em *Signing & Capabilities* e rode no
aparelho. Com conta gratuita da Apple o app vale 7 dias; para TestFlight/App Store é
preciso a conta de desenvolvedor (anual).

## Testes no CI

- `e2e/app.test.mjs`: o fluxo do app (pareamento, sync com token, offline, consultor com uma
  IA falsa, salvar como nota) no Chromium.
- Job `android`: compila o APK e o abre num emulador Android 14
  (`mobile/scripts/smoke-android.sh`): confere que a tela inicial aparece (WebView carregou
  as telas e o WebAssembly) e que não houve erro fatal.
- Job `ios`: compila para o simulador.

## Próximos passos

- **Play Store** (taxa única) e **App Store/TestFlight** (conta Apple anual): dependem das contas.
- **Chave de assinatura fixa do APK** (veja *Publicação* acima): para atualizar por cima.
