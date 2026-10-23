# Roadmap

Ideias para as próximas versões, com o que já foi pensado sobre cada uma.
A arquitetura atual está em [ARCHITECTURE.md](ARCHITECTURE.md).

## App mobile

> **Status:** app Android desde a v1.2; na v1.5, leitor de QR, consultor dentro do app,
> lembretes de vencimento (notificações locais), projeto iOS compilado no CI e teste do APK
> num emulador Android no CI. Como usar e como foi feito: [MOBILE.md](MOBILE.md).
> Falta só o que depende de contas pagas: Play Store, App Store/TestFlight (conta Apple) e a
> chave fixa de assinatura do APK (secrets do GitHub, ver MOBILE.md).

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

### Modo "só no celular"

> **Feito em parte:** Ajustes do app → *Usar só neste celular* cria as categorias no aparelho e
> tudo (menos o consultor) funciona sem computador. Se depois parear, as categorias padrão se
> juntam às do computador sem duplicar. Consultor sem computador: IA na nuvem com plano
> grátis, chamada do celular com os nomes anonimizados e a chave cifrada (ver MOBILE.md).
> IA dentro do aparelho (wllama/llama.cpp): feito, ver MOBILE.md. Ela é pequena e precisa
> de WebGPU para ser rápida; sem ferramentas (não lança nada pela conversa).

Para quem não quer deixar um computador ligado:

- o app guarda tudo no celular;
- o consultor usa um provedor de IA na nuvem (Ollama Cloud, Groq, OpenRouter);
- a sincronização vira opcional, com um servidor `finanrich` numa VPS (com `FINAN_PASSWORD`).

A arquitetura local-first atual já permite isso: o celular é dono dos dados e o servidor é
só o ponto de encontro.

## Parecido com o Pierre, só que no seu aparelho

O [Pierre](https://pierre.finance) é um assistente financeiro que lê suas contas pelo Open
Finance e conversa pelo WhatsApp. O finan_rich já tem o consultor com IA e os dados no seu
aparelho; o que falta para chegar perto, do mais simples ao mais caro:

1. ~~**Importar extratos** (OFX, CSV e fatura em PDF)~~ — feito: categoria pelo histórico,
   palavras-chave ou IA, sem duplicar.
2. ~~**Consultor mais completo**~~ — feito em parte: a IA recebe os gastos por estabelecimento
   (12 meses), metas, recorrentes e notas, e as respostas podem virar nota. Com
   *ferramentas*: busca lançamentos de qualquer período; lança, corrige, apaga, cria metas e
   dívidas por mensagem, registra aportes em metas e pagamentos de dívidas, sempre com um
   cartão para confirmar (salvo na conversa).
3. ~~**Alertas**~~ — feito: avisos no painel e lembrete de vencimento no celular.
4. ~~**Open Finance**~~ — feito pelo Meu Pluggy (grátis para a pessoa, com a conta dela; os
   dados vão do banco direto para o aparelho). Falta: buscar sozinho de tempos em tempos
   (hoje é pelo botão "Buscar do banco") e ler as notificações dos apps de banco no Android.
5. **WhatsApp**: exige a API paga da Meta e um servidor sempre ligado na internet.

## Outras ideias

- ~~**Metas** com valor e prazo~~ — feito (tela Metas e painel).
- ~~**Contas e cartões** (fatura, compra parcelada, patrimônio)~~ — feito (tela Contas).
- ~~**Busca e filtros nos lançamentos**, **orçamento por categoria** e **primeiro uso
  guiado**~~ — feito.
- ~~**Lançamentos recorrentes** automáticos~~ — feito ("Repetir todo mês").
- ~~**Instalar como aplicativo no computador**~~ — feito: um comando no terminal
  (`install.sh`/`install.ps1`) instala, cria o ícone no menu e abre em janela própria; sem o
  aviso de "editor desconhecido", porque não passa pelo navegador. O programa avisa quando
  sai versão nova, com o comando de atualizar.
- **Executáveis assinados** no Windows e no macOS: só faz diferença para quem baixa o
  arquivo pelo navegador (custa a conta Apple e um certificado de código no Windows).
- ~~**Homebrew e Scoop**~~ — feito (este repositório é o tap e o bucket). **winget**:
  configurado, liga com o secret `WINGET_GITHUB_TOKEN` (ver DISTRIBUICAO.md).
- **Flathub, .deb/.rpm**: gratuitos; vale quando houver mais usuários de Linux pedindo.

## Pendências conhecidas (revisão de outubro de 2026)

Achados das revisões de código, segurança e testes que ficaram para depois:

- **Cota grátis da IA na nuvem**: cada pergunta com ferramentas faz de 2 a 5 chamadas; o
  plano grátis do Gemini acaba rápido (erro 429, a tela avisa). Vale mostrar o uso do dia.
- **Pelo Wi-Fi o tráfego é HTTP**: para usar fora de casa, Tailscale (HTTPS).
- **IA "custom" num proxy local** (LiteLLM etc.) conta como local e vai sem apelidos.
