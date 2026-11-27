# Preparativo da avaliação reservada (5.2.6)

Estado: preparativo, não avaliação concluída. Nenhum modelo foi executado nesta entrega.
O passo 5 permanece aberto. Treinamento e execução reservada final aguardam as correções
do app pelo Claude, o aviso da pessoa e a quarta revisão independente. Este documento
não autoriza coleta, treino, download, publicação ou envio de dados.

## O que existe

`reservedeval` é um comparador local, sem dependências novas e sem código de rede.
O padrão calcula a resposta **real do app**, sem provedor: se `advisor.Prepared` aceitar
a pergunta, chama `advisor.Answer` com provedor nil; caso contrário, chama `advisor.Offline`.
Essa resposta é uma candidata avaliada, nunca a verdade usada para construir critérios.

O modo explícito `replay` compara também respostas previamente coletadas. Ele chama o
`advisor.Answer` real com `AskAI: true` e um Provider fake que reproduz cada rodada coletada.
O pedido reconstruído precisa ter o SHA-256 registrado naquela rodada. Ferramentas são
as reais do advisor, sobre um livro sintético em memória; propostas não são persistidas.
Não há banco, escrita no corpus ou mudança em `internal/`.

O replay é autenticado **por vínculo de hashes**, não por assinatura ou atestação do Ollama.
Ele verifica consistência entre casos, critérios, manifesto, variante e pedidos. Não prova
que alguém realmente executou o modelo declarado. Arquivos fabricados também podem ter
hashes consistentes. A coleta original e sua revisão continuam indispensáveis.

Variantes fixas: `producao` mantém `Request.System`; `cauteloso` acrescenta uma instrução
curta de cautela. Só o wrapper local `systemWrapper.Chat` altera esse campo. Mensagens,
ferramentas e fluxo de produção continuam intactos. Não existe seleção automática.

## Entradas estritas, fora do Git

Execute da raiz do checkout. Entradas devem existir dentro de `.artifacts`, inclusive
após resolver links. Não use dados reais. O envelope exige `synthetic: true`; isso é
uma declaração obrigatória, não um detector de dados pessoais. O comparador não imprime
perguntas, respostas, livros ou erros brutos do provedor.

Fixtures são **um objeto JSON**, não JSONL, com estas propriedades:

| Campo | Contrato |
|---|---|
| `synthetic` | Obrigatoriamente `true` |
| `cases` | Lista de 1 a 100 casos |
| `cases[].id` | Identificador único, `[a-z0-9][a-z0-9_-]{0,63}` |
| `family` | Mesmo formato; uma família não pode cruzar `valid` e `test` |
| `split` | Somente `valid` ou `test`; nunca `train` |
| `ledger` | Objeto obrigatório, desserializado como `domain.Ledger` |
| `today` | Data real no formato estrito `AAAA-MM-DD` |
| `agent` | `otavio`, `lia` ou `tito` |
| `question` | Texto não vazio, até 4.096 bytes |
| `history` | Opcional; até 12 objetos `{ "role": "user ou assistant", "content": "texto" }` |
| `rates` | Opcional; objeto `rates.Rates` com `selicBP`, `cdiBP`, `ipcaBP`, `date` e `ipcaMonth` |

Cada mensagem de histórico tem até 4.096 bytes. Campos desconhecidos, chaves repetidas
(inclusive com outra capitalização), JSON adicional, profundidade maior que 32, `ledger`
ausente/null e `app_answer` são rejeitados. Campos do livro seguem os tipos Go, inclusive
dinheiro inteiro em centavos. O limite por livro é 64 KiB. O comparador não substitui
a revisão da coerência financeira, das referências, dos valores ou das datas do livro.
As taxas são inteiras em pontos-base. Quando preenchidos, `rates.date` e `rates.ipcaMonth`
devem ser data e mês reais nos formatos estritos `AAAA-MM-DD` e `AAAA-MM`. O mesmo objeto
é passado em `Question.Rates` para o app e para a IA em replay, incluindo casos positivos
de Selic com data. Nenhuma taxa é buscada na rede ou inferida da resposta do app.

As regras são outro objeto JSON, definido e revisado **antes de qualquer inferência**:

- `synthetic: true` e `fixtures_sha256`: SHA-256 dos bytes exatos da fixture.
- `cases`: exatamente uma entrada por caso, com `id` e `criteria`.
- Cada critério tem `id`, `category` e **somente um** entre `contains`, `forbids`, `calls` e `proposals`.
- IDs e categorias seguem o formato dos identificadores. Há 1 a 20 critérios por caso.
- `contains`/`forbids` são substrings literais, sensíveis a maiúsculas, com até 512 bytes.
  Não há regex, normalização, tolerância monetária ou julgamento por outro modelo.
