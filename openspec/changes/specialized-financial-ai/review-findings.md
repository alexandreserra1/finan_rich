# Revisão do corpus e correção das causas financeiras

**Estado vigente em 26/11/2026: blocked.** A terceira rodada independente substitui a
conclusão técnica sequencial da segunda, no mesmo contexto d9115ca5. Há 11 bloqueantes
e 40 avisos; nenhum foi rebaixado. As seções anteriores são histórico, não aprovação.

Data: 25/11/2026. Revisão técnica assistida por IA, não certificação financeira.
Contexto: `5b22bf3e7ee9e5b05c639e6f8b58ec3247b70a5c8d676529d79817481c014e4d`.
Corpus sintético com 24 exemplos. Nenhum treinamento financeiro foi iniciado.

## Bloqueante 1: orçamento não é pagamento efetivo

Em `consulta-periodo-1` e `consulta-periodo-2`, o contexto gerado sugere mandar
R$ 3.160,10 à dívida, com saldo informado de R$ 2.400,00 e juros mensais de R$ 216,00.
A sugestão supera até saldo mais esses juros em R$ 544,10. A recorrência conhecida
de Internet, R$ 90,00, também precisa ser reconciliada com os compromissos do exemplo.

Confirmado no pacote e nos textos de `internal/advisor/snapshot.go`, a partir de
`internal/finance/monthplan.go`. Isso demonstra um problema no contexto de treinamento;
o alcance nos fluxos reais do app ainda precisa de investigação, inclusive a materialização
de recorrentes. Não afirmar que todas as contas do produto ignoram esses compromissos.

Correção necessária: distinguir orçamento disponível, compromissos e valor efetivo de
quitação. Definir premissas e testes antes de mudar a regra ou a redação. Não mascarar
o problema editando só os alvos do corpus. Regenerar contexto e revisar novamente.

## Bloqueante 2: premissas incompatíveis na dívida

Em `simulacao-divida-1/2`, o contexto inclui afirmações sobre limite do rotativo e
crescimento para sempre, enquanto a simulação genérica informa R$ 2.673,90 de juros
futuros para um saldo de R$ 2.600,00, sem modalidade ou principal original esclarecidos.

Não é prova de qual teto legal aplicar ao registro: faltam premissas e fonte atualizada.
`internal/advisor/knowledge.go` e `internal/finance/debts.go` precisam ser reconciliados
com modalidade, escopo e hipóteses. Não aplicar automaticamente um limite genérico a
todas as dívidas. Esse ponto continua pendente de revisão financeira e de fontes.

## Outros achados

- Explicitar que a meta de 16 aportes inclui o mês atual; não chamar isso de 16 meses
  transcorridos. As convenções de dívida e meta diferem e precisam estar claras.
- Os exemplos com chamadas oferecem somente a ferramenta esperada; faltam seleção
  entre alternativas, parâmetros inválidos e falhas do executor.
- Dois enunciados por família não fornecem diversidade suficiente para generalização.
- Reproduzir o mesmo Go não é um oráculo independente. Acrescentar expectativas revisadas
  separadamente, com premissas, unidades e arredondamentos.
- A revisão semântica não prova resistência do modelo a injeção nem substitui controles
  de autorização. Segurança em produção e licenças do runtime continuam pendentes.

## Decisão da primeira rodada

Checks de código aprovados não liberam este corpus. Os pareceres foram consolidados
com modo misto: QA em subagente; financeiro e segurança sequenciais na sessão principal
após indisponibilidade por cota. Essas limitações estão nos registros dos revisores.

O orquestrador funcionou e bloqueou a liberação. Na primeira rodada, os achados financeiros
ainda não estavam corrigidos. Os registros originais foram preservados como evidência.

## Segunda rodada: conclusão técnica sequencial, posteriormente substituída

Contexto: `d9115ca5e5cc3004ae4d69ce6f22cc74458c1c0eb10b7c259db8fab700b75f3b`.
Corpus regenerado: `.artifacts/ai-lab/dataset-z3ygxc3f/`.
Pareceres: `.artifacts/ai-lab/corpus-review-81h1oz04/reviews.json`.
Resultado: `.artifacts/ai-lab/corpus-review-8pfsx2h4/report.json`.

- `PlanMonth` reserva recorrências ativas ainda não lançadas, respeitando pausa, vigência
  e exclusão intencional do lançamento gerado. O plano distingue orçamento de pagamento
  estimado, limitado ao saldo com os juros do primeiro período. Não é cotação de quitação.
- Pagamentos já lançados não repetem o mínimo. Como a categoria agrega dívidas, o plano
  reserva conservadoramente os mínimos restantes sem atribuir pagamentos a uma dívida.
- A simulação informa taxa constante, juros antes do pagamento e primeiro pagamento no
  próximo mês. O conhecimento distingue o teto específico do cartão da simulação genérica;
  não aplica teto sem modalidade, principal original e encargos conhecidos.
