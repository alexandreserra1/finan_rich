## Context

Os dados ficam no aparelho e no computador da pessoa; o único lugar com várias formas de entrar
é o computador servindo as telas pela rede. O backup é a única cópia que costuma sair dele.

## Decisions

- Backup: `VACUUM INTO` para um arquivo intermediário, limpa os segredos e as tabelas de acesso
  ali, e `VACUUM INTO` de novo para o arquivo final. O segundo `VACUUM` reconstrói as páginas
  só com o que sobrou, então o texto apagado não fica no arquivo (o teste procura nos bytes).
- Registro de acesso guarda um texto curto em português ("Senha errada", "Aparelho pareado:
  Android"), a data e o endereço (vazio neste computador; o login do Tailscale quando vier por
  ele). Teto de 200: o mais antigo sai. As recusas do limitador não entram, para uma rajada não
  empurrar os eventos verdadeiros para fora.
- Gravar no registro nunca muda a resposta do pedido.
- CodeQL com `build-mode: manual` no Go: o `finance.wasm` precisa existir antes do `go build`
  (`go:embed`). Bibliotecas copiadas e telas geradas ficam de fora.
- MFA, backup automático do Android, banco do celular cifrado e chaveiro do computador mudam
  login, dados ou dependências: ficam como decisões pendentes em `docs/SEGURANCA.md`.

## Validation

`TestBackupLeavesSecretsOut`, `TestAccessLog`, `TestAccessLogInSettings` e o CodeQL no PR.

## Risks

- O registro não é à prova de adulteração: quem controla o computador controla o banco.
- Backup novo restaurado pede a chave da IA, o segredo do Open Finance e um novo pareamento.
