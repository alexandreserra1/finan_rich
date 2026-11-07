# ADR-003: Download do app por QR code, com o APK no Cloudflare Pages, e licença fechada

## Situação

Aceita.

## Data

07/11/2026

## Contexto

Até a v1.10.0, o app Android era baixado do GitHub Releases, e o plano era entrar no F-Droid
e no IzzyOnDroid. O código era MIT. O Alexandre decidiu:

- o código deixa de ser aberto, mas continua à vista no GitHub ("só a licença fechada");
- a pessoa baixa o app por um QR code, não pelo GitHub;
- nada pago, e as ferramentas usadas devem ser de código aberto;
- sem domínio próprio por enquanto.

Pesquisa (07/11/2026):

- O F-Droid e o IzzyOnDroid só aceitam apps de código aberto.
- **Verificação de desenvolvedor do Google:** desde 30/09/2026, no Brasil, apps de lojas
  participantes exigem desenvolvedor verificado; em 2027, todos os apps em celulares
  certificados. App sem verificação só instala por um "fluxo avançado" (modo desenvolvedor,
  reiniciar e esperar 24 horas) ou pelo ADB. A verificação custa US$ 25 e pede documento;
  há uma conta grátis limitada a 20 aparelhos. Hoje, segundo a imprensa, APK baixado de site
  ainda instala normalmente no Brasil.
- Hospedagem grátis do APK (16 MB): Cloudflare Pages (sem cartão, tráfego sem limite,
  arquivos de até 25 MB), Cloudflare R2 (10 GB guardados, download sem custo; pede cartão),
  um segundo projeto na Vercel (sem cartão; 100 GB por mês divididos com o site e a IA, e o
  plano grátis é só para uso não comercial) e Vercel Blob (10 GB de download por mês).

## Decisão

- **Página `/baixar` no site** (`site/baixar.html`), com o QR code que a abre no celular
  (`/baixar/qr.png`, gerado em Go com `rsc.io/qr`), o botão de baixar e os passos. Os Ajustes
  do programa do computador mostram o mesmo QR.
- **APK no Cloudflare Pages** (projeto `finan-rich-app`). A Release assina o APK com a
  chave do projeto e publica, de uma vez só, `android/finan_rich.apk` e
  `android/latest.json` com o `wrangler` (código aberto, versão fixa). O site redireciona
  `/baixar/android` e `/baixar/versao` para lá (`site/download.go`; o endereço fica em
  `apkBase`).
- **Atualizador no próprio app**, no lugar das lojas: ao abrir, o app confere
  `/baixar/versao`; com versão nova, avisa e mostra em Ajustes o botão que abre o download no
  navegador. O Android instala por cima (mesma chave) e os dados ficam.
- **Licença:** todos os direitos reservados, código à vista. O que foi publicado antes desta
  mudança continua MIT. As bibliotecas de terceiros continuam só de código aberto (o CI
  confere), com os avisos delas no programa e em `docs/LICENSES.md`.
- **Sem a verificação do Google** (sem gasto). O F-Droid, o IzzyOnDroid, a receita, o
  workflow F-Droid e os textos e capturas das lojas (`fastlane/`) saíram.

## Alternativas consideradas

- **GitHub Releases** (como antes): pedido para sair.
- **F-Droid / IzzyOnDroid:** exigem código aberto.
- **Play Store** (US$ 25, mais 12 testadores por 14 dias para contas novas): instala e
  atualiza sozinho, mas tem custo.
- **Cloudflare R2:** foi a primeira escolha, mas pede cartão para ligar.
- **Vercel:** no projeto do site, cada push publicaria o site sem o APK; um segundo projeto
  divide os 100 GB por mês com o site e a IA (passou, a Vercel pausa todos) e o plano grátis
  é só para uso não comercial.
- **Vercel Blob:** o download fica bloqueado depois de 10 GB no mês (uns 600 APKs).
- **Hostinger:** paga.
- **Instalar a atualização sem o navegador** (PackageInstaller, com a permissão
  `REQUEST_INSTALL_PACKAGES`): menos toques, mas exige código nativo e uma permissão sensível.
  O navegador já baixa e o Android já confere a chave.

## Consequências

- **Configuração única:** o Alexandre cria o token da Cloudflare e os secrets
  `CLOUDFLARE_*` ([DISTRIBUICAO.md](../DISTRIBUICAO.md)); a primeira Release cria o projeto e
  mostra o endereço, que vai para `apkBase`. Até lá, `/baixar/android` responde "ainda não
  está no ar" e a Release falha na etapa do APK.
- **Verificação do Google:** quando a regra valer para APK de site (2027, talvez antes no
  Brasil), quem instalar pode cair no fluxo avançado. Os caminhos grátis seguem: o site no
  navegador (vira ícone na tela inicial) e a conta limitada a 20 aparelhos para testes.
  Pagar os US$ 25 resolve, se a regra de não gastar mudar.
- **Limite de 25 MB por arquivo no Pages:** o APK tem uns 16 MB e o CI barra acima de 18 MB.
  Se o app crescer além disso, a saída é o R2 (com cartão).
- **iPhone:** sem app (a App Store custa US$ 99 por ano); a página leva ao site no Safari.
- **Programa do computador:** continua no GitHub Releases, Homebrew, Scoop e winget, como antes.
