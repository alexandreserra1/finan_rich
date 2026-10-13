# Roadmap

Ideias para as próximas versões, com o que já foi pensado sobre cada uma.
A arquitetura atual está em [ARCHITECTURE.md](ARCHITECTURE.md).

## App mobile

> **Status:** começou na v1.2. Já existe o app Android com Capacitor (passos 1 a 4 e a
> distribuição do `.apk` abaixo). Como usar e como foi feito: [MOBILE.md](MOBILE.md).
> Faltam: iOS, notificações locais, leitor de QR dentro do app, consultor dentro do app,
> Play Store e testes num emulador Android no CI.

### Por que

Hoje o celular usa o finan_rich pelo navegador, como app instalado (PWA). Isso já cobre
bastante coisa:

- os dados ficam guardados no celular;
- o app funciona sem conexão enquanto está aberto;
- as alterações sincronizam quando a conexão com o computador volta.

Ficam três limitações que só um app de verdade resolve:

1. **Abrir sem conexão pelo Wi-Fi de casa.** O navegador só deixa o app abrir offline em
   endereços `https://`. Pelo Wi-Fi o endereço é `http://192.168…`, então hoje é preciso o
   Tailscale para isso. Um app instalado leva as telas dentro dele e abre sempre.
2. **Instalação familiar.** Baixar da loja (ou de um arquivo `.apk`) é mais natural para a
   maioria das pessoas do que "Adicionar à tela de início".
3. **Recursos do celular.** Notificações de vencimento de dívidas, atalhos e widgets, que no
   navegador são limitados (principalmente no iPhone).

### Opções avaliadas

| Opção | Como funciona | Reaproveita | Esforço | Observações |
|---|---|---|---|---|
| **Capacitor** (recomendada) | Empacota o app web atual (`static/js`, `finance.wasm`) num app Android/iOS | Quase tudo: telas, IndexedDB, sincronização, regras em Go | Baixo | As telas ficam dentro do app, então ele abre offline sempre. Com o HTTP nativo, sincroniza com `http://192.168…` sem precisar de HTTPS. |
| Flutter / React Native | Reescreve as telas com componentes nativos | Regras em Go via `gomobile` (ou reescritas) | Alto | Melhor sensação de app nativo, mas é uma segunda interface para manter. |
| Kotlin + Swift | Um app nativo por sistema | Regras em Go via `gomobile` | Muito alto | Só faz sentido com uma equipe dedicada a cada sistema. |
| PWA atual + Tailscale | O que já existe | Tudo | Zero | Funciona hoje; exige instalar o Tailscale no PC e no celular. |

### Plano sugerido (Capacitor)

1. **Parear o celular com o computador.**
   - O QR code dos Ajustes passa a levar o endereço do computador e um token de acesso
     (ex.: `finanrich://parear?url=…&token=…`).
   - O app lê o QR e guarda os dois no aparelho.
2. **Autenticação por token.**
   - Hoje a sincronização usa o cookie de sessão.
   - Para o app, `requireAuth` e `/api/sync` passam a aceitar também
     `Authorization: Bearer <token>`.
   - O servidor lista e revoga aparelhos pareados, aproveitando a tabela `sessions`.
3. **Endereço configurável.**
   - Hoje `sync.js` chama `/api/sync` no mesmo endereço da página.
   - No app, o endereço vem do pareamento, e as chamadas usam o plugin de HTTP nativo do
     Capacitor (sem as restrições de HTTPS/CORS do navegador).
4. **Projeto `mobile/`.**
   - `npx cap init`, com as pastas `android/` e `ios/`.
   - Um passo de build copia `internal/web/static` e o `finance.wasm` para dentro do app.
   - As telas que dependem do servidor (Consultor IA e Ajustes) abrem o endereço do
     computador quando houver conexão.
5. **Notificações locais** de vencimento das dívidas (campo `dueDay`), agendadas no próprio
   celular, que funcionam offline.
6. **Distribuição.**
   - Android: `.apk` assinado publicado no GitHub Releases pelo mesmo workflow de release
     (grátis); Play Store depois (taxa única).
   - iOS: exige conta de desenvolvedor da Apple (anual), um Mac para compilar e revisão da
     App Store; ou TestFlight para testes.
7. **Testes.** Rodar o `e2e/` também contra o app Android num emulador, no CI.

### Modo "só no celular" (opcional)

Para quem não quer deixar um computador ligado:

- o app guarda tudo no celular;
- o consultor usa um provedor de IA na nuvem (Ollama Cloud, Groq, OpenRouter);
- a sincronização vira opcional, com um servidor `finanrich` numa VPS (com `FINAN_PASSWORD`).

A arquitetura local-first atual já permite isso: o celular é dono dos dados e o servidor é
só o ponto de encontro.

## Outras ideias

- **Importar extratos** (OFX/CSV do banco), sugerindo categorias automaticamente.
- **Metas** (ex.: "juntar R$ 5.000 até dezembro"), como um novo tipo de registro sincronizado.
- **Exportar e restaurar backup** pela tela de Ajustes (hoje é copiar o arquivo `finan.db`).
- **Lançamentos recorrentes** automáticos, além do botão "Copiar do mês anterior".
- **Executáveis assinados** no Windows e no macOS, para não aparecer o aviso de "editor
  desconhecido".
