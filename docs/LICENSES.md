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
| `github.com/anthropics/anthropic-sdk-go` (+ `tidwall/*`, `invopop/jsonschema`, …) | falar com a API da Anthropic, **só se** você escolher essa IA | MIT / BSD / Apache-2.0 |
| `rsc.io/qr` | QR code de pareamento | BSD-3-Clause |
| `golang.org/x/sys`, `golang.org/x/sync`, `google/uuid`, `dustin/go-humanize` | utilitários | BSD / MIT |

Excel, CSV, PDF, OFX e a leitura de faturas são código nosso (sem bibliotecas).

## No navegador e no app (`internal/web/static/vendor`)

| Biblioteca | Para quê | Licença |
|---|---|---|
| [pdf.js](https://github.com/mozilla/pdf.js) 6.3 | ler o texto das faturas em PDF | Apache-2.0 |
| [jsQR](https://github.com/cozmo/jsQR) 1.4 | ler o QR code de pareamento pela câmera | Apache-2.0 |
| [Chart.js](https://github.com/chartjs/Chart.js) 4.5 | gráficos do Painel, da Análise, das Dívidas e do patrimônio | MIT |

## App do celular

| Dependência | Para quê | Licença |
|---|---|---|
| Capacitor 8 (`core`, `android`, `ios`, `cli`) | casca nativa com WebView | MIT |
| `@capacitor/app`, `filesystem`, `share`, `local-notifications` | links `finanrich://`, salvar/compartilhar arquivos, lembretes | MIT |
| `@wllama/wllama` (llama.cpp) | IA dentro do celular | MIT |
| Modelo Qwen2.5 0.5B Instruct (baixado sob demanda, não vai no app) | IA dentro do celular | Apache-2.0 |
| `@capgo/capacitor-native-biometric` | chave da IA no Keystore/Keychain e digital/rosto no bloqueio do app | MPL-2.0 |
| AndroidX (vem com o Capacitor) | base do app Android | Apache-2.0 |

O leitor de QR usa a câmera do próprio WebView (`getUserMedia`) com o jsQR. Antes era o
`@capacitor/barcode-scanner`, que traz o ML Kit do Google (fechado) e deixava o APK com
38 MB; sem ele, e com o R8 (`minifyEnabled`/`shrinkResources`), o APK fica bem menor e o
CI falha se passar de 12 MB.

## IA

- **Padrão: Ollama** (MIT) com modelos de pesos abertos (Qwen, Apache-2.0), tudo no seu
  computador.
- **Opcional**: Anthropic, OpenAI, Gemini, etc. são serviços fechados de terceiros. Só são
  usados se você configurar a chave, e então os dados enviados ao consultor saem do seu
  computador (veja o README).

## Distribuição

Como o app não tem nenhuma peça fechada, ele pode entrar no
[F-Droid](https://f-droid.org) (que compila a partir do código-fonte) ou no
[IzzyOnDroid](https://apt.izzysoft.de/fdroid/) (que usa o APK do GitHub Releases). Os dois
exigem a chave de assinatura fixa (veja `docs/MOBILE.md`).
