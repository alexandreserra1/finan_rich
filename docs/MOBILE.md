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
  (tabela `devices`), como faz com as sessões. No app, o token fica no **cofre do sistema**
  (Keystore/Keychain, chave `server-token`), não no banco do app; um token antigo em texto
  passa para o cofre na próxima abertura (`js/device.js`).
- Aparelho que passa **180 dias sem sincronizar** perde o acesso e precisa parear de novo
  (`store.DeviceByToken`): um token esquecido num celular vendido ou perdido não vale para
  sempre. Remover o aparelho em Ajustes do computador corta na hora.
- O app manda `Authorization: Bearer <token>`. Com esse cabeçalho, o servidor aceita
  **apenas** o token, mesmo vindo do próprio computador: assim uma página de outro site
  aberta no computador não consegue usar o acesso local para ler os dados.
- `/api/` responde CORS (`Access-Control-Allow-Origin: *`) porque o app roda em outra
  origem. Não há risco de CSRF: o navegador nunca envia o token (nem o código) sozinho,
  como faria com um cookie.
- O link do QR code abre `/parear` no navegador, que leva a `finanrich://parear?url=…&codigo=…`.
  O app **não conecta sozinho** ao abrir esse link: mostra o endereço e pede confirmação,
  para que um link malicioso não troque o computador com que o app sincroniza.
- **Bloqueio do app** (Ajustes do app → *Bloqueio do app*, em `js/lock.js`): pede a digital
  ou o rosto (mesmo plugin do cofre) ou a senha do app (4 a 6 números) ao abrir e ao voltar
  depois de 1 minuto fora. Da senha só fica o hash (PBKDF2-SHA256, 600 mil rodadas, com
  sal). A cada 5 senhas erradas, o app espera 30 segundos, mesmo se for fechado. Quem
  esquece a senha só desbloqueia apagando os dados do celular (os do computador continuam
  lá). O bloqueio protege de quem pega o celular desbloqueado; ele não cifra os dados.
- **CSP**: as telas não rodam script embutido na página. No computador, a política vai no
  cabeçalho `Content-Security-Policy` (`browserCSP` em `server.go`). No app, vai numa tag
  `<meta>` do `index.html` (`appCSP` em `export.go`), com a rede liberada para falar com o
  computador pareado e com as IAs na nuvem.

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

