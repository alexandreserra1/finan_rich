# Código aberto: o que vai dentro do finan_rich

O finan_rich é [MIT](../LICENSE). Tudo o que vai dentro do programa do computador e do app
do celular também é código aberto, sem SDK fechado, sem Google Play Services, sem
rastreamento. O CI confere isso a cada mudança (`scripts/check-licenses.sh`): uma
dependência nova com licença fora da lista (MIT, BSD, ISC, Apache-2.0, MPL-2.0) faz o
build falhar.

## Programa do computador (Go)

| Dependência | Para quê | Licença |
|---|---|---|
| `modernc.org/sqlite` (+ `libc`, `mathutil`, `memory`) | banco SQLite em Go puro (sem C) | BSD-3-Clause |
| `rsc.io/qr` | QR code de pareamento | BSD-3-Clause |
| `golang.org/x/sys`, `golang.org/x/sync`, `google/uuid`, `dustin/go-humanize` | utilitários | BSD / MIT |

Excel, CSV, PDF, OFX e a leitura de faturas são código nosso (sem bibliotecas).

## No navegador e no app (`internal/web/static/vendor`)

| Biblioteca | Para quê | Licença |
|---|---|---|
| [pdf.js](https://github.com/mozilla/pdf.js) 6.3 | ler o texto das faturas em PDF | Apache-2.0 |
| [jsQR](https://github.com/cozmo/jsQR) 1.4 | ler o QR code de pareamento pela câmera | Apache-2.0 |
| [Chart.js](https://github.com/chartjs/Chart.js) 4.5 | gráficos do Painel, da Análise, das Dívidas e do patrimônio | MIT |
| [SQLite WebAssembly](https://github.com/sqlite/sqlite-wasm) 3.54 (pacote npm @sqlite.org/sqlite-wasm) | banco de dados da web (site e navegador do computador), gravado no OPFS | Apache-2.0 (o SQLite é de domínio público; a cola do Emscripten, MIT) |
| [DM Sans](https://github.com/googlefonts/dm-fonts) (pacote npm @fontsource-variable/dm-sans 5.3.0) | fonte do texto das telas, embutida (funciona sem internet) | SIL OFL 1.1 (só para fontes) |
| [Manrope](https://github.com/sharanda/manrope) (pacote npm @fontsource-variable/manrope 5.3.0) | fonte dos títulos e números, embutida | SIL OFL 1.1 (só para fontes) |

## App do celular

| Dependência | Para quê | Licença |
|---|---|---|
| Capacitor 8 (`core`, `android`, `ios`, `cli`) | casca nativa com WebView | MIT |
| `@capacitor/app`, `filesystem`, `share`, `local-notifications` | links `finanrich://`, salvar/compartilhar arquivos, lembretes | MIT |
| `@capgo/capacitor-native-biometric` | chave da IA no Keystore/Keychain e digital/rosto no bloqueio do app | MPL-2.0 |
| `@capacitor-community/sqlite` 8.1 | banco SQLite nativo do app (no lugar do IndexedDB do WebView) | MIT |
| SQLCipher (`net.zetetic:sqlcipher-android` no Android, `SQLCipher.swift` no iOS), ZIPFoundation (iOS) | o SQLite que o plugin usa por baixo (o app não liga a criptografia) e a cópia de bancos zipados (o app não usa) | BSD-3-Clause / MIT |
| AndroidX (vem com o Capacitor) | base do app Android | Apache-2.0 |

O leitor de QR usa a câmera do próprio WebView (`getUserMedia`) com o jsQR. Antes era o
`@capacitor/barcode-scanner`, que traz o ML Kit do Google (fechado) e deixava o APK com
38 MB; sem ele, e com o R8 (`minifyEnabled`/`shrinkResources`), o APK fica bem menor e o
CI falha se passar de 18 MB (o banco nativo, com o SQLCipher, ocupa uns 8 MB).

Ferramentas de montar o app (vêm com o `@capacitor/cli` e **não vão dentro do APK**): `glob`,
`minimatch`, `rimraf` e `@isaacs/nonrecursive-glob-match` usam a BlueOak-1.0.0, licença
permissiva aprovada pela OSI. A checagem do `scripts/check-licenses.sh` cobre só o que vai no
app (`--production`), por isso não as lista.

## IA

- **Padrão: Ollama** (MIT) com modelos de pesos abertos (Qwen, Apache-2.0), tudo no seu
  computador.
- **Opcional, nos planos grátis**: a IA grátis da Rich (Groq), Groq, Gemini e OpenRouter são
  serviços de terceiros. Só são usados se você escolher, e então o resumo enviado ao
  consultor sai do seu computador com apelidos no lugar dos nomes (veja o README). A Rich
  não oferece IA paga.

## Distribuição

Como o app não tem nenhuma peça fechada, ele pode entrar no
[F-Droid](https://f-droid.org) (que compila a partir do código-fonte) ou no
[IzzyOnDroid](https://apt.izzysoft.de/fdroid/) (que usa o APK do GitHub Releases). Os dois
exigem a chave de assinatura fixa (veja `docs/MOBILE.md`).
