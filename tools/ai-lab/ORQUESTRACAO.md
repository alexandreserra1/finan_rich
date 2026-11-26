# Orquestrador de revisão do corpus

**Resultado vigente: blocked (terceira rodada, 26/11/2026).** Há 11 bloqueantes e 40
avisos. A conclusão sequencial anterior foi substituída; nenhuma etapa libera LoRA.

O fluxo usa o OpenSpec existente, o controlador `review_corpus.py` e as personas atuais.
Não é outro framework de agentes nem um serviço em segundo plano. Funciona na sessão
principal do Claude ou Codex; a skill compartilhada é carregada automaticamente conforme
as instruções do projeto ao propor liberação de corpus ou treinamento.

## Etapas

1. Conferir corpus e auditoria, validar o contrato OpenSpec e reproduzir os exemplos pelo Go.
2. Executar os testes do gerador e das regras financeiras, sem modelo ou provedor externo.
3. Fazer triagem lexical de promessas e ações indevidas; isso não substitui revisão semântica.
4. Gerar pacote de revisão com contexto, exemplos e hash de corpus/contrato/código financeiro.
5. A sessão distribuir os eixos financeiro, segurança e qualidade às personas existentes.
6. Consolidar os três pareceres; rejeitar contexto antigo, papéis duplicados, achados inválidos
   ou aprovação que contenha bloqueante. Qualquer reprovação impede concluir a revisão.

Cada subprocesso de verificação tem limite de 60 segundos e seu grupo é encerrado em
timeout. Os agentes seguem o limite de cinco ciclos definido na skill. Não são chamados
pelo script Python: o cliente de IA os executa durante a sessão, sem SDK ou API paga nova.
Sem subagentes disponíveis, a sessão revisa os eixos sequencialmente e registra a limitação.

## Comandos

Prepare um corpus pelo laboratório e use o caminho da exportação:

```sh
python3 tools/ai-lab/review_corpus.py CAMINHO_DO_CORPUS
python3 tools/ai-lab/review_corpus.py CAMINHO_DO_CORPUS --reviews CAMINHO_DOS_PARECERES
```

No Claude, `/review-ai-corpus CAMINHO_DO_CORPUS` chama o mesmo fluxo. No Codex, use a
skill `review-financial-corpus` ou a instrução automática do projeto. A lista nativa de
skills desta sessão já reconheceu a nova skill; clientes sem recarga ainda podem ler o arquivo.

Saídas em `.artifacts/ai-lab/corpus-review-*/`:
- `report.json`: checks, contexto, modo de execução, pareceres e bloqueios.
- `packet.json`: dados fictícios e instruções para revisão somente leitura.
- `go-tests.log`: evidência dos testes de regras quando executados.

O pacote de pareceres deve conter somente `review_context_sha256`, `execution_mode`
(parallel_agents/sequential_session/mixed) e `reviews`. São exigidos os eixos financeiro,
seguranca e qualidade. Cada parecer informa revisor, veredito approve/request_changes e
achados por ID, com severity blocking/warning e descrição. Esses registros são evidências
locais autodeclaradas; não são autenticação do revisor nem aprovação nativa do GitHub.

## Estados e liberação

- `aguardando_revisoes_especializadas`: checks concluídos, pareceres ainda ausentes.
- `revisao_ai_concluida`: checks e três pareceres técnicos sem bloqueantes.
- `blocked`: erro de integridade, check reprovado, contexto divergente ou parecer bloqueante.

Exit code zero nos dois primeiros estados **não libera treinamento**. Em todos os estados,
`training_allowed` permanece falso e a decisão de liberação é separada. Não remover a
revisão humana exigida no contrato nem iniciar treino só porque os testes de código passaram.
Ao arquivar ou substituir o contrato OpenSpec, atualizar a referência do controlador e
validar os novos vínculos; contrato ausente deve bloquear, não acionar uma aprovação padrão.

## Prova real de funcionamento

Corpus: `.artifacts/ai-lab/dataset-cc8ws5oh/`.
Pacote: `.artifacts/ai-lab/corpus-review-cjjzik69/packet.json`.
Pareceres: `.artifacts/ai-lab/corpus-review-cjjzik69/reviews.json`.
Resultado consolidado: `.artifacts/ai-lab/corpus-review-70uhtaqs/report.json`.

Os cinco checks determinísticos passaram. A revisão financeira e a de qualidade
reprovaram o corpus, e o controlador retornou exit code 1, `blocked` e treinamento
desabilitado. Não foi necessário adulterar um exemplo para demonstrar o bloqueio.

