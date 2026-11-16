# Cadastro da Rich sem depender do Google

## Estado e objetivo

Supabase Auth aprovado pela pessoa em 16/11/2026. Ainda não há projeto Supabase
criado nem cadastro ativado. Esta entrega prepara a base e os contratos; não
registra endpoints públicos novos nem aplica migrações na produção.

A base SQL em `internal/identity/migrations/001_accounts.sql` foi executada apenas
em PostgreSQL 16 efêmero e isolado. O teste `sh scripts/test-identity-db.sh` cobre
papéis, isolamento de perfis, contexto transacional, restrições e exclusão em cascata.
Isso não comprova cadastro ou entrega de e-mail reais.

O CI executa o contrato PostgreSQL em cada PR. O banco de teste não usa rede,
volumes persistentes, credenciais de produção nem um projeto Supabase real.

Objetivo: criar conta com e-mail e senha, confirmar o e-mail, recuperar acesso e
entrar sem Google. O Google existente continua funcionando. Entrar na Rich e
autorizar o Drive são operações diferentes: uma conta de e-mail não ganha acesso
ao Drive e não deve receber uma permissão Google simulada no banco local.

## O que existe hoje

- Go serve a API e a interface; as regras financeiras são Go/Wasm.
- O site identifica a sessão pela integração Google; `main.js`, `db-idb.js` e
  `db-sqlite.js` possuem guardas específicas dessa sessão.
- O SQLite do programa do computador é um banco de uma instalação, não um banco
  multiusuário da Vercel. Não deve receber senhas de clientes do site.
- O intermediário na Vercel não possui banco persistente de contas. Um arquivo
  SQLite no disco temporário de uma função não é armazenamento durável.
- Hoje a sincronização remota do site usa o Drive pessoal. Um novo login não
  cria sozinho uma alternativa de armazenamento para os lançamentos.
- O banco do navegador tem um nome compartilhado. Isolar os dados por identidade
  deve acontecer antes de permitir alternar contas ou provedores.

## Direção escolhida e limites

Usar Supabase Auth para identidade e manter um BFF em Go, no mesmo domínio da Rich.
As telas continuam em JavaScript sem framework. Não é necessário adicionar React
nem duplicar o sistema de senhas no Go.

O provedor cuida de hashes de senha, confirmação, recuperação e revogação. A Rich
continua responsável por autorização, cookies, proteção contra abuso, isolamento
dos dados, tratamento de falhas e políticas de privacidade.

```text
Interface Rich -> /auth/* no Go -> Supabase Auth
                       |
                       +-> sessão opaca e revogável em PostgreSQL

Interface offline -> regras Go/Wasm -> banco local da identidade
                       |
                       +-> transporte de sincronização escolhido
                           Drive existente OU nuvem Rich aprovada
```

Não substituir o login Google atual nem vincular identidades só porque os e-mails
coincidem. Vinculação exige sessão recente e prova de controle das duas contas.

### Decisão de produto sobre os dados

Há dois recortes possíveis, que não devem ser confundidos:

1. Conta de e-mail com dados apenas no aparelho: sem promessa de recuperação ou
   sincronização entre dispositivos. A pessoa precisa exportar backup. Isso precisa
   aparecer antes do cadastro, não escondido nos termos.
2. Conta de e-mail com sincronização Rich: PostgreSQL persistente, controle por
   usuário, backup e exclusão. Introduz armazenamento financeiro no servidor,
   custos e obrigações de privacidade. Exige consentimento e atualização da
   landing e da política, que hoje dizem que o servidor não guarda lançamentos.

Recomendação para um produto de clientes: opção 2, com adesão explícita. Até essa
ativação, preservar o armazenamento existente e não prometer sincronização nova. Não enviar
automaticamente dados antigos do navegador ou do Drive para o novo serviço.

## Backend e contratos propostos

Rotas ainda não implementadas. Respostas não retornam senha, hash, refresh token
nem JWT ao JavaScript do site. Erros usam código público e mensagem em português.

