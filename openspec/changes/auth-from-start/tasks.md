## 1. Autenticação
- [x] 1.1 TOTP e códigos de recuperação em `internal/auth`, com os vetores da RFC 6238.
- [x] 1.2 Login pela rede em dois passos (senha, depois o código), com limites separados.
- [x] 1.3 Ligar o acesso pelo celular já oferece o código; Ajustes com QR, confirmação e códigos de recuperação.
- [x] 1.4 Reautenticação pela rede para trocar a senha, desligar o acesso e ligar/desligar o código.
## 2. Autorização
- [x] 2.1 Tabela de rotas com o nível de cada uma; `authorize` nega o nível zero.
- [x] 2.2 Teste da matriz (quem pede × nível) e das regras da tabela.
## 3. Celular
- [x] 3.1 Primeiro uso oferece o bloqueio do app; o link abre o formulário.
- [x] 3.2 Backup do Android só cifrado, com teste das regras.
## 4. Dados
- [x] 4.1 Backup `.db` sem o segredo do código.
## 5. Pendente
- [ ] 5.1 Conferir o backup do Android num celular de verdade.
  - Emulador (Android 14, 22/11/2026): com o transporte local sem cifra, o backup da Rich sai vazio e é
    recusado (0 bytes); marcado como cifrado (`is_encrypted=true`), leva 4,5 MB com os dados. Falta o
    backup do Google num celular com bloqueio de tela.
- [ ] 5.2 Banco do celular cifrado (decisão de 18/11: depois, com teste no aparelho).
