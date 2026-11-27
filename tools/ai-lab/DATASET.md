# Corpus supervisionado inicial

**Estado vigente: blocked, quarta rodada (27/11/2026).** Três revisores independentes,
`parallel_agents`, reprovaram o contexto 9f32c90c: 18 bloqueantes e 34 avisos, incluindo
repetições. As novas correções ainda precisam da quinta revisão; não representam aprovação.
LoRA pausado; formulário antigo obsoleto e preservado. As contagens iniciais abaixo são histórico.

## Correções preparadas para a quinta rodada

Base: main `748df75`, com correções do app #62–#64. O gerador passa pela API real do
consultor com provedor roteirizado local, sem modelo. Os livros incluem parcelas pagas
na categoria correta e contas fixas sem duplicação. O treino inclui um perfil com duas
dívidas; compras históricas variam, em vez de repetir todo mês o mesmo valor.

- Alterar/apagar usa a compra do período da busca, não o primeiro lançamento histórico.
- Vencimentos futuros e pendências antigas são separados na resposta.
- Propostas registram cartões completos do executor; pagamento descreve os dois cartões.
- Onboarding preserva a pergunta sobre renda antes das contas fixas.
- Injeções cobrem descrição, loja e nota nos três conjuntos e personagens, sem marcador fixo
  nem pedido do usuário que já diga para ignorar o ataque. O fake não prova resistência real.
- Ferramenta oferecida sem chamada, vencimentos bem-sucedidos, golpe e busca vazia aparecem
  em todos os conjuntos. Há erros com recuperação; chamadas rejeitadas não viram alvos positivos.
- Taxas com e sem fonte formam pares da mesma pergunta dentro de cada conjunto; as respostas
  citam Banco Central/data ou explicam a ausência. Ficção fica na auditoria, não na resposta.
- Nomes, valores e datas são mascarados ao procurar duplicatas de perguntas e alvos entre
  conjuntos. Os nomes usados ficam em `dedup_names`; renomear uma família não contorna o gate.

`proposals` e `injection_location` são metadados de auditoria. IDs aleatórios de registros
novos são substituídos por IDs fictícios estáveis só nesses metadados; IDs de registros
existentes e seu estado anterior não são trocados. Nenhum cartão é aplicado.

O manifesto exportado contém as contagens atuais, os hashes e `training_allowed: false`.
O contexto da revisão cobre também o avaliador, runtime-model.json e o comparador reservado.
O comparador aceita critérios de chamadas e cartões; isso não executa modelos nem dispensa
rubrica financeira independente. Corpus pequeno, seleção roteirizada e proximidade semântica
continuam exigindo revisão humana/especializada. Não usar a sobra do Plano como autorização
de pagamento: investimento planejado e reserva inicial ainda dependem de decisão de produto.

Estado: gerador e exportador implementados; **não houve treinamento do Qwen com este corpus**.
Preparação autorizada após o piloto. Não há dados de usuários, geração por outro LLM,
upload ao Hugging Face ou acesso a banco de produção.

Etapa posterior: [preparação de alvos LoRA e preflight](LORA.md). O smoke numérico dessa
etapa usa uma camada aleatória pequena, não treina o Qwen ou os exemplos financeiros.

## Conteúdo e separação

O gerador Go contém 12 famílias de cenários, com duas perguntas por família: 24 exemplos.
As variações de uma família ficam sempre juntas. A divisão explícita serve à avaliação
por cenário, não pretende representar uma amostra estatística do uso real.

| Conjunto | Exemplos | Famílias |
|---|---|---|
| Treino | 16 | período, categoria, simulação de dívida, simulação de meta, orçamento, comparação de meses, injeção de instruções, dados insuficientes |
| Validação | 4 | vencimentos e taxa atual sem fonte |
| Teste reservado | 4 | busca sem resultado e promessa de retorno |

Cada família recebe um livro fictício próprio, com valores em centavos e data fixa.
Os testes finais não são usados para ajustar hiperparâmetros nem escolher exemplos de treino.
Ao ampliar, manter essa regra para documentos de origem e variações semanticamente próximas.
O detector atual encontra duplicatas normalizadas; revisão semântica humana continua necessária.

## Origem dos exemplos

- Perguntas e respostas de segurança foram escritas no código do projeto como cenários fictícios.
- Contexto e conhecimento usam `advisor.BuildRequest`, `BuildFacts` e `Snapshot` atuais.
- Contratos de ferramentas vêm de `advisor.NewTools(...).Defs()`.
- Resultados vêm de `Tools.Run`, sem reimplementar dinheiro ou datas em Python.
- As ferramentas usadas são de consulta/simulação. Nenhuma proposta é aplicada ou gravada.
- Respostas após ferramentas preservam o resultado Go e acrescentam um próximo passo.
  Esse formato é uma referência técnica inicial, não o acabamento final do atendimento.

Isso verifica a execução técnica das ferramentas, não substitui revisão financeira humana.
Conhecimento estático herdado do app também precisa ser revisado quanto à fonte e validade.
Ainda faltam conversas longas, argumentos inválidos, conflitos, correção/remoção com confirmação
e diversidade suficiente de perfis. Não multiplicar paráfrases para aparentar maior cobertura.

## Reprodução

Usar o ambiente local descrito no [README](README.md):

```sh
go test ./tools/ai-lab/corpusgen
PYTHONDONTWRITEBYTECODE=1 .artifacts/ai-lab/venv/bin/python -m unittest discover -s tools/ai-lab -v
PYTHONDONTWRITEBYTECODE=1 .artifacts/ai-lab/venv/bin/python tools/ai-lab/prepare_dataset.py
```

O último comando não chama Ollama ou APIs; gera um diretório exclusivo
`.artifacts/ai-lab/dataset-*/` e imprime seu caminho. O ambiente precisa de Go, mas não
de modelos baixados ou dependências Python externas.

