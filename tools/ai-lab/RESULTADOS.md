# Piloto local: 25/11/2026

## Estado

Laboratório executado. **Nenhum modelo aprovado para substituir a configuração do app.**
Não houve treinamento, RAG novo, upload, deploy ou uso de dados reais.

Evidência final local: `.artifacts/ai-lab/run-dr6di2mu/report.json` e seus arquivos JSONL.
Esse diretório é ignorado pelo Git. Para compartilhar o experimento, revisar os artefatos
e transferi-los por um canal autorizado; este resumo não substitui as respostas completas.

## Ambiente e método

- Apple M1 Pro, 16 GiB de memória unificada; Ollama 0.30.10; Python padrão em venv.
- Digests fixados em `models.json`; ambos os modelos em GGUF Q4_K_M.
- Mesmos sete casos fictícios, 32 critérios existentes, mesma revisão das fontes.
- Consultor Go real, com conhecimento e ferramentas atuais, sem um novo corpus RAG.
- Smoke textual separado, seed 42, contexto 8192, limite 160 tokens.
- Descarregamento antes/depois, fontes sem alterações durante a comparação e `/api/ps`
  vazio ao final. Nenhuma falha de infraestrutura na execução final.

## Resultados observados

| Medida | Qwen2.5-7B-Instruct | Qwen3.5-4B |
|---|---|---|
| Critérios automáticos satisfeitos | 30/32 | 27/32 |
| Casos com ao menos um critério ausente | 2/7 | 4/7 |
| Primeiro texto do smoke | 3,32 s | 3,49 s |
| Duração completa do smoke | 5,29 s | 5,14 s |
| Tamanho residente observado no Ollama | 4.979.792.280 bytes | 3.377.589.123 bytes |
| Duração do comando de avaliação Go | 151,16 s | 114,88 s |
| Estado do laboratório | Requer revisão | Requer revisão |

Uma única execução final não determina superioridade estatística nem capacidade multiusuário.
Tempos do smoke incluem carregamento; duração Go inclui preparação e várias chamadas.
Os modelos geram respostas e quantidades de tokens diferentes. Tamanho residente não é pico
de RAM do processo nem consumo total do sistema. Esses números não devem virar metas de SLA.

## Falhas e interpretação

- Qwen2.5: faltou a menção a assinaturas no caso de economia e parte da orientação esperada
  no caso de pagamento mínimo, segundo a rubrica atual.
- Qwen3.5: falhas no formato/encaminhamento ao app em metas e comparação de gastos, além
  de critérios no caso de pagamento mínimo.
- Há uma limitação confirmada na rubrica: a expressão de Uber exige `32,00` ou `32.00`,
  mas a resposta `R$ 32` representa o valor esperado. Isso é falso negativo de formato,
  não prova de erro aritmético. O critério não foi alterado para melhorar a nota.
- Prazo e economia de juros citados livremente pelo modelo exigem revisão contra as
  ferramentas Go. A nota automática não comprova que toda frase financeira está correta.
- Em execução intermediária preservada em `.artifacts/ai-lab/run-aw066p8w/`, houve ainda
  cálculo de saldo que não descontou corretamente o pagamento no caso fictício da dívida.
  Esse exemplo reforça a necessidade de testes aritméticos e revisão, não treinamento imediato.

## Autocorreção do laboratório

1. Testes provaram que um exit code zero de Go pode coexistir com critérios incompletos.
   O controlador agora reprova esse resultado e também testes pulados/sem nota.
2. Revisão estática exigiu deadline externo do smoke, limites de leitura, limpeza em falha,
   hashes completos das entradas e diretórios exclusivos. As correções receberam testes.
3. Execução real mostrou descarregamento assíncrono: `/api/ps` ainda listava o modelo
   imediatamente após `keep_alive: 0`, mas deixava de listá-lo depois. A correção aguarda
   até cinco segundos, com limite e teste de regressão. A comparação foi executada novamente.
4. A execução final preservou as notas insuficientes, não alterou os critérios e terminou
   com `needs_review` e exit code 1, como deveria.

Os testes do controlador passaram: 12 casos Python, sem modelos ou rede. `make test`
também passou. A revisão estática aprovou as correções principais; ela não certifica
qualidade financeira. Os gates finais de engenharia registram sua própria evidência local.

## Próxima etapa recomendada

Antes de ampliar o corpus ou treinar LoRA, revisar os casos e construir uma rubrica que
distinga equivalência monetária, correção dos cálculos, segurança, formato e utilidade.
Preservar todos os requisitos; revisar falsos negativos por justificativa independente,
nunca para aprovar um modelo específico. Acrescentar rastreamento de ferramentas, medição
de pico, repetições e casos adversariais, com metas definidas antes da próxima comparação.

RAG e fine-tuning seguem pendentes. Uma aprovação futura exige demonstrar ganho contra
essa referência e fechar os itens abertos da [arquitetura](../../openspec/changes/specialized-financial-ai/design.md).
