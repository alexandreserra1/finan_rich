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
| Assinaturas e quanto custam por ano | Rocket Money | Lista de assinaturas (recorrentes cadastrados + cobranças que se repetem) com custo anual |
| Gastos pequenos e frequentes | "Latte factor" (David Bach), popular nos apps de orçamento | Lugar com muitas compras pequenas em 30 dias |
| Divisão 50/30/20 | Elizabeth Warren, *All Your Worth* | Já existe no Painel (Divisão do salário); a análise avisa quando "Desejos" estoura |

## Regras (`internal/finance/insights.go`)

- **Fixos x variáveis**: fixo é o lançamento gerado por recorrência (ID `rec-…`); o resto é
  variável. Investimentos não contam como gasto.
- **Ritmo**: gasto variável acumulado por dia neste mês e no mês passado. Projeção do mês =
  variável até hoje ÷ dias corridos × dias do mês + fixos do mês (os que já caíram e os que
  ainda vão cair).
- **Pode gastar por dia**: renda do mês (ou a média dos 3 meses anteriores, se ainda não
  entrou) − investimento planejado (percentual da divisão) − gastos do mês − recorrentes que
  ainda vão cair, dividido pelos dias que faltam (contando hoje). Nunca negativo.
- **Onde foi**: por lugar (iFood, Uber… reconhecido pela descrição, como no consultor) e por
  categoria, com a fatia de cada um e a comparação com a média de 3 meses.
- **Onde economizar** (cada item com a economia estimada por mês, maior primeiro):
  - categoria acima da média de 3 meses (pelo menos 25% e R$ 50 a mais): economia = diferença;
  - lugar com 8 compras ou mais em 30 dias e tíquete médio até R$ 60: economia = metade;
  - assinaturas: soma por mês e por ano, para a pessoa revisar;
  - "Desejos" acima do planejado na divisão do salário: economia = o que passou;
  - ritmo 10% acima do mês passado: quanto a projeção passa do mês passado.
- **Maiores gastos do mês**: os 5 maiores lançamentos de saída.

O consultor também recebe o resumo da análise (ritmo, projeção e sugestões), para responder
"onde posso economizar?" com os mesmos números da tela.
