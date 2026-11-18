# Segurança e privacidade no desenho da Rich

Mapa do capítulo "Segurança no projeto de sistemas" (autenticação, dados, aplicação,
infraestrutura e conformidade) para a Rich. Para limites de pedidos e abuso, veja
[LIMITACAO-DE-TAXAS.md](LIMITACAO-DE-TAXAS.md).

## Escolha para a Rich

A Rich não tem um servidor que guarde dados financeiros. Os dados ficam no celular, no
computador ou no Google Drive da própria pessoa. Por isso a maior parte do capítulo vira
"guardar pouco, guardar perto da pessoa e proteger cada porta": o computador que serve as
telas pela rede, o app do celular, o site, o intermediário da IA na Vercel e o cadastro sem
Google (Supabase), que ainda não está ligado.

Entrou nesta mudança:

1. **Backup sem segredos**: o `backup.db` do computador não leva mais a chave da IA, o
   segredo do Pluggy, as sessões nem os aparelhos pareados. O texto apagado também não fica
   no arquivo, conferido byte a byte no teste.
2. **Registro de acesso** no computador (Ajustes → Acesso pelo celular): entradas pela rede,
   senhas erradas, saídas, pareamentos e códigos errados, aparelhos desconectados, troca de
   senha, acesso ligado e desligado, backup baixado, troca da chave da IA e das credenciais do
   Open Finance. Cada evento tem data e endereço; nunca senha, token ou dado financeiro.
3. **Análise estática no CI** (CodeQL, `.github/workflows/codeql.yml`): Go, JavaScript e os
   próprios workflows, a cada PR, na main e toda segunda.
4. **A política de privacidade** diz que a pergunta à IA grátis passa pelos Estados Unidos.
5. Este documento, com a classificação dos dados e a LGPD.

## Autenticação e autorização

Cada porta responde às duas perguntas do capítulo: quem é você e o que pode fazer.

| Porta | Quem é você | O que pode fazer |
|---|---|---|
| Computador, neste computador | O dono do computador (sem senha). Outro site não usa esse acesso: `Origin`/`Sec-Fetch-Site` barram CSRF e `trustedHost` barra DNS rebinding. | Tudo, inclusive fechar o programa e baixar o banco. |
| Computador pela rede (Wi-Fi, Tailscale) | Senha (PBKDF2-HMAC-SHA256, 600 mil iterações, sal próprio, comparação em tempo constante), com limite de tentativas. Sessão de 30 dias no servidor: o banco guarda só o hash SHA-256 do token; o cookie é `HttpOnly` e `SameSite=Lax`. | Tudo, menos fechar o programa. Trocar a senha ou desligar o acesso derruba todas as sessões e aparelhos. |
| App do celular pareado | Token por aparelho (só o hash no banco), dado uma vez com um código de pareamento de uso único. | Só a API (`/api/`: sincronizar, consultor). Não troca a IA nem baixa o banco. Parado 180 dias, perde o acesso; "Desconectar" revoga na hora. |
| Site e app com Google | OAuth 2.0 com código, PKCE e `state`; OpenID Connect só para mostrar o e-mail. | Escopo `drive.appdata`: só a pasta escondida da Rich no Drive. |
| Intermediário da IA | Ninguém: o `Bearer` do aparelho pode ser inventado e serve só para dividir a cota. | Uma pergunta com as instruções da Rich, dentro dos limites por aparelho e por IP. |
| Cadastro sem Google (não ligado) | E-mail e senha no Supabase. | RLS: cada pessoa só lê o próprio perfil; o papel do limitador só executa a função. |

**Papéis (RBAC).** Na prática são três: o dono no computador, a sessão pela rede e o
aparelho pareado. Cada um tem só o que precisa, e isso basta para uma pessoa por
instalação. ABAC e papéis configuráveis não se aplicam.

**Sessão ou JWT.** No computador, sessão no servidor: dá para revogar na hora, sem lista
negra. Com o Google vale o híbrido do capítulo: o token de acesso dura 1 hora e fica só na
memória; o token de renovação, que pode ser revogado, fica no cofre do sistema (app) ou num
cookie `HttpOnly` do intermediário (site), nunca no JavaScript do site.

**SSO e SAML.** O "Entrar com Google" já é o SSO (OIDC). SAML é para empresas e não se aplica.

**MFA.** O celular tem o bloqueio do app (digital ou rosto, ou a senha do app) contra quem
pega o aparelho desbloqueado. O acesso pela rede ao computador ainda é só senha: veja
"Decisões pendentes".

