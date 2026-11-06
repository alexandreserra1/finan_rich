# Rich: entender o dinheiro sem perder tempo

Plano de 05/11/2026. Pedido: deixar a experiência fácil sem tirar o controle dos dados.
Este documento orienta a execução e registra o que foi conferido.

## Objetivo

Responder, nesta ordem: quanto entrou e saiu, o que vence e o que merece uma ação.
Os números continuam no Go e funcionam sem IA. O Consultor explica e prepara propostas;
nenhuma conversa passa a ser necessária para consultar o próprio dinheiro.

## O que já existe

- Go: saldo do mês, contas e cartões, orçamento diário estimado, projeções, alertas,
  resumos do time, metas e dívidas.
- Importação: arquivo, busca bancária configurada pela pessoa e notificações Android,
  sempre com revisão antes de gravar.
- Consultor: perguntas prontas sem IA e propostas que só entram depois da confirmação.
- Proteção: login obrigatório no site, sessão, escape de texto, CSP, armazenamento local
  e sincronização. Nada disso será retirado.

## Módulos e ordem de execução

| Módulo | Entrega | Depende de |
|---|---|---|
| início | Resumo enxuto, estimativa explicada, semana e até três insights úteis | regras Go existentes |
| navegação | Início, Movimentos, Análise e Mais no celular; funções preservadas | template atual |
| registro | Formulário curto que prepara uma mensagem no Consultor, sem enviar sozinho | Consultor atual |
| entrada de dados | Atalhos para registro manual e importação com revisão; primeiro uso sem obrigar quatro passos | fluxos atuais |
| validação | Regras, navegador, acessibilidade e revisão de segurança | módulos anteriores |

Implementar as regras e o Início; integrar navegação e registro; conferir tudo antes de publicar.
As partes independentes podem ser feitas em paralelo. Dois agentes não editam o mesmo arquivo.

## Decisões de produto e proteção financeira

- Saldo do mês não é saldo bancário nem dinheiro livre. Os rótulos continuam distintos.
- A estimativa existente desconta reserva planejada, gastos e recorrências. Ela não vira
  promessa de saldo disponível. Sem renda suficiente para estimar, mostrar o motivo.
- Com dívida ativa ou cartão em aberto, não promover essa conta como limite para gastar:
  pedir revisão dos compromissos, sem descontá-los novamente nem inventar saldo seguro.
- Bloquear também renda futura misturada com recebida, fatura futura pendente e saldo
  cadastrado negativo. Crédito em outro cartão não libera a referência de um cartão devedor.
- Vencimentos são previsões cadastradas, não prova de que algo continua sem pagamento.
  Mostrar somente valores pendentes da fatura; recorrências e dívidas mantêm esse aviso.
- O resumo dos últimos sete dias conta lançamentos registrados, sem transferências entre
  contas. Os próximos sete dias aparecem separados como compromissos previstos.
- Os três insights priorizam avisos importantes e reaproveitam o time, sem três cartões
  de apresentação nem mensagens vazias. Cada insight leva a uma ação existente.
- O gráfico anual sai do Início, não os dados. Análise, relatórios e exportações continuam.
- Metas só ocupam espaço quando existem. Não apagar telas, rotas ou registros.

## Registro e privacidade

- Texto livre do registro rápido fica apenas na memória da aba e segue ao Consultor sem
  query na URL, localStorage, histórico de conversa ou requisição de IA automática.
- Usar o formulário manual continua possível, inclusive sem IA ou internet.
- Importar continua abrindo a revisão existente. Não coletar extratos, notificações ou
  credenciais automaticamente e não alterar as permissões do Android.
- Nomes, descrições e mensagens são texto escapado. Não montar HTML com conteúdo da pessoa.
- Não adicionar serviços, bibliotecas, métricas externas ou rastreamento.

## Critérios de aceitação