Saídas:
- `train.jsonl`, `valid.jsonl`, `test.jsonl`: conversas com `messages` e `tools`.
- `audit.jsonl`: IDs, famílias, conjunto, origem sintética e revisão por exemplo.
- `manifest.json`: hashes dos arquivos/fontes, contagens e `training_allowed: false`.

Os registros de chamada incluem argumentos como objeto, identificador e mensagem de retorno
ligada à chamada. Antes de escolher TRL/PEFT ou MLX, validar o template específico do modelo,
suporte às ferramentas e máscara de perda apenas nas respostas do assistente. Não treinar
o modelo para prever mensagens do usuário ou saídas do executor Go como se fossem decisões suas.
Referência de formato: [SFT Trainer oficial](https://huggingface.co/docs/trl/main/en/sft_trainer).

Cada exportação valida IDs, mensagens, famílias, perguntas normalizadas, argumentos básicos
e vínculos de ferramentas antes de escrever arquivos. O manifesto é escrito por último;
uma pasta incompleta não é um corpus liberado. Os JSONL não entram no Git: o gerador é a
fonte versionada. O conteúdo é reproduzível para a mesma revisão; nomes das pastas variam.

## Revisão antes de treinar

1. Conferir cada pergunta, contexto, chamada, resposta Go e resposta final.
2. Conferir fontes financeiras, privacidade, ausência de promessas e decisões não autorizadas.
3. Ampliar cobertura e separar casos semanticamente próximos antes da comparação final.
4. Registrar aprovação por ID/hash e responsável em um fluxo de revisão próprio; não mudar
   apenas um booleano para considerar todos os exemplos revisados.
5. Aprovar o runtime, revisões, orçamento de memória/tempo e critérios de ganho do piloto LoRA.

O corpus de 24 exemplos serve para testar o pipeline. Não há evidência de que seja suficiente
para melhorar o consultor. `training_allowed: false` registra a restrição para a futura etapa;
não impede alguém de executar manualmente um treinador externo ignorando o manifesto.

## Evidência desta preparação

Em 25/11/2026, exportação inicial em `.artifacts/ai-lab/dataset-cc8ws5oh/`: 16/4/4 exemplos,
com revisão pendente. Os arquivos de dados de exportações anteriores foram comparados e
eram idênticos; o manifesto registra também a revisão atual das fontes.

Passaram 22 testes Python do laboratório e o teste Go de reprodução do corpus. A revisão
técnica detectou e reproduziu duas falhas do validador: resposta de ferramenta não textual
e chamada pendente na mensagem final. Ambas receberam correção e testes negativos.
Isso é revisão técnica, não aprovação financeira humana dos exemplos.

Após corrigir as causas no plano mensal e explicitar as premissas das simulações, a
exportação passou a `.artifacts/ai-lab/dataset-z3ygxc3f/`, ainda com 16/4/4 exemplos.
A revisão técnica sequencial foi concluída em
`.artifacts/ai-lab/corpus-review-8pfsx2h4/report.json`, sem revisão humana ou liberação
de treinamento. A exportação antiga permanece apenas como evidência da falha detectada.

Essa conclusão da segunda rodada foi substituída pela revisão independente:
`.artifacts/ai-lab/corpus-review-a3q0ph26/reviews.json` e
`.artifacts/ai-lab/corpus-review-39awpqnf/report.json`. Contas e sondas originais em
`corpus-review-a3q0ph26/pareceres/`, na pasta principal. Não são arquivos do PR.
As próximas versões precisam interpretar ferramentas, reproduzir personagens e pedidos
de produção, testar injeção dentro dos dados e equilibrar recusas com respostas possíveis.
Livros devem ser coerentes e validação/teste devem cobrir ferramentas e perfis novos.
Regeneração final somente após correções do Claude na main e aviso da pessoa.

## Gerador refeito, ainda sem exportação final

O código agora prepara 70 cenários em memória: 28 de treino, 21 de validação e 21 de
teste. São seis perfis separados por conjunto, com variação de renda, quantidade de
dívidas, datas e contas. Livros materializam recorrências e pagamentos ou explicitam atraso.
As 14 ferramentas aparecem nos cardápios reais, inclusive criação de conta fixa pelo
onboarding. Validação e teste cobrem todas as ferramentas oferecidas pela produção.

Pedidos são capturados de `advisor.Answer` com provedor roteirizado local: prompt, voz,
retrato e cardápio não são cópias paralelas. Há injeção em descrição, loja e nota com
escrita disponível nos três conjuntos; contrapontos com taxas sintéticas datadas, números
prontos, histórico, chamadas sucessivas, propostas confirmáveis e erro do executor.
As respostas interpretam os resultados sem colar identificadores, datas ISO ou fechos fixos.

Metadados de auditoria (`ledger`, `today`, `agent`, `question`, `profile`, `app_answer`,
`rates` e `executor_error` quando presentes) não viram campos extras das mensagens-alvo.
`app_answer` serve ao diagnóstico, não é verdade automática da avaliação reservada.

O teste Go de integração passa os registros em memória pelos validadores Python e pelas
triagens; não exporta JSONL. Ele revelou três desacordos corrigidos com regressões:
reconhecimento de "consulta falhou", deduplicação pelo turno atual e alerta de golpe
confundido com promessa. A triagem também não pode usar "sem risco" para negar uma
transferência concluída. Persistem limites semânticos e necessidade de quarta revisão.

Esses números descrevem o gerador, não um corpus liberado. A exportação final, seus hashes
e o pacote da quarta rodada aguardam as correções do app na main e o aviso da pessoa.
O conjunto antigo permanece blocked; nenhum formulário novo foi criado.
