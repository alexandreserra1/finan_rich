# Site e "Entrar com Google"

O site do finan_rich (`https://finan-rich-ia.vercel.app`) são as telas do app do celular
publicadas na internet. Sem conta, os dados ficam no navegador. Com **Entrar com Google**,
os dados vão também para o **Google Drive da própria pessoa**, numa pasta de dados do app
(`appDataFolder`, que só o finan_rich enxerga), e aparecem iguais no site e no app. Nenhum
servidor do finan_rich guarda dados.

```
navegador / app ──login (código + PKCE)──▶ Google ──volta──▶ /oauth (site)
       │                                                   │ web: termina aqui
       │                                                   │ app: finanrich://google
       │  POST /oauth/token, /oauth/refresh                │ pc.<porta>: 127.0.0.1
       ├──────────────────▶ intermediário (cmd/api, Vercel; tem o client secret)
       │                         └──▶ oauth2.googleapis.com/token
       └── Drive API (Bearer) ──▶ appDataFolder/finan_rich.json (todos os registros)
```

- **Código**: `js/google.js` (login, troca e renovação do token), `js/gdrive.js` (arquivo no
  Drive e rodada de sincronização), `js/pages/oauth.js` (volta do login),
  `internal/drivesync` (regra: precisa regravar o arquivo?), `internal/proxy/oauth.go`
  (rotas `/oauth/config`, `/oauth/token`, `/oauth/refresh`), pacote `site` (serve as telas
  no mesmo programa da Vercel).
- **Mesclagem**: vence o `updatedAt` maior, como em toda sincronização. Cada aparelho baixa o
  arquivo quando a versão muda, aplica e regrava o arquivo inteiro se tiver algo que falta lá.
  Duas gravações no mesmo instante: a que perdeu regrava na próxima rodada (o dado continua
  no aparelho). Categorias padrão têm o mesmo ID em todo aparelho, então não duplicam.
- **Segurança**: o client secret fica só na Vercel; o código do login vem com PKCE (sem o
  verificador, que fica no aparelho, ele não serve); o token de renovação fica no cofre do
  sistema no app e no banco do navegador no site; o de acesso (1 hora), só na memória. As
  rotas `/oauth/*` não guardam nada, não registram tokens e têm limite por IP. A volta para o
  computador só aceita `127.0.0.1` com porta numérica. Escopos: `openid email` e
  `drive.appdata` (não sensíveis para o Google).
- **Sair** (Ajustes → Conta Google) revoga o acesso no Google e apaga o token do aparelho.
  No site, como o computador pode ser de outra pessoa, sair também tira os dados do
  navegador: antes ele salva tudo no Drive e, se não conseguir, não sai nem apaga nada.
  Entrar de novo traz os dados de volta do Drive. No celular, os dados ficam no aparelho.

Por enquanto o login funciona no site. O app Android e o programa do computador entram nas
próximas versões (o caminho de volta `app.*` e `pc.*` da página `/oauth` já existe).

## Cadastro no Google (uma vez, ~10 minutos, sem cartão)

1. **Projeto**: em console.cloud.google.com, seletor de projetos → **Novo projeto** →
   `finan-rich` → **Criar**, e selecione o projeto.
2. **API do Drive**: **APIs e serviços → Biblioteca** → **Google Drive API** → **Ativar**.
3. **Tela de login** (Google Auth Platform → **Começar**): nome `finan_rich`, e-mail de
   suporte, público **Externo**, e-mail de contato → **Criar**. Em **Branding**: página inicial
   `https://finan-rich-ia.vercel.app`, política `https://finan-rich-ia.vercel.app/privacidade.html`,
   domínio autorizado `finan-rich-ia.vercel.app`.
4. **Acesso a dados** → **Adicionar ou remover escopos**: `.../auth/userinfo.email`, `openid`,
   `.../auth/drive.appdata` → **Salvar**.
5. **Público** → **Publicar app** → **Em produção** (em "Teste" o login expira em 7 dias).
6. **Clientes** → **Criar cliente** → **Aplicativo da Web**, nome `finan_rich`, URI de
   redirecionamento autorizado `https://finan-rich-ia.vercel.app/oauth` → **Criar**.
7. **Vercel** (projeto `finan-rich-ia` → Settings → Environment Variables): `GOOGLE_CLIENT_ID`
   com o ID do cliente e `GOOGLE_CLIENT_SECRET` com a chave secreta (nunca em chat ou no
   repositório) → **Deployments → Redeploy**.

Pronto: o botão "Entrar com Google" aparece nas boas-vindas e em Ajustes. Sem as variáveis,
`/oauth/config` responde 404 e o botão não aparece.

Trocar a chave secreta: crie outra no mesmo cliente, troque `GOOGLE_CLIENT_SECRET` na Vercel,
Redeploy, e apague a antiga.