- [x] Regras novas de semana, priorização e disponibilidade da estimativa em Go, com testes.
- [x] Início com dinheiro, próximos compromissos, até três insights e movimentos recentes.
- [x] Explicação acessível das estimativas; nenhum valor apresentado como dinheiro livre garantido.
- [x] Navegação principal simplificada, mantendo todas as funções no Mais/sidebar.
- [x] Registro rápido sem dados na URL, sem envio automático e sem dispensar confirmação.
- [x] Primeiro uso com opções diretas de importar, registrar e seguir o guia opcional.
- [x] Resumo de sete dias e vencimentos em Go, sem IA nem alteração dos dados.
- [x] Claro/escuro, celular, teclado, texto longo e conteúdo semelhante a HTML conferidos.
- [x] `make test`, testes de navegador e revisões de código, testes e segurança aprovados no escopo.
- [x] Documentação atualizada; publicação inclui somente os arquivos desta tarefa.

## Orquestração

- Sessão principal: plano, regras Go, Início e integração.
- `code-reviewer`: navegação no template, depois revisão final sem editar arquivos da integração.
- `test-engineer`: suíte da experiência simples e adaptação dos testes afetados.
- `security-auditor`: fronteiras dos dados, texto livre, sessão e confirmação das alterações.

## Não entra nesta entrega

Voz, importação automática sem revisão, detecção nova de aumento de assinatura, notificações
semanais novas e cálculo universal de caixa disponível. O resumo de sete dias aparece no
Início; os lembretes atuais continuam. Esses extras precisam de requisitos e testes próprios.
Também não serão alterados autenticação, migrações, segredos, infraestrutura ou o protótipo local.

## Verificação final

- `make test`: Go, vet, formatação e regras como WebAssembly aprovados.
- `go test -race ./internal/finance ./internal/wasmapi`: aprovado.
- `PORT_OFFSET=1200 make e2e`: doze suítes Chromium aprovadas.
- `PORT_OFFSET=2200 npm run webkit` em `e2e`: onze suítes WebKit aprovadas.
- Depois do ajuste final de ordem e espaçamento, `simple.test.mjs` repetido nos dois
  motores: aprovado. Sem rolagem horizontal em 320 px, inclusive com a explicação aberta.
- Testes negativos: HTML malicioso não executa; rascunho não vai para URL, armazenamento
  ou IA; não sobrescreve mensagem existente; permanece acessível sem IA e some ao recarregar.
- Testes de regras: renda futura e mista, cartão com crédito compensatório, fatura futura,
  pagamento parcial, mês curto, mudança de mês, registros apagados e leitura sem mutação.
- `scripts/check-licenses.sh`: aprovado; nenhuma dependência nova.
- Capturas conferidas: computador claro e celular escuro. Interface existente preservada.
- Revisões `code-reviewer` e `security-auditor`: GO após corrigir renda futura e rascunho
  sem IA. A revisão de segurança vale para estas mudanças, não para o produto inteiro.
- Publicar na `main` e acompanhar a matriz Chromium/WebKit do CI antes de encerrar.

Nenhum teste de navegador substitui a conferência posterior em iPhone e Android reais.

## Sessão vencida (corrigido em 05/11/2026)

**Era média, preexistente:** `google.markActive()` podia renovar uma sessão já vencida quando
alguém clicava ou digitava numa aba ainda aberta, antes da checagem periódica.

Agora a hora do último uso é lida e renovada numa transação só do banco do navegador
(`db.siteSession`), e sessão vencida não renova: só um login novo. No site, gravar
lançamentos confere a sessão na mesma transação; vencida, nada é gravado. Clique, tecla ou
envio depois do prazo é barrado até o banco confirmar a sessão. A sincronização não conta
como uso. Testes: `e2e/session.test.mjs` (banco real, Chromium e WebKit) e os casos de
clique, tecla e outra aba em `e2e/google.test.mjs`.

Em computador compartilhado, sair da conta continua sendo o certo: os 30 minutos sem uso
são uma rede de proteção, não a proteção principal.