| Método e rota | Entrada | Resultado |
|---|---|---|
| `GET /auth/config` | Nenhuma | Métodos habilitados; sem chaves privadas. |
| `POST /auth/signup` | E-mail, senha, versão dos termos | `202` com mensagem neutra, inclusive para conta já existente. |
| `POST /auth/confirm` | Token de uso único recebido por e-mail | Confirmação; não aceita redirecionamento arbitrário. |
| `POST /auth/login` | E-mail, senha, prova antiabuso quando exigida | Cookie de sessão opaca; erro genérico em credenciais incorretas. |
| `GET /auth/session` | Cookie | Identidade, prazo, geração de sessão e capacidades: local, Drive ou nuvem; `401` quando ausente ou vencida. |
| `POST /auth/logout` | Cookie e proteção CSRF | Revoga sessão sem depender da sincronização; preserva a fila isolada, sem apagar silenciosamente. |
| `POST /auth/password/request` | E-mail | `202` neutro, exista a conta ou não. |
| `POST /auth/password/reset` | Token de uso único e nova senha | Troca senha e revoga sessões anteriores. |
| `POST /auth/sessions/revoke` | Sessão recente | Revoga outros aparelhos. |
| `POST /auth/account/delete` | Reautenticação e confirmação | Agenda exclusão com regra explícita para backup e retenção. |
| `POST /auth/cloud-consent` | Sessão recente e versão do documento de nuvem | Adesão explícita; sem ela, `/api/cloud/sync` recusa o envio. |
| `POST /api/cloud/sync` | Cursor e lote de alterações | Somente na fase de nuvem; aplica regras Go e retorna cursor do servidor. |

`POST /auth/confirm` e reset recebem token de um link próprio. A página remove
token da URL imediatamente, usa `Referrer-Policy: no-referrer`, não contém scripts
de terceiros e não registra a URL. O token tem escopo limitado: recuperar senha
não pode virar uma sessão completa antes da conclusão do fluxo.

O Go verifica a identidade no provedor antes de criar uma sessão; não basta
decodificar JWT ou confiar em `user_id`, e-mail ou `owner_id` enviados pela página.
Origem permitida vem de configuração fixa, nunca do cabeçalho Host do pedido.

## Modelo de dados proposto

PostgreSQL separado do SQLite local. UUIDs são identificadores, não autorização.
As tabelas de credenciais, confirmação e recuperação ficam no esquema gerenciado
do provedor; não copiar hashes nem inventar uma segunda tabela de senhas.

| Tabela | Campos essenciais | Restrições |
|---|---|---|
| `auth.users` | Identidade, e-mail, hash e confirmação | Gerenciada pelo provedor; não acessível pela API pública da Rich. |
| `rich.profiles` | `user_id`, `display_name`, `created_at` | PK/FK para identidade; mínimo de dados pessoais; RLS por dono. |
| `rich_private.account_security` | `user_id`, `auth_version`, `blocked_at` | Invalidação das sessões e bloqueio da conta; somente backend. |
| `rich_private.sessions` | `id`, `user_id`, `token_hash`, `provider_session_id`, tokens cifrados, chave de cifra, `auth_version`, `authenticated_at`, `last_verified_at`, `created_at`, `last_seen_at`, `expires_at`, `revoked_at` | Hash único; tokens aleatórios de 256 bits; tabela somente do backend. |
| `rich_private.consents` | `user_id`, `document`, `version`, `accepted_at`, `withdrawn_at` | Aceite versionado e retirada; sem guardar senha, código ou corpo dos pedidos. |
| `rich.sync_records` | `user_id`, `type`, `record_id`, `payload`, `revision`, `deleted_at` | Apenas na fase futura de nuvem; chave composta por usuário, tipo e registro. |
| `rich.sync_state` | `user_id`, `revision`, `minimum_cursor` | Apenas na fase futura de nuvem; contador transacional bloqueado por usuário. |

Exemplo de invariantes SQL, não uma migração para executar:

```sql
PRIMARY KEY (user_id, type, record_id)
UNIQUE (token_hash)
CHECK (expires_at > created_at)
```

`rich.sync_records.payload` conserva o formato de registros do domínio atual,
validado em Go antes de gravar. Centavos continuam inteiros, datas `AAAA-MM-DD`
e exclusões são tombstones para não ressuscitar dados em aparelhos offline.
Limite de lote e tamanho, transações, idempotência e conflitos terão testes de
contrato antes de ligar a nuvem. Não aceitar `revision` do cliente como cursor
de confiança. Consultas sempre incluem a identidade obtida da sessão.

### Caminho de autorização no PostgreSQL

O BFF usa conexão PostgreSQL com papel restrito. Não usa `service_role` para ler
ou gravar registros financeiros e não usa a Data API como atalho à validação Go.
Um cookie opaco não preenche `auth.uid()` automaticamente.

A migração separa os papéis `rich_accounts` e `rich_sessions`, ambos sem login,
superusuário ou `BYPASSRLS`. Logins de implantação receberão apenas o papel necessário.
O papel de perfis não lê sessões; o papel de sessões não lê perfis. Os esquemas
`rich` e `rich_private` não devem ser expostos na Data API. A migração precisa de
um papel administrativo; ele não será a credencial de execução do aplicativo.

