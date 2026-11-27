# Corrigir os defeitos do app achados na revisão do corpus

## Why
A terceira revisão do corpus da IA (26/11/2026, três revisores independentes) achou números
errados que vêm do próprio app, não só do corpus. O mais grave: o aviso "Gastando mais rápido
que no mês passado" multiplicava a compra do mês feita cedo pelos dias restantes e acusava
R$ 206,32 a mais quando o aumento real era de R$ 10,00. O consultor repete esses números como
corretos, então treinar ou responder em cima deles ensina conclusões falsas.

## What Changes
- Ritmo do mês: a projeção soma ao gasto de até hoje o que os meses anteriores gastaram depois
  deste dia, e o aviso compara só gastos do dia a dia até o mesmo dia do mês passado.
- Parcelas mínimas que faltam pagar entram na projeção e saem do que ainda cabe no mês, sem
  contar duas vezes o que já foi lançado.
- Os demais achados (sugestões de economia, atrasos, diagnóstico e plurais) entram em PRs
  separados desta mesma mudança.

## Impact
- Capability: advisor. Regras Go de `internal/finance` (também no WebAssembly), textos do
  consultor e o gráfico da tela Análise, que passa a usar a projeção calculada em Go.
- Sem tabelas, migrações ou dependências novas. Não aprova o corpus nem libera LoRA.
