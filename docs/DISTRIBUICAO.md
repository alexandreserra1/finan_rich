# Como o app chega ao celular (sem pagar loja)

| Onde | Custo | Atualiza | Situação |
|---|---|---|---|
| **QR code / página `/baixar` do site** (APK no Cloudflare R2) | grátis | o app avisa e baixa a versão nova | ✅ pronto; falta criar o bucket (abaixo) |
| iPhone pelo Safari (Tela de Início) | grátis | sim (é o site) | ✅ já funciona |
| Play Store | US$ 25 uma vez | sim | não planejado (sem gasto) |
| App Store / TestFlight | US$ 99 por ano | sim | não planejado (sem gasto) |

O código da Rich não é mais aberto (licença em [LICENSE](../LICENSE)), então o F-Droid e o
IzzyOnDroid, que só aceitam código aberto, saíram. Decisão e alternativas em
[ADR-003](decisions/ADR-003-download-por-qr-code.md).

## Android: QR code e página de download

1. A pessoa aponta a câmera do celular para o QR code. Ele aparece na página
   `https://finan-rich-ia.vercel.app/baixar` (aberta no computador) e nos Ajustes do programa
   do computador (*App do celular*). O QR abre a mesma página no celular.
2. No celular, **Baixar o app** busca o APK (`/baixar/android` redireciona para o R2).
3. O Android pede, uma vez, para permitir instalar apps do navegador. Depois, **Instalar**.

**Versões novas:** cada release publica o APK no R2 e, por último, o `android/latest.json`.
Ao abrir, o app confere `/baixar/versao` e, se houver versão nova, avisa e mostra em Ajustes
o botão **Baixar a versão X**, que abre o download no navegador. O Android instala por cima
(mesma chave de assinatura) e os dados ficam.

Código: `site/baixar.html` (a página), `site/download.go` (rotas `/baixar/android`,
`/baixar/versao` e `/baixar/qr.png`), `newVersion` em `js/native.js` (o aviso no app) e o job
`android` de `.github/workflows/release.yml` (publica no R2).

### Criar o bucket no Cloudflare R2 (uma vez, grátis)

O plano grátis guarda 10 GB e o download não tem custo nem limite de tráfego.

1. Crie uma conta grátis em <https://dash.cloudflare.com> e abra **R2 Object Storage**
   (o Cloudflare pede um cartão para ligar o R2, mas o plano grátis não cobra).
2. **Create bucket**, nome `finan-rich` (ou outro).
3. No bucket, **Settings → Public access → R2.dev subdomain → Allow**. Copie o endereço
   (`https://pub-….r2.dev`) e me mande: ele não é segredo, vai em `site/download.go`.
4. Em **R2 → Manage API tokens → Create API token**, permissão *Object Read & Write*, só
   nesse bucket. Guarde o *Access Key ID* e o *Secret Access Key* (a chave nunca no chat).
5. No GitHub, **Settings → Secrets and variables → Actions**, crie os secrets:
   `R2_ACCOUNT_ID` (o *Account ID* do painel do R2), `R2_BUCKET` (o nome do bucket),
   `R2_ACCESS_KEY_ID` e `R2_SECRET_ACCESS_KEY`.

Sem esses secrets, a Release falha na etapa do APK (para não publicar pela metade). O
endereço `r2.dev` tem limite de pedidos por segundo, folgado para downloads; com um domínio
próprio no Cloudflare, dá para trocar por `download.<domínio>`.

### A verificação de desenvolvedor do Google

Desde 30/09/2026, no Brasil, o Android passou a exigir desenvolvedor verificado para apps
de lojas participantes; em 2027 a regra chega a todos os apps em celulares certificados.
Sem a verificação (US$ 25 e documento com foto, no Android Developer Console), quem instalar
o APK pode cair num "fluxo avançado": ligar o modo desenvolvedor, reiniciar o celular e
esperar 24 horas. A escolha foi não gastar (ADR-003). Caminhos grátis quando isso valer:
o site pelo navegador, que vira ícone na tela inicial e não passa por essa regra, e a conta
grátis de distribuição limitada (até 20 aparelhos, para testes).

## Computador: Homebrew, Scoop e winget

Todos gratuitos. Este repositório serve de *tap* do Homebrew
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