## Segurança dos dados

**Em trânsito.** O site, o intermediário, o Google, o Groq e o Banco Central usam HTTPS. O site
manda HSTS de 1 ano. No Wi-Fi de casa a conexão com o computador é `http` sem cifra: os
Ajustes avisam, e fora de casa o caminho é o Tailscale (WireGuard, com HTTPS). Não há TLS
próprio na rede de casa porque o certificado seria autoassinado e o navegador o recusaria.

**Em repouso.**

- **Celular.** O banco fica na área privada do app, que o Android e o iOS cifram quando o
  aparelho tem bloqueio de tela. A chave da IA fica no Keystore/Keychain (o banco só tem a
  versão cifrada) e o token do Google fica no cofre. O banco SQLite do app não tem cifra
  própria: veja "Decisões pendentes".
- **Navegador (site).** Os dados ficam no banco do navegador. A chave da IA é cifrada com
  AES-GCM por uma chave do aparelho que o navegador não deixa exportar.
- **Computador.** O banco fica na pasta do usuário, protegido pela conta do sistema. A chave
  da IA e o segredo do Pluggy ficam em texto no banco. Cifrar com uma chave guardada ao lado
  não protegeria nada (o capítulo avisa: chave junto do dado não é proteção). O caminho certo
  seria o chaveiro do sistema. Enquanto isso, recomenda-se a cifra de disco do sistema
  (FileVault, BitLocker), e o backup sai sem esses segredos.
- **Servidor.** Nenhum dado financeiro. Os segredos do intermediário (chaves do Groq e do
  Google) ficam nas variáveis da Vercel.

**Senhas.** PBKDF2-HMAC-SHA256 com 600 mil iterações, que é a recomendação da OWASP para
PBKDF2, e sal por senha. O capítulo cita bcrypt, scrypt e Argon2. PBKDF2 nesse nível é a
opção aceita pela OWASP (e pelo FIPS) e está na biblioteca padrão do Go, sem dependência nova.

**Mascaramento e tokenização.** A IA na nuvem só vê apelidos ("Loja AB", "Dívida 1", "Meta 1")
e nunca as notas. O mapa de volta fica só no aparelho, como o cofre de tokens do capítulo.
Os logs gravam método e caminho, sem corpo nem consulta; o intermediário grava só o hash do
aparelho. Testes e capturas usam dados fictícios.

**Classificação dos dados.**

| Classe | Na Rich | Controles |
|---|---|---|
| Pública | Site, código, política de privacidade | Nenhum. |
| Interna | Modelo de IA escolhido, tema, categorias padrão | Ficam no aparelho. |
| Confidencial | Lançamentos, dívidas, metas, notas, conversa do consultor, e-mail da conta Google | Só no aparelho, no computador ou no Drive da pessoa. Apelidos para a IA. Exportar e apagar à mão. |
| Restrita (segredo) | Senha, tokens de sessão e de aparelho, chave da IA, segredo do Pluggy, token do Google, segredos do servidor | Hash (senha e tokens), cofre ou AES-GCM (celular e site), variáveis da Vercel. Nunca em log, backup ou commit. |

## Aplicação e infraestrutura

- **Entrada.** Todo SQL usa parâmetros (conferido em 18/11/2026: nenhuma consulta montada com
  texto). Todo registro passa pelas regras do Go (`domain`), iguais no computador, no
  navegador e no celular. Todo corpo de pedido tem limite de tamanho.
- **XSS.** As telas escapam o texto (`html/template` e o `html` do JS). O markdown das respostas
  da IA escapa o HTML antes e não gera links. O CSP não aceita script embutido
  (`browserCSP`, `appCSP`, `siteCSP`), e há `frame-ancestors 'none'` e `nosniff`.
- **CSRF.** `Origin`/`Sec-Fetch-Site` e cookie `SameSite=Lax`. O app usa `Bearer`, que o
  navegador nunca manda sozinho.
- **DDoS e WAF.** A borda da Vercel recebe o tráfego, e o Go tem limites por IP e por aparelho
  ([LIMITACAO-DE-TAXAS.md](LIMITACAO-DE-TAXAS.md)). A regra do Firewall da Vercel é configuração
  do dono no painel.
- **Menor privilégio.** Os workflows começam com `contents: read`. Ganham escrita só o release
  (`contents: write`) e o CodeQL (`security-events: write`, para enviar o resultado). O token
  do celular vale só para a API e o Google só para a pasta do app.