Em cada transação de perfis, o Go valida a sessão, escolhe o papel e define
`set_config('rich.user_id', identidadeVerificada, true)`. O último argumento
restringe o contexto à transação. Nunca usar `SET` de sessão persistente num pool.
As políticas têm `USING` e `WITH CHECK`, negam por padrão e usam `FORCE ROW LEVEL
SECURITY`. O cliente não define esse contexto. A mesma regra será necessária nas
tabelas financeiras, com teste usando o papel real da conexão, não só o proprietário.

### Sessão do provedor e revogação

`provider_session_id` liga a sessão Rich à sessão Supabase. Se houver retenção de
tokens, o backend os cifra com AEAD, chave fora do banco e autenticação adicional
vinculada ao usuário e à sessão. O esquema registra `encryption_key_id`; a cifra
e sua rotação ainda precisam ser implementadas e testadas. Nunca gravar token cru
no campo de ciphertext nem derivar a chave de um segredo público.

Cada pedido verifica revogação, bloqueio, `auth_version`, prazo absoluto e ocioso.
Operações sensíveis exigem `authenticated_at` recente, não `last_seen_at`. Reset
incrementa `auth_version` e revoga as sessões Rich antes de responder sucesso;
falha parcial na revogação do provedor continua pendente e não reabre a sessão.
Mudanças feitas diretamente no painel Supabase precisam de revalidação: não
prometer revogação imediata enquanto não houver integração comprovada. Definir
e testar esse limite é um bloqueio de lançamento do cadastro.

### Protocolo da nuvem, antes de implementá-lo

Reaproveitar o protocolo de `internal/store/records.go`: transação por lote,
contador em `rich.sync_state` bloqueado por usuário, atualização e revisão no mesmo
commit. Não usar uma sequência global como garantia de ordem de confirmação.
Conflito reutiliza as regras atuais do Go, incluindo desempate determinístico.
Retorno paginado contém cursor e `more`; reconhecer apenas o lote confirmado.
Repetir após timeout é idempotente, não duplica registros. Tombstones e eventos
não serão apagados sem regra de cursor mínimo e resincronização completa para
aparelhos antigos. Não há tabela separada de eventos sem necessidade demonstrada.
Esses contratos exigem testes de concorrência, commit fora de ordem, retry e
cursor vencido antes de criar as tabelas financeiras.

## Segurança obrigatória

- Cadastro sem MFA exige ao menos 15 caracteres, aceita gerenciadores, Unicode e
  espaços, e rejeita senhas acima de 72 bytes por causa do limite atual do provedor.
  Não truncar nem criar regras cosméticas de composição. Contas existentes não
  ficam impedidas de entrar apenas por terem senha menor que a nova política.
- Confirmar e-mail antes de abrir dados. Recuperação com prazo curto e uso único;
  mesmo fluxo externo para conta existente e inexistente.
- Cookie `__Host-`, `Secure`, `HttpOnly`, `Path=/`, sem `Domain`; política SameSite
  compatível com o fluxo e verificação CSRF nos pedidos que alteram estado.
- Sessão opaca e revogável. Manter o bloqueio atual de 30 minutos por inatividade;
  definir também prazo absoluto no servidor, não só no relógio do navegador.
- Limites persistentes por conta e origem de tráfego, proteção de login/cadastro/
  recuperação, atrasos progressivos e CAPTCHA proporcional ao abuso. Memória
  de uma instância serverless não constitui limite global.
- Não salvar senha ou refresh token em localStorage/IndexedDB. No site, refresh
  tokens do provedor ficam só no backend, cifrados se a integração exigir retenção.
- Proteger rotação concorrente, logout e reentrada. Revogar no provedor e na Rich
  conforme o escopo; não prometer que apagar um cookie revoga todos os aparelhos.
- Dados locais isolados por ID estável, locks e canal de abas também por identidade.
  Um usuário novo não pode ver nem enviar a fila do anterior. Migração antiga
  exige escolha explícita e backup; nunca apagar banco ou fila para corrigir login.
- Uma identidade ativa por origem. Um canal global avisa troca de sessão, interrompe
  sync antigo e invalida respostas atrasadas. Cada envio informa a identidade
  esperada; o BFF a compara com a identidade da sessão e recusa divergências. Esse
  campo detecta troca de conta, mas nunca concede acesso. Cookies são compartilhados
  pelas abas: separar apenas o nome do banco não resolve esse risco.
