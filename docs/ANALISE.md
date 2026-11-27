# Análise de gastos

Tela **Análise** (`/analise`, link no Painel e no menu do computador): responde "gastei onde?",
"estou gastando rápido demais?" e "onde dá para economizar?". Tudo calculado no aparelho, em Go
(`finance.Insights`, pelo WebAssembly), só com os dados da pessoa; nada vai para a internet.

## Referências (apps americanos)

| Ideia | De onde vem | Como fica aqui |
|---|---|---|
| Ritmo do mês: linha do gasto acumulado deste mês contra o do mês passado, com projeção de como o mês fecha | Copilot Money (gráfico do Dashboard; tira as contas recorrentes para não dar degrau) | Gráfico "Ritmo do mês" só com gastos variáveis; os fixos (recorrentes) entram na projeção |
| "Quanto posso gastar hoje" | PocketGuard ("In My Pocket" / "safe to spend") | "Pode gastar R$ X por dia até o fim do mês" |
| Comparar cada categoria com a própria média | Monarch Money, Copilot (categorias contra o mês passado) | Categoria contra a média dos 3 meses anteriores |
| Assinaturas e quanto custam por ano | Rocket Money | Lista de assinaturas (recorrentes cadastrados + cobranças que se repetem, menos as de necessidade, como o mercado do mês) com custo anual |
| Gastos pequenos e frequentes | "Latte factor" (David Bach), popular nos apps de orçamento | Lugar com muitas compras pequenas em 30 dias |
| Divisão 50/30/20 | Elizabeth Warren, *All Your Worth* | Já existe no Painel (Divisão do salário); a análise avisa quando "Desejos" estoura |

## Regras (`internal/finance/insights.go`)

- **Fixos x variáveis**: fixo é o lançamento gerado por recorrência (ID `rec-…`) e o
  pagamento de dívida (categoria "Pagamento de dívidas"); o resto é variável. Investimentos
  não contam como gasto.
- **Ritmo**: gasto variável acumulado por dia neste mês e no mês passado. Projeção do mês =
  variável até hoje + o que os meses anteriores (até 3, só os que têm lançamentos) gastaram,
  em média, depois deste dia + o que está agendado para depois de hoje (parcelas) + fixos do
  mês (os que já caíram, os recorrentes que ainda vão cair e as parcelas mínimas de dívidas
  que faltam pagar, como no Plano do mês). Multiplicar o gasto pelo ritmo repetiria a compra
  do mês feita cedo (o mercado do dia 8); só sem histórico a projeção segue o ritmo de até
  hoje (variável ÷ dias corridos × dias do mês).
- **Pode gastar por dia**: renda do mês (ou a média dos 3 meses anteriores, se ainda não
  entrou) − investimento planejado (percentual da divisão) − gastos do mês − recorrentes e
  parcelas mínimas que ainda vão cair, dividido pelos dias que faltam (contando hoje). Nunca
  negativo.
- **Onde foi**: por lugar (iFood, Uber… reconhecido pela descrição, como no consultor) e por
  categoria, com a fatia de cada um e a comparação com a média de 3 meses. O nome do lugar é a
  descrição inteira quando ela vem sempre igual ("Pão de Açúcar"), o que vem antes do "*" no
  extrato do cartão ("UBER *TRIP" → "UBER") e só a primeira palavra quando o mesmo lugar vem
  escrito de jeitos diferentes; sem o "(2/3)" das parcelas. Pagamento de dívida não é lugar.
- **Onde economizar** (cada item com a economia estimada por mês, maior primeiro):
  - categoria acima da média de 3 meses (pelo menos 25% e R$ 50 a mais): economia = diferença.
    "Pagamento de dívidas" fica de fora: pagar mais dívida não é gasto a cortar;
  - lugar com 8 compras ou mais em 30 dias e tíquete médio até R$ 60: economia = metade;
  - assinaturas: soma por mês e por ano das que não são necessidade (aluguel e contas da casa
    não são corte), para a pessoa revisar;
  - "Desejos" acima do planejado na divisão do salário: economia = o que passou;
  - ritmo: gasto variável até hoje 10% acima do mês passado até o mesmo dia, a partir do
    dia 7 (fixos não são ritmo): economia = a diferença até hoje.
- **Maiores gastos do mês**: os 5 maiores lançamentos de saída.

O consultor também recebe o resumo da análise (ritmo, projeção e sugestões), para responder
"onde posso economizar?" com os mesmos números da tela.