- **Rede.** O computador escuta só nele mesmo até a pessoa ligar o acesso pelo celular (com
  senha). Nada fica aberto na internet.
- **Segredos.** Nenhum no repositório. O GitHub tem secret scanning e push protection ligados
  (conferido em 18/11/2026).
- **Confiança zero.** Cada pedido ao computador é autenticado (sessão, token ou, neste
  computador, o endereço conferido). Estar no mesmo Wi-Fi não dá acesso.

## Conformidade e privacidade (LGPD)

A lei que vale é a LGPD. A GDPR e a CCPA têm a mesma ideia; a HIPAA (saúde) não se aplica.

- **Acesso e portabilidade.** Exportar em Excel, CSV, PDF e backup.
- **Eliminação.** "Apagar dados" no app e no site. No computador, apagar o banco. O arquivo no
  Drive é da pessoa.
- **Transparência e finalidade.** A política de privacidade (`site/privacidade.html`) diz o que
  sai do aparelho, para onde vai e por quê.
- **Transferência internacional.** A pergunta à IA grátis, já com apelidos, passa pela Vercel e
  pelo Groq, nos Estados Unidos. Os dados financeiros não saem do aparelho, do computador ou do
  Drive da pessoa.
- **Registro de auditoria.** O registro de acesso guarda os últimos 200 eventos no próprio
  computador. Ele não é à prova de adulteração: quem controla o computador controla o
  registro. Serve para a pessoa ver se alguém entrou pela rede, não como prova para terceiros.

**Segurança em todo o ciclo (SSDLC).**

| Fase | O que a Rich faz |
|---|---|
| Desenho | Cada mudança relevante tem proposta com riscos (OpenSpec). |
| Desenvolvimento | `gofmt`, `go vet`, revisão e CodeQL. |
| Testes | Go, navegador em Chromium e WebKit, simulador do iPhone e Android no CI. |
| Dependências | `govulncheck`, `npm audit` e só licenças abertas (`scripts/check-licenses.sh`). |
| Publicação | Versão só pelo workflow Release. Push protection contra segredo em commit. |
| Operação | O workflow diário confere a IA grátis publicada. |

## Decisões pendentes (do dono)

1. **Código de 6 dígitos (MFA) no acesso pela rede.** Seria opcional, com o código do app
   autenticador (TOTP) depois da senha, QR pelo gerador que os Ajustes já usam e códigos de
   recuperação. Muda o login, por isso espera confirmação.
2. **Backup automático do Android.** O manifesto tem `allowBackup="true"`, então o Android copia
   o banco do app para o backup do Google (cifrado de ponta a ponta quando o celular tem
   bloqueio de tela). Desligar protege mais, mas quem troca de celular sem Drive, computador ou
   backup da Rich perde os dados.
3. **Banco do celular cifrado (SQLCipher do plugin).** Exige migrar os bancos que já existem,
   com risco de perda, e só se testa num celular de verdade.
4. **Chave da IA no chaveiro do computador.** Precisa de dependência nova. Hoje fica no banco,
   fora do backup.
5. **Verificação em duas etapas nas contas do dono** (GitHub, Vercel, Google Cloud, Cloudflare,
   Groq). Quem entrar numa delas troca os segredos ou o código publicado. Não dá para conferir
   daqui.
6. **Regra de limite por IP no Firewall da Vercel**, pendente desde a auditoria de 03/11.

## O que não se aplica

- SAML e SSO corporativo, RBAC com papéis configuráveis, ABAC: é uma pessoa por instalação.
- VPC, sub-redes, grupos de segurança, KMS e Vault: não há servidor com dados nem equipe com
  acesso. A Vercel só repassa pedidos.
- Tokenização de cartão (PCI DSS): a Rich não guarda número de cartão.
- HIPAA: não há dado de saúde.

## Validação

- `go test ./internal/store`: `TestBackupLeavesSecretsOut` lê os bytes do backup procurando a
  chave da IA, o segredo do Pluggy e o hash do aparelho. `TestAccessLog` confere a ordem e o
  teto de 200.
- `go test ./internal/web`: `TestAccessLogInSettings` faz entrada errada e certa pela rede,
  saída, pareamento errado e certo, backup, troca de senha, da chave da IA e do Open Finance.
  Confere que tudo aparece nos Ajustes com o endereço e que nenhuma senha ou chave vai para o
  registro.
- CodeQL roda no PR. Os avisos que ele achar são tratados antes do merge.

Não foram feitos teste de invasão, ataque de carga nem verificação das contas do dono.
