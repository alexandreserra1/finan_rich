# Open Finance: cada pessoa com o seu

O Open Finance do finan_rich é por pessoa. Não existe um servidor nosso que busca os bancos de
todo mundo: cada instalação busca só os bancos de quem a usa, com a conta dessa pessoa na Pluggy.

## Onde roda

- **No computador:** o programa da pessoa (`finanrich`) busca direto na Pluggy. Enquanto ele
  estiver aberto, busca sozinho a cada 6 horas, mesmo com o navegador fechado
  (`Server.WatchOpenFinance` em `internal/web/openfinance.go`).
- **No app:** com o Open Finance configurado no próprio app, o celular busca direto na Pluggy, a
  cada 6 horas com o app aberto. Sem credenciais no app, mas pareado, ele pergunta ao computador.
- **No site** (finan-rich.vercel.app): não tem Open Finance.

Nada passa pela Vercel nem por outro servidor do finan_rich. Os lançamentos sempre passam pela
revisão antes de entrar.

## Credenciais

Cada pessoa:

1. cria a própria conta grátis em [dashboard.pluggy.ai](https://dashboard.pluggy.ai);
2. conecta os próprios bancos em [meu.pluggy.ai](https://meu.pluggy.ai);
3. cola o Client ID, o Client Secret e os Item IDs em **Ajustes → Open Finance**.

No computador, as credenciais ficam na tabela `settings` do banco de dados dele. No app, o Client
ID e os Item IDs ficam no banco do aparelho e o Client Secret no cofre do sistema.

Cada instalação tem um conjunto de credenciais, ou seja, uma pessoa. Ela pode conectar vários
bancos (vários Item IDs), todos da mesma conta Pluggy.

## Custo e limites

Para o finan_rich, o custo é zero. Os limites da Pluggy valem para a conta de cada pessoa; uma
busca a cada 6 horas fica bem abaixo do que a conta grátis permite.

## Por que não um "conectar banco" com um clique

A desvantagem do modelo atual é que cada pessoa faz o cadastro na Pluggy sozinha (uns 10
minutos, passo a passo no [README](../README.md)).

Um botão de um clique para todos os clientes exigiria:

- um servidor nosso sempre ligado;
- um contrato pago com a Pluggy;
- os dados bancários de todo mundo passando por esse servidor.

Isso vai contra duas regras da casa: só usar o que é grátis, e os dados não passarem por servidor
do finan_rich. Por isso fica como está.
