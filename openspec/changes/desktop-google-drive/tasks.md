## 1. Drive em Go
- [x] 1.1 `internal/gdrive`: achar, baixar e gravar o arquivo; erros do Drive com os motivos do Google.
## 2. Login e sincronização no programa
- [x] 2.1 Rotas `/google/entrar` e `/google/retorno` (só neste computador), `/google/sincronizar` e `/google/sair`.
- [x] 2.2 Token: troca e renovação pelo intermediário; renovação recusada desliga a conta.
- [x] 2.3 Rodada de sincronização e `WatchGoogle` (2 min e depois do `/api/sync`).
- [x] 2.4 Backup do banco sem o token de renovação.
## 3. Tela e documentação
- [x] 3.1 Seção "Conta Google" nos Ajustes do computador.
- [x] 3.2 GOOGLE.md, privacidade, SEGURANCA.md, README.
## 4. Validação
- [x] 4.1 Testes Go com Google e Drive falsos; `TestRouteTable`; backup.
- [x] 4.2 Ajustes abertos no navegador (claro e escuro, computador e celular), sem e com conta.
- [ ] 4.3 Login real com a conta do dono (depende do dono).
