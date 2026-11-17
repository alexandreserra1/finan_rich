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

1. confere a elegibilidade do uso pessoal no [guia oficial](https://docs.pluggy.ai/en/docs/guides/meu-pluggy-personal-use);
2. conecta os próprios bancos em [meu.pluggy.ai](https://meu.pluggy.ai);
3. vincula o conector **MeuPluggy** no Dashboard, conforme o guia, para autorizar a leitura;
4. cola o Client ID, o Client Secret e os Item IDs em **Ajustes → Open Finance**.

No computador, as credenciais ficam na tabela `settings` do banco de dados dele. No app, o Client
ID e os Item IDs ficam no banco do aparelho e o Client Secret no cofre do sistema.

Cada instalação tem um conjunto de credenciais, ou seja, uma pessoa. Ela pode conectar vários
bancos (vários Item IDs), todos da mesma conta Pluggy.

## Custo e limites

Consulta às fontes em **16/11/2026**. Não há gratuidade comercial multicliente confirmada.

O [guia pessoal da Pluggy](https://docs.pluggy.ai/en/docs/guides/meu-pluggy-personal-use)
oferece até cinco conexões, atualização bancária a cada 24 horas e uso não comercial.
A autorização de leitura do MeuPluggy dura um ano, separada do consentimento de cada banco.
Consultar a API a cada seis horas não faz o banco atualizar seus dados nesse intervalo.

A [página comercial](https://www.pluggy.ai/precos) anuncia Dados a partir de R$ 2.500 por mês,
com até 500 conexões ativas e teste de 15 dias. Conexões não equivalem necessariamente a
clientes. A página também descreve possibilidade técnica de uso do conector gratuito em
produto sem SLA; isso não resolve a restrição comercial explícita do guia. Precisamos de
autorização escrita para o nosso modelo, não presumir permissão pela possibilidade técnica.

## Por que não um "conectar banco" com um clique

A desvantagem do modelo atual é que cada pessoa faz o cadastro na Pluggy sozinha (uns 10
minutos, passo a passo no [README](../README.md)).

Um botão integrado para clientes exige definir e validar:

- participante/integrador responsável e contrato que permita o uso do produto;
- jornada de consentimento, autenticação no banco e revogação;
- cobertura de bancos, custo, limites, frequência, retenção e suporte;
- onde ficam tokens e dados, isolamento por usuário e quem pode acessá-los.

Centralizar todos os dados em servidor nosso é uma escolha de arquitetura, não uma
consequência universal do Open Finance. O desenho depende do contrato e da integração.
Nada foi contratado nem alterado em produção nesta pesquisa; o adaptador Pluggy permanece.

## Por que não trocar Pluggy por uma API chamada Open Finance

Open Finance é o ecossistema de compartilhamento, não um provedor substituto. APIs de
dados abertos descrevem produtos e tarifas; não entregam extrato pessoal. Sandboxes usam
dados sintéticos e não substituem acesso a transações reais.

O caminho direto exige enquadramento como participante autorizado, diretório, certificados
e conformidade. Para um aplicativo não participante, o caminho a avaliar é parceria com
instituição participante, incluindo obrigações de consentimento, segurança e tratamento
de dados. Consulte as [normas do BCB](https://www.bcb.gov.br/estabilidadefinanceira/openfinance)
e o [guia de certificação](https://openfinancebrasil.atlassian.net/wiki/spaces/OF/pages/155910145).
O enquadramento e as normas vigentes precisam ser confirmados com a instituição parceira;
não basta obter consentimento e chamar qualquer endpoint bancário.

## Caminho gratuito viável agora

1. Manter a importação **OFX/CSV** existente, com prévia, deduplicação e revisão. A pessoa
   exporta no banco e importa no Rich. Não é sincronização bancária automática.
2. Usar sandbox para desenvolver contratos e testes sem dados ou credenciais reais.
3. Usar MeuPluggy apenas nos termos pessoais autorizados, sem anunciar oferta comercial gratuita.
4. Solicitar proposta e autorização escrita para o modelo descentralizado antes de trocar
   o adaptador. Não criar um backend de tokens ou capturar senha bancária por conta própria.

Para o cliente ter **conectar banco e importar automaticamente**, faltam provedor/parceria
permitidos, consentimento e revogação, armazenamento seguro, idempotência de transações,
reconexão e testes reais autorizados. Esta pesquisa não habilita essas condições nem
promete infraestrutura comercial gratuita.
