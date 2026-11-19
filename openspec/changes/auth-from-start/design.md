## Context

Uma pessoa por instalação. As portas são: o próprio computador, o login pela rede, o token do
app pareado e as rotas públicas (login, arquivos do app, pareamento por código).

## Decisions

- **TOTP em vez de SMS ou chave física.** Funciona sem internet, com qualquer app autenticador,
  e fica na biblioteca padrão do Go (HMAC-SHA1, como os apps esperam). Janela de ±30 s; o último
  passo usado fica guardado, então o mesmo código não entra duas vezes.
- **Login em dois passos sem sessão no meio.** A senha certa gera um bilhete de 5 minutos, só
  na memória e num cookie `HttpOnly` restrito a `/entrar`; a sessão só nasce depois do código.
  Reiniciar o programa apaga os bilhetes (a pessoa digita a senha de novo).
- **Dois limites.** A senha conta por endereço e zera quando está certa. O código tem um limite
  só (10 a cada 10 min) para todos os endereços: quem chega nele já tem a senha, e trocar de IP
  no Wi-Fi é fácil. Quem já tem a senha consegue travar o login pela rede por 10 min; o dono
  continua entrando pelo computador e deve trocar a senha.
- **Ligar o código derruba as sessões abertas pela rede**, que passaram só pela senha; quem
  ligou pela rede continua conectado.
- **Códigos de recuperação** com SHA-256 (aleatórios e longos; as tentativas passam pelo limite).
  Aparecem uma vez, na resposta que liga o código, com `Cache-Control: no-store`.
- **Autorização numa tabela** em vez de regras por prefixo. Cada rota tem um nível; o zero nega.
  O teste confere: o que é público, que só a API aceita o token do app e que só fechar o
  programa é exclusivo deste computador.
- **Neste computador, sem senha.** A conta do sistema já protege; pedir senha aqui não
  protegeria de quem tem a conta e tiraria a saída de quem perdeu o celular.
- **Backup do Android**: `dataExtractionRules` com `disableIfNoEncryptionCapabilities` (12+),
  `requireFlags` em duas regras (9 a 11) e exclusão de tudo no 7 e 8.

## Validation

`TestTOTPRFCVectors` e os outros de `internal/auth/totp_test.go`; `TestAuthorizeMatrix`,
`TestRouteTable`, `TestMFALogin`, `TestSecurityChangesNeedReauth`, `TestEnableLANOffersMFA`;
`TestBackupLeavesSecretsOut`; e2e do app (convite do bloqueio no primeiro uso);
`BackupRulesTest` no Android (CI).

## Risks

- O backup do Android só se confere num celular de verdade (`bmgr`); o CI confere o XML.
- Relógio do celular muito errado: o código não confere. A tela diz para acertar o relógio.
