## ADDED Requirements

### Requirement: Ritmo do mês compara a mesma base
A análise do mês SHALL projetar o resto do mês pelo que os meses anteriores gastaram depois
do dia de hoje e SHALL comparar o ritmo só com gastos do dia a dia até o mesmo dia do mês
passado. Parcelas mínimas que faltam pagar SHALL entrar na projeção e no que ainda cabe no mês.

#### Scenario: Compra do mês feita cedo
- **WHEN** o mercado do mês foi comprado no dia 8, como no mês passado, e hoje é dia 22
- **THEN** a projeção não repete essa compra até o fim do mês
- **AND** com R$ 10,00 a mais até o mesmo dia, não aparece "Gastando mais rápido que no mês passado"

#### Scenario: Gasto do dia a dia maior de verdade
- **WHEN** a partir do dia 7, o gasto variável até hoje passa em mais de 10% o do mês passado até o mesmo dia
- **THEN** a sugestão de ritmo mostra a diferença até hoje

#### Scenario: Sem histórico
- **WHEN** não há meses anteriores com lançamentos
- **THEN** a projeção do gasto variável segue o ritmo de até hoje

#### Scenario: Parcela mínima pendente
- **WHEN** uma dívida tem parcela mínima que ainda não foi lançada no mês
- **THEN** a parcela entra nos fixos da projeção e sai do que ainda cabe no mês
- **AND** depois de lançada em "Pagamento de dívidas", não é contada de novo

### Requirement: Onde economizar não sugere cortar compromissos
As sugestões de economia SHALL deixar de fora pagamentos de dívida e contas de necessidade.
A detecção de cobranças que se repetem SHALL ignorar lugares de necessidade.

#### Scenario: Pagamento de dívida acima da média
- **WHEN** a pessoa pagou mais dívida neste mês do que na média dos meses anteriores
- **THEN** não aparece "voltar à média" para "Pagamento de dívidas"

#### Scenario: Mercado do mês e conta da casa
- **WHEN** o mercado é comprado uma vez por mês com valor parecido e a Internet é recorrente
- **THEN** o mercado não entra na lista de assinaturas
- **AND** a sugestão de revisar assinaturas soma só as que não são necessidade, no singular quando é uma
