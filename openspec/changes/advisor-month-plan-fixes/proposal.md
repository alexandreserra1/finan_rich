# Corrigir orçamento mensal e premissas do consultor

## Why
O retrato do consultor confundia orçamento com pagamento estimado, não reservava fixos
pendentes e podia repetir mínimos já pagos. A meta confundia aportes com meses decorridos.

## What Changes
- Reservar recorrências pendentes conforme pausa, vigência e lançamentos existentes.
- Separar orçamento e pagamento estimado, sem ultrapassar saldo com juros do primeiro período.
- Evitar repetir mínimos e explicitar a limitação de pagamentos agregados por categoria.
- Explicar hipóteses de simulação, sem aplicar teto legal genérico a empréstimos.
- Usar frase curta com mês de chegada e quantidade de aportes, incluindo o mês atual.

## Impact
- Capability: advisor. Regras Go compartilhadas com Wasm; contexto e respostas do consultor.
- Sem tabelas, migrações ou dependências novas. Não altera provedores de IA.
- Não corrige projeção de ritmo, diagnóstico, atrasos ou demais achados da terceira revisão:
  serão tratados pelo Claude em mudanças separadas. Não aprova o corpus nem libera LoRA.