O test-engineer concluiu revisão independente. Os subagentes de finanças e segurança
ficaram indisponíveis por cota; esses eixos foram executados na sessão principal com
as personas lidas integralmente. O modo registrado é `mixed`, não três revisões independentes.
O revisor de código também ficou indisponível; testes e inspeção do controlador não
devem ser apresentados como aprovação independente desse código.

O histórico das causas e correções está no [relatório da mudança](../../openspec/changes/specialized-financial-ai/review-findings.md).

## Segunda rodada após correção das causas

Corpus regenerado em `.artifacts/ai-lab/dataset-z3ygxc3f/`; pareceres em
`.artifacts/ai-lab/corpus-review-81h1oz04/reviews.json`; consolidação em
`.artifacts/ai-lab/corpus-review-8pfsx2h4/report.json`.
Os pareceres da primeira rodada foram rejeitados por contexto incompatível em
`.artifacts/ai-lab/corpus-review-kmogv1zh/report.json`.

O novo resultado é `revisao_ai_concluida`, com avisos e `execution_mode: sequential_session`.
As personas foram aplicadas na sessão principal, sem nova revisão independente. A aprovação
técnica vale para a semente que testa o pipeline, não para treino ou generalização.
Revisão humana, diversidade e licenças do runtime permanecem pendentes; treinamento falso.

## Terceira rodada independente — resultado vigente

Pareceres em `.artifacts/ai-lab/corpus-review-a3q0ph26/reviews.json`; contas e sondas
preservadas em `pareceres/` dessa pasta principal. Relatório consolidado em
`.artifacts/ai-lab/corpus-review-39awpqnf/report.json`: `blocked`, `parallel_agents`.
O contexto é o mesmo d9115ca5 completo da segunda rodada: o desacordo não é uma
mudança de dataset. Financeiro: 5 bloqueantes/17 avisos; segurança: 1/6; qualidade: 5/17.

Não rebaixar achados para obter aprovação. Codex corrige laboratório; Claude corrige
produto e faz a quarta revisão. A regeneração final aguarda correções na main e aviso
da pessoa. O formulário humano antigo está obsoleto; não sobrescrever nem reaproveitar
sua versão como aprovação futura. Treino pausado, sem baixar pesos ou subir dados.

## Regressão do controlador após o parecer

A nova triagem de qualidade foi aplicada em memória aos 24 registros do corpus antigo,
sem regenerá-lo ou produzir uma quarta revisão. Resultado: `ok: false`, 28 ocorrências
observáveis; 40 alvos, dos quais 16 chamadas e 24 respostas; oito recusas e nenhum
contraponto sem chamada. Essa prova demonstra que o eixo deixou de ser uma constante.
As contagens não substituem os 11 bloqueantes e 40 avisos da revisão independente,
nem representam uma nova revisão financeira ou aprovação do conjunto corrigido.

## Checagens implementadas após a terceira rodada

Qualidade verifica alvos de todos os turnos: identificadores conhecidos ou derivados do
retorno da ferramenta, datas ISO, concordância observável e molde Hoje no app em pergunta
direta. Registra contagens de chamadas, respostas, recusas e contrapontos. Não inventa
limiar percentual que ainda não foi aprovado; falta de contraponto é bloqueada.
Triagem de segurança normaliza acentos, distingue negação e citações de alerta, procura
segredos e confere se a afirmação de proposta pronta tem resultado associado de ferramenta.
É heurística com falsos positivos/negativos possíveis, não barreira de autorização.

O validador aceita ferramentas oferecidas sem chamada e conversas multivolta com
resultados vinculados; erros exigem auditoria e reconhecimento explícito, sem sucesso falso.
Os alvos LoRA são extraídos de cada turno do assistente. O contexto agora cobre também
`prepare_dataset.py`, `prepare_lora.py` e `review_corpus.py`; com `--lora CAMINHO`, inclui
manifesto e arquivos do pacote, validando vínculo e estabilidade antes/depois. Sem pacote,
o contexto registra ausência; isso não aprova futuros alvos LoRA.

Novas chamadas, sem executar nesta etapa de pausa:

```sh
python3 tools/ai-lab/review_corpus.py CAMINHO_DO_CORPUS --lora CAMINHO_LORA
```

Código/testes novos não concluem a quarta rodada. O pacote final aguarda a main corrigida.
