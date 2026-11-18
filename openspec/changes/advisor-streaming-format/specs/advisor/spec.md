## ADDED Requirements

### Requirement: Resposta aos poucos sem expor apelidos
O Consultor SHALL mostrar a resposta da IA na nuvem enquanto ela chega, já com os nomes
verdadeiros, e SHALL NOT mostrar um apelido, inteiro ou partido, na tela.

#### Scenario: Apelido partido entre dois pedaços
- **WHEN** a IA manda "Primeiro pedaço sobre a Loj" e, depois, "a AB e o resto."
- **THEN** a tela mostra "Primeiro pedaço sobre a " antes do segundo pedaço
- **AND** a resposta final mostra o nome verdadeiro no lugar de "Loja AB"

#### Scenario: No app do celular
- **WHEN** o `CapacitorHttp` troca o fetch por um que entrega a resposta inteira no fim
- **THEN** o pedido à IA na nuvem vai pelo fetch do WebView e a tela mostra o texto enquanto chega

#### Scenario: Texto final igual ao de antes
- **WHEN** a resposta é partida em pedaços de qualquer tamanho
- **THEN** a junção dos pedaços mostrados é igual à resposta inteira com os nomes trocados

### Requirement: Formato pelo tipo de pergunta
As instruções do Consultor SHALL pedir resposta curta, sem plano em passos, para pergunta
direta, e o formato de plano para pedido de conselho, decisão ou plano.

#### Scenario: Pergunta direta
- **WHEN** a pessoa pergunta "quanto gastei com Uber esse mês?"
- **THEN** a resposta traz o valor em até umas 60 palavras, sem passos numerados

#### Scenario: Gasto num lugar
- **WHEN** a pessoa pergunta "quanto gastei com uber?" e há lançamentos da Uber
- **THEN** o Otávio recebe o total do mês e de sempre já calculado pelo app
- **AND** sem IA, a resposta do app mostra esse total

#### Scenario: Pedido de conselho
- **WHEN** a pessoa pergunta "o que eu faço com meu dinheiro?"
- **THEN** a resposta começa pela conclusão em negrito e termina com um próximo passo no app