- `calls` declara a sequência completa de chamadas, com nome, argumentos JSON exatos e
  `error: true` quando o executor deve rejeitar. A lista vazia afirma que não deve chamar.
- `proposals` declara a sequência de cartões, ação, tipo e campos do registro esperado;
  edições incluem `before` e ID existente. Registros são comparados por campos declarados
  para não exigir IDs aleatórios novos, mas os critérios precisam incluir valores e dados
  financeiros importantes. Lista vazia afirma que não deve haver cartão. `Applied` nunca é aceito.
- Um fluxo com chamadas/propostas ou com qualquer critério estruturado exige critérios
  para **ambas** as listas, inclusive ausência explícita. Omissão reprova como
  `rubrica_incompleta`, sem premiar apenas a frase final. Em caso somente textual sem
  emissão de ferramenta, detectar uma chamada omitida pelo modelo ainda exige rubrica
  de chamadas definida por quem revisa o caso; não é inferido da frase da pessoa.

Um critério de texto não certifica correção financeira. Categorias e critérios finais,
seus denominadores e a rubrica humana ainda precisam de revisão. Não foram criados casos
financeiros gold para fazer esta preparação parecer uma avaliação concluída.

## Conversão do auditgen para fixtures

O JSONL de auditoria do gerador **não é entrada direta** deste comando. Seu esquema inclui
campos como `messages`, `tools`, `profile`, `review` e `app_answer`, rejeitados pela fixture.
O adaptador/conversor ainda não foi implementado; não use o JSONL renomeado para `.json`.

Protocolo para a conversão futura, após a fonte do app estar corrigida e revisada:

1. Fixar o hash do pacote original e preservar o diretório inteiro em `--reserved-dir`.
   O comparador confere os bytes de todos os arquivos antes e depois, sem regenerar nada.
2. Selecionar somente exemplos sintéticos `valid`/`test`, preservando IDs, famílias e splits.
   Rejeitar família compartilhada entre os dois. Não mover itens de treino para o reservado.
3. Copiar explicitamente `id`, `family`, `split`, `ledger`, `today`, `agent` e `question`.
   Os campos de auditoria `ledger`/`today` precisam existir e representar o estado inicial
   do caso, não o estado após uma proposta. Se faltarem, parar e pedir nova exportação.
   Não reconstruir dinheiro ou datas a partir de textos do prompt ou de `app_answer`.
   Quando houver `rates` na auditoria, copiar o objeto explicitamente e preservar seus
   pontos-base, data e mês. Não preencher taxas ausentes nem consultar o Banco Central.
4. Copiar `history` apenas se houver histórico inicial explícito. Caso contrário, revisá-lo
   manualmente ou deixar ausente quando o caso realmente não tiver histórico. Não copiar
   o transcript inteiro: contém a pergunta atual, dados financeiros, ferramentas e respostas.
5. Escrever o envelope próprio `{ "synthetic": true, "cases": [...] }` em arquivo novo
   de `.artifacts`. Não copiar `messages`, `tools`, respostas finais ou `app_answer`.
6. Revisores definem critérios independentes por caso, conferindo cálculos e comportamento
   esperado sem usar a resposta do app como oráculo. Gravar regras novas com o hash exato
   da fixture. Registrar fora do corpus o hash original, o convertido e a decisão de revisão.
7. Congelar fixture/regras antes da coleta. Qualquer edição invalida o replay e a seleção.
   Ajustes de wrapper são decididos somente em `valid`. Teste final não pode orientar ajustes.

## Coleta real ainda pendente

Não há flag de inferência nesta versão. `--mode infer` é rejeitado. Não são chamados
`llm.NewOllama`, APIs externas, pull, uploads, treino ou comandos do Ollama.

Um coletor futuro deverá usar `llm.NewOllama("http://127.0.0.1:11434", baseline)` e
o mesmo `advisor.Answer`/wrapper, com autorização explícita e prazos. Somente
`qwen2.5:7b-instruct`, digest
`845dbda0ea48ed749caafd9e6037047aa19acfcfd82e704d7ca97d631a0b697e`, já fixado em
`models.json`, é aceito. Deve conferir o digest instalado e a ausência de origem remota,
bloquear redirects/proxies externos e não baixar modelos. Outros modelos do manifesto
não são candidatos deste comparador.

