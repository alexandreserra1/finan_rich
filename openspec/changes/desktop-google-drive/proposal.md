## Why

"Entrar com Google" já funciona no site: os dados vão para a pasta de dados do app no Google
Drive da própria pessoa e aparecem iguais em todo navegador com a mesma conta. O programa do
computador ainda não falava com o Drive. Quem usa o site e o computador ficava com dois
conjuntos de dados separados, e o computador é o que guarda tudo e sincroniza com o celular
pareado. O plano do login com Google (fase 5) previa o computador logo depois do site.

## What Changes

- Ajustes do computador ganham a seção **Conta Google**, com:
  - "Entrar com Google";
  - a conta ligada e a última sincronização;
  - "Sincronizar agora" e "Sair da conta Google".
- O login abre no navegador deste computador. A página `/oauth` do site repassa a volta para
  `127.0.0.1` nesta porta (state `pc.<porta>.*`, que a página já aceitava). O código vem com
  PKCE, e a troca passa pelo intermediário, como no site.
- Enquanto o programa estiver aberto, ele sincroniza com o Drive a cada 2 minutos e logo depois
  que um aparelho manda alterações. A regra é a mesma do site (`drivesync.NeedsUpload`): vence
  o `updatedAt` maior.
- O token de renovação fica na tabela `settings`, como o segredo da Pluggy. Ele não vai no
  backup do banco. O token de acesso fica só na memória.

## Capabilities

### New Capabilities
- google-drive: entrar com Google e sincronizar pelo Drive da pessoa, no programa do computador.

## Impact

`internal/gdrive` (novo: cliente da API do Drive em Go), `internal/web/google.go` (novo: rotas,
token e rodada de sincronização), `internal/web/api.go` (pede uma rodada depois do
`/api/sync`), `internal/store/store.go` (backup sem o token), `cmd/finanrich` (rodada em
segundo plano), Ajustes do computador e docs. Sem dependência nova. O intermediário e o site
não mudam.