O app roda em Android 7.0 ou mais novo e iOS 15 ou mais novo.

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
sh scripts/android-key.sh
```

O script usa só o OpenSSL (já vem no macOS e no Linux; no Windows, rode pelo Git Bash).
Ele cria a chave na pasta `finan_rich-chave/` (fora do git) e, se o
[GitHub CLI](https://cli.github.com) (`gh`) estiver instalado e logado, já cadastra os
secrets. Sem o `gh`, ele mostra o que colar em **GitHub → Settings → Secrets and variables
→ Actions**:

| Secret | Valor |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | conteúdo de `finan_rich-chave/keystore-base64.txt` |
| `ANDROID_KEYSTORE_PASSWORD` | em `finan_rich-chave/senhas.txt` |
| `ANDROID_KEY_ALIAS` | `finan_rich` |
| `ANDROID_KEY_PASSWORD` | em `finan_rich-chave/senhas.txt` |

A versão do app fica em `mobile/android/app/version.properties`: o workflow de release (Actions → Release → Run workflow) atualiza o arquivo, faz o commit na main e cria a tag (`v1.6.0` → `versionName 1.6.0`, `versionCode 10600`). O F-Droid e o IzzyOnDroid leem a versão desse arquivo.

## Recursos do celular

- **Consultor sem computador** (Ajustes do app → *Consultor sem computador*): o celular chama
  uma IA na nuvem com plano grátis (Groq, Google Gemini, OpenRouter) direto, sem passar pelo
  computador. Cada pessoa usa a própria chave grátis: a tela mostra o passo a passo de cada
  provedor (`KeySteps` em `llm/presets.go`) e, ao salvar, testa a chave com uma pergunta
  mínima, sem dados (`askapi.Check`). O app não traz chave embutida: ela sairia do APK em
  minutos e todo mundo dividiria (e esgotaria) a mesma cota. Para quem não quer criar conta,
  há a opção "finan_rich grátis (sem chave)", que passa por um intermediário na Vercel com
  limite por aparelho ([IA-GRATIS.md](IA-GRATIS.md)); ela só aparece depois de publicado.
  Os provedores tiram modelos do ar com poucas semanas de aviso. Por isso, se o modelo salvo
  deixar de existir, o app pergunta ao provedor (com a chave da pessoa) quais modelos ele tem,
  troca pelo melhor da lista `llm.Preferred` e tenta de novo; os desligados conhecidos já vão
  direto para o substituto (`llm.retired`).
  Privacidade:
  - **anonimizado**: antes de sair, nomes de lojas e pessoas, categorias criadas pela pessoa,
    dívidas, credores e metas viram apelidos ("Loja AB", "Categoria 1", "Dívida 1") e as
    notas ficam de fora; a pergunta e o histórico também (inclusive a descrição inteira, com
    CPF, se ela voltou numa resposta). As categorias padrão ("Mercado", "Aluguel") ficam: são
    genéricas. Os apelidos de loja são letras do mesmo tamanho, para a lista por
    estabelecimento e a busca não juntarem lojas diferentes. A IA vê valores, datas e
    categorias. Na volta, o app troca os apelidos pelos nomes, e os cartões
    gravam sempre o nome verdadeiro (pelo ID do registro). Regras em `advisor/anon.go`.
  - **chave no cofre do sistema**: no app, a chave da IA fica no Android Keystore / iOS
    Keychain, pelo plugin [capacitor-native-biometric](https://github.com/Cap-go/capacitor-native-biometric)
    (MPL-2.0), em `js/cloud-ai.js`. Quem copiar a pasta do app não leva a chave. Uma chave
    salva por uma versão antiga (cifrada com AES-GCM no banco do app) passa para o cofre na
    primeira leitura. Fora do app, continua cifrada com AES-GCM. Ela nunca vai para o backup
    do finan_rich nem para a sincronização. A conversa fica só no celular.
  - **conexão cifrada** (HTTPS). A IA precisa ler os números para responder: não existe IA na
    nuvem que trabalhe com dados cifrados. No plano grátis do Gemini, o Google pode usar o que
    recebe para melhorar os produtos dele.
  - O consultor, as ferramentas e a anonimização são os mesmos do computador, compilados para
    um WebAssembly à parte (`cmd/finanadvisor` → `advisor.wasm`, ~3 MB no APK), que só o app
    leva e só carrega ao abrir o Consultor. O Claude (Anthropic) fica só no computador.

- **IA dentro do celular** (Ajustes do app → *IA dentro do celular*, em `js/local-ai.js`): o
  consultor roda no aparelho com o [wllama](https://github.com/ngxson/wllama) (llama.cpp em
  WebAssembly, MIT). Nada sai do celular: os nomes vão sem apelido e não precisa de
  internet depois de baixar o modelo.
  - **Modelo**: Qwen2.5 0.5B Instruct, Q4_K_M (Apache-2.0, ~470 MB), baixado do Hugging Face
    sob demanda, com a URL fixada num commit. Fica no armazenamento do app (OPFS), fora do
    APK; Ajustes tem o botão para apagar.
  - **Instruções e dados**: as mesmas regras em Go do consultor (`advisor.LocalRequest`, pelo
    `advisor.wasm`), com histórico curto e só os 10 maiores estabelecimentos, porque cada
    palavra do pedido custa tempo. Sem ferramentas: um modelo desse tamanho não as usa bem.
  - **Velocidade**: com WebGPU, o wllama usa a GPU (no teste num Mac, primeira palavra em ~3 s).
    Sem WebGPU, roda numa thread da CPU (o app não tem os cabeçalhos COOP/COEP das
    threads) e leva minutos por resposta; a tela avisa quando o celular não tem WebGPU.
  - **Empacotamento**: `@wllama/wllama` é dependência do `mobile/package.json`; o script
    `capacitor:copy:before` copia o JS e o `wllama.wasm` (~8,7 MB, ~2,4 MB comprimido no APK)
    para `www/static/vendor/wllama` em todo `cap sync`. O programa do computador não leva, e a
    versão do F-Droid também não (só aceita o que compila; veja DISTRIBUICAO.md).
  - A política de segurança do app libera `worker-src blob:`, porque o wllama cria o worker
    dele a partir de um blob.

- **Ler QR code** (Ajustes do app → *Ler QR code*): câmera do WebView (`getUserMedia`) +
  [jsQR](https://github.com/cozmo/jsQR) (Apache-2.0), em `js/qrscan.js`. Lê o QR dos Ajustes
  do computador (`http://…/parear?codigo=…`) e conecta. Sem bibliotecas fechadas (veja
  [LICENSES.md](LICENSES.md)).