O coletor deverá gravar objeto JSON novo com `synthetic`, `model`, `digest`, `endpoint`,
`fixtures_sha256`, `rules_sha256`, `variant` e `cases`. Cada caso tem `id` e `rounds`.
Cada rodada tem:

- `request_sha256`: SHA-256 de `json.Marshal(llm.Request)` **depois** do wrapper local.
  Este é o formato Go, não o JSON HTTP do Ollama. Não remonte esse hash pelo JSONL.
- `complete`: verdadeiro apenas depois de confirmar fim íntegro do stream.
- `error`: verdadeiro se houve erro; não armazenar erro bruto com dados.
- `reply`: `llm.Reply`, com as propriedades Go `Text` e `ToolCalls`; chamadas usam
  `ID`, `Name`, `Args` (objeto JSON) e `Extra`, conforme os structs existentes.

Preservar todas as rodadas e seus pedidos, inclusive ferramentas e novas tentativas.
O adaptador Ollama atual não comprova término íntegro por si só: `complete` não pode
ser inferido apenas de um retorno sem erro. A coleta precisará dessa verificação antes
de ser aceita como evidência. Nenhum coletor nem esse gate de stream foi implementado aqui.

## Execução sem rede

Depois de preparar e revisar as entradas, o modo padrão é apenas app:

```sh
go run ./tools/ai-lab/reservedeval \
  --fixtures .artifacts/reservado/cases.json \
  --rules .artifacts/reservado/rules.json \
  --reserved-dir .artifacts/dataset-reservado --split valid
```

Replay opt-in de coleta prévia, sem executar o modelo:

```sh
go run ./tools/ai-lab/reservedeval --mode replay --allow-collected \
  --fixtures .artifacts/reservado/cases.json \
  --rules .artifacts/reservado/rules.json \
  --reserved-dir .artifacts/dataset-reservado --split valid --variant producao \
  --collected .artifacts/coleta/producao.json
```

Para `test` em replay, indicar também `--selection CAMINHO_DO_REPORT_VALID` e a variante
escolhida manualmente em valid. Exige mesmos hashes de fixture/regras (que incluem os dois
splits), reservado e código. O relatório valid não precisa ter pontuação perfeita: fixa
o contexto e a escolha, não libera o modelo. Não há busca de melhor wrapper no teste.
Esse vínculo não impede uma pessoa de examinar o teste fora do comando; a separação
também depende do procedimento humano. Não execute agora a avaliação reservada final.

## Resultados, gates e limites

A saída é sempre nova: `.artifacts/ai-lab-reserved-runs/run-*/report.json`, diretório 0700,
arquivo 0600 com criação exclusiva. Não sobrescreve relatório nem escreve no reservado.
O relatório contém hashes de entradas, código e dataset antes/depois; versão Go, prazos,
split, wrapper, acerto/falha por critério e categoria, totais por fonte e categorias.
Textos de respostas são substituídos por hashes. A fonte `modelo` significa **replay de
coleta declarada**, não inferência executada pelo comparador.

Todos os casos do split entram no denominador. Coleta ausente, erro, stream incompleto,
pedido divergente, rodada extra, resposta vazia ou fallback reprovam todos os critérios
da resposta do modelo. A resposta de fallback do app nunca conta como acerto da IA.
Falhas de ferramenta conhecidas também invalidam o replay; novas mensagens de erro do
app exigem revisão do comparador. Há até 8 rodadas coletadas, 8 chamadas por rodada,
4.096 bytes por argumento e 16 KiB de texto por resposta. Não há avaliação semântica
automática da proposta ou garantia de ausência de outros erros do executor.

Prazos: 5 segundos por resposta e 1 minuto global; configuráveis até 30 segundos e
5 minutos, respectivamente. Cancelamento por contexto é cooperativo; cálculos síncronos
do app não recebem interrupção forçada. Cada arquivo tem até 8 MiB; reservado tem até
200 arquivos regulares e 32 MiB, sem links internos. Prazo global vencido ou mudança
de hash aborta a publicação do relatório; falha de critério publica relatório e retorna 1.
Os hashes detectam diferenças entre as leituras, não substituem isolamento contra escrita
concorrente que altere e restaure bytes entre elas. Pare outros escritores para a execução final.

Gates locais, sem modelo real:

```sh
go test ./tools/ai-lab/reservedeval
node scripts/engineering.mjs check affected
```

Testes usam livros vazios e texto de engenharia, não referências financeiras aprovadas.
Fonte do app ainda pendente: resultados futuros precisam registrar os novos hashes após
as correções do Claude. Coleta Ollama real, conversor auditgen, casos/rubrica finais,
revisão independente e avaliação reservada final continuam fora desta entrega.