- Sair e apagar dados são decisões diferentes. Sem rede, bloquear o acesso local,
  preservar a fila isolada e oferecer exportação ou descarte confirmado. Não alegar
  revogação remota enquanto o servidor não a confirmar; falha de sync não obriga a
  pessoa a continuar autenticada nem autoriza apagar dados pendentes.
- CSP, encoding de saída, HTTPS, corpo limitado, timeout, consultas parametrizadas,
  logs sem dados financeiros ou credenciais. Não registrar tentativas com senha.
- Exportação, exclusão, backup, restauração testada e retenção documentada antes
  de abrir armazenamento financeiro remoto. Não anunciar criptografia ponta a
  ponta: ela não existe apenas por usar HTTPS e banco cifrado em disco.

## Interface e etapas de entrega

1. Corrigir o erro atual do Drive com mensagens específicas e testes, sem usar
   o novo cadastro como atalho para ignorar a sincronização que falhou.
2. Aprovar provedor, localização dos dados, orçamento e serviço de e-mail.
3. Implementar backend de identidade, sessões persistentes e testes de abuso.
4. Isolar o banco local por identidade e testar duas contas em duas abas.
5. Implementar telas Entrar, Criar conta, Confira seu e-mail e Recuperar senha,
   com os componentes Rich, labels visíveis, autocomplete e erros acessíveis.
6. Implementar e testar a nuvem, se aprovada; revisar privacidade antes de habilitar.
7. QA em 390/768/1440/1920, claro/escuro, Chromium/WebKit; Go/Wasm e revisão por agentes.
8. Publicar por PR, aguardar checks e só então solicitar merge/ativação.

### Bloqueios reais para produção

Projeto Supabase, SMTP próprio com domínio verificado, segredos no ambiente,
política de retenção, escolha do armazenamento e aceite da mudança de privacidade.
O SMTP padrão do Supabase não é um serviço de envio para clientes de produção.
Nenhum serviço pago ou migração real foi iniciado por este documento.

O cliente HTTP inicial em `internal/identity` não cria uma sessão Rich, não libera
o app e não substitui o isolamento local. A existência da tabela de sessões também
não comprova cifra, revogação, CSRF ou autorização de uma API que ainda não existe.

## Evidências desta base

- `go test -race -cover ./internal/identity`: passou, com 96,7% de cobertura.
  Provedor simulado em HTTPS; nenhum usuário real criado. Senha não é truncada,
  redirects são recusados e o cliente não retorna tokens ao chamador.
- `sh scripts/test-identity-db.sh`: passou em PostgreSQL 16 real e descartável,
  incluindo papéis sem acesso, isolamento entre perfis e limpeza do contexto
  depois da transação. Nenhum banco pessoal ou remoto foi usado.
- `make test`: passou, incluindo Go/Wasm; `scripts/check-licenses.sh`: passou.
- `make e2e`: as 16 suítes Chromium passaram no estado final desta base.
- Núcleo WebKit: 15 suítes passaram. Regressões direcionadas Drive: 213
  verificações em Chromium e 213 em WebKit, com 32 capturas dos dois estados
  de erro em quatro larguras e dois temas. Revisão visual corrigiu o alinhamento
  do botão de reconexão; o contraste medido foi 7,17:1.
- Inspeção de produção isolada confirmou a solicitação de `drive.appdata` e PKCE,
  não o acesso real ao Drive. O motivo do 403 da conta pessoal continua pendente
  de identificação; não foi usado como justificativa para ampliar os escopos.
- Revisão independente aprovou esta base não ativada, não o lançamento do cadastro.

Bloqueios preexistentes que o novo fluxo não pode herdar: o banco Google atual
não é particionado por identidade e o prazo ocioso atual é um bloqueio local,
não uma sessão revogável de servidor. Não anunciar isolamento multiusuário ou
cadastro seguro concluído antes de substituir esses contratos e testar duas
contas, abas concorrentes e respostas atrasadas.

Como preparar o projeto: [SUPABASE-CONFIGURACAO.md](SUPABASE-CONFIGURACAO.md).
Políticas e contratos antiabuso: [LIMITACAO-DE-TAXAS.md](LIMITACAO-DE-TAXAS.md).

## Referências primárias

- [Autenticação por senha](https://supabase.com/docs/guides/auth/passwords).
- [SMTP de produção](https://supabase.com/docs/guides/auth/auth-smtp).
- [Checklist de segurança](https://supabase.com/docs/guides/security/production-checklist).
- [RLS](https://supabase.com/docs/guides/database/postgres/row-level-security).
- [Regras de autenticação OWASP](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html).
- [Erros do Drive](https://developers.google.com/workspace/drive/api/guides/handle-errors).