- **Lembretes de vencimento**: `@capacitor/local-notifications` agenda, para cada dívida com dia
  de vencimento, um aviso mensal 2 dias antes, às 9h. É o próprio Android/iOS que avisa,
  então funciona sem internet. Liga e desliga em Ajustes do app (`js/native.js`).
- **Open Finance** (Ajustes do app → *Open Finance*): lançamentos direto do banco pelo Meu
  Pluggy, com as credenciais da própria pessoa. O Client ID e os Item IDs ficam no banco do
  app (meta `pluggy`); o Client Secret, no cofre do sistema (`pluggy-secret`, o mesmo plugin
  da chave da IA). A busca roda no `advisor.wasm` (`askapi.OpenFinance` → `internal/pluggy`) e
  sai do celular direto para `api.pluggy.ai` (CapacitorHttp, sem CORS). Sem credenciais no
  app, mas pareado, quem busca é o computador (`/api/openfinance`). Os lançamentos passam pela
  revisão da importação e caem na conta escolhida.
- **Notificações do banco** (Importar → *Notificações do banco*, só Android): o serviço
  `BankNotificationService` (um `NotificationListenerService`) guarda o texto das notificações
  de uma lista fechada de apps de banco (Nubank, Itaú, Inter, C6, BB, Bradesco, Santander,
  Caixa, Mercado Pago, PicPay, Neon, PagBank) nas preferências privadas do app, até 300 e 500
  caracteres cada. O plugin `BankNotifications` (Java no próprio app, registrado no
  `MainActivity`) expõe `status`, `openSettings` e `drain` (ler apaga). Em JS
  (`js/banknotes.js`), `importer.ParseNotification` (Go) separa compra, Pix e transferência
  de propaganda, saldo, fatura e compra negada; o que parece lançamento vai para a revisão da
  importação e sai do aparelho depois de importado. A pessoa autoriza em Ajustes do Android →
  *Acesso a notificações*. A Play Store exige justificar esse acesso na publicação; o F-Droid
  aceita.
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
- Job `test`: `scripts/check-licenses.sh` (só dependências de código aberto).
- Job `android`: compila o APK de release (R8 ligado; falha se passar de 12 MB) e o abre num emulador Android 14
  (`mobile/scripts/smoke-android.sh`): confere que a tela inicial aparece (WebView carregou
  as telas e o WebAssembly) e que não houve erro fatal.
- Job `ios`: compila para o simulador.

## Próximos passos

- **F-Droid / IzzyOnDroid**: receita, textos e capturas prontos; veja
  [DISTRIBUICAO.md](DISTRIBUICAO.md).
- **Play Store** (taxa única) e **App Store/TestFlight** (conta Apple anual): dependem das contas.
- **Chave de assinatura fixa do APK** (veja *Publicação* acima): para atualizar por cima.