- Metas informam quantidade de aportes, incluindo o primeiro no mês atual, sem confundir
  essa quantidade com meses transcorridos.

Testes em `internal/finance/monthplan_test.go` e
`internal/advisor/monthplan_regression_test.go` cobrem os defeitos e uma matriz de 48
combinações de orçamento. As expectativas monetárias foram calculadas separadamente.
Uma conferência com Decimal reproduziu a dívida de R$ 2.600 a 9%: mínimo de R$ 300
resulta em 18 períodos e R$ 2.673,90 de juros; R$ 750 resulta em cinco períodos e
R$ 661,88. Isso usa truncamento a centavos em cada período, como o Go atual; contratos
podem adotar arredondamentos e encargos diferentes. Não alteramos a convenção para passar.

Os três eixos foram revistos sequencialmente na sessão principal, sem independência da
implementação. Não confundir esses pareceres com os subagentes da primeira rodada.
O controlador rejeitou os pareceres antigos por contexto divergente e concluiu a nova
revisão técnica. `training_allowed` continua falso; revisão humana, ampliação das famílias,
seleção entre ferramentas e validação do runtime completo permanecem pendentes.

## Terceira rodada: revisão independente em paralelo

Data: 26/11/2026. Mesmo contexto completo da segunda rodada:
`d9115ca5e5cc3004ae4d69ce6f22cc74458c1c0eb10b7c259db8fab700b75f3b`.
Pareceres: `.artifacts/ai-lab/corpus-review-a3q0ph26/reviews.json`.
Relatório: `.artifacts/ai-lab/corpus-review-39awpqnf/report.json`.
Contas e scripts: `.artifacts/ai-lab/corpus-review-a3q0ph26/pareceres/` na pasta principal.
Resultado: `blocked`, `parallel_agents`, `training_allowed: false`.

| Eixo | Bloqueantes | Avisos | Veredito |
|---|---:|---:|---|
| Financeiro | 5 | 17 | request_changes |
| Segurança | 1 | 6 | request_changes |
| Qualidade | 5 | 17 | request_changes |

### Conferência dos bloqueantes no código antes de corrigir

Os índices abaixo são locais a cada eixo, na ordem do parecer original; não substituem IDs.

- Financeiro 1, geral: confirmado em `insights.go`, projeção linear e comparação de bases
  distintas. A correção da regra e sua revisão cabem ao Claude, não ao gerador.
- Financeiro 2–3, achados-orcamento-1/2: confirmado em `corpusgen/main.go`, que colava
  o alerta da ferramenta e usava livro com recorrência nunca materializada. Sem aprovação
  do alvo até correção da regra e regeneração.
- Financeiro 4–5, vencimentos-1/2: confirmado no fixture sem pagamento/materialização
  e na ferramenta que olha adiante. Novo livro deve ser coerente ou declarar o atraso;
  correção de vencimentos do produto cabe ao Claude.
- Segurança 1, injecao-instrucoes-2: confirmado: nota inexistente, pedido direto, sem
  ferramentas de escrita nem casos adversariais nos conjuntos reservados.
- Qualidade 1, geral: confirmado no molde `Resultado consultado no app` + tool + fecho;
  contradiz a primeira frase direta prevista pelo prompt de produção.
- Qualidade 2, geral: confirmado: todas as respostas sem ferramenta eram recusas ou
  pedido de dados, sem taxas datadas ou contas prontas como contraponto.
- Qualidade 3, geral: confirmado: prompt sem voz do personagem e cardápio 0/1 ferramenta;
  o validador antigo recusava ferramenta oferecida sem chamada.
- Qualidade 4, geral: confirmado: validação/teste com poucas famílias e ausência de
  avaliador sobre esses conjuntos. Corrigir cobertura não demonstra ganho do modelo.
- Qualidade 5, dados-insuficientes-2: confirmado no alvo que pergunta o saldo declarado
  desconhecido e ignora empréstimo já cadastrado. Orientar onde consultar e conferir duplicidade.

Não houve discordância desses bloqueantes nesta conferência. Contas corretas de dívida,
meta e datas não resolvem esses problemas. Os 40 avisos seguem no parecer original;
controlador, alvos, livros, privacidade, cobertura e concordância exigem teste e revisão.

### Decisão e coordenação

O PR #52 move cenários do plano mensal para `advisor-month-plan-fixes`, capacidade
`advisor`. Depois dele, o Codex não altera `internal/advisor` nem `internal/finance`.
Claude corrige app, revisa PRs e faz merge; Codex cuida de laboratório, contrato e skills.
O treino está pausado, sem downloads de pesos, upload ou publicação de modelo.
`Revisao-IA-Rich.docx` ficou obsoleto e não será sobrescrito. Formulário novo somente
depois de revisão sem bloqueantes, com outro nome e o hash do novo corpus.
Regeneração final depende de aviso da pessoa após correções na main. Ao concluir o
gerador, produzir pacote de revisão e parar para a quarta rodada do Claude.
