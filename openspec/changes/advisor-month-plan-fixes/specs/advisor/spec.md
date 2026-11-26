## ADDED Requirements

### Requirement: Plano mensal distingue orçamento de pagamento estimado
O consultor SHALL reservar fixos ainda não lançados e mínimos pendentes, sem repetir
valores pagos, e SHALL distinguir orçamento disponível de pagamento estimado.

#### Scenario: Conta fixa pendente
- **WHEN** uma recorrência de saída está ativa no mês e seu lançamento ainda não existe
- **THEN** seu valor é reservado antes de calcular a sobra do plano
- **AND** recorrências pausadas, fora da vigência ou com lançamento já existente não são reservadas novamente

#### Scenario: Lançamento recorrente excluído intencionalmente
- **WHEN** o lançamento gerado de uma recorrência existe mas está excluído
- **THEN** o plano não o recria nem o reserva novamente

#### Scenario: Orçamento maior que o saldo estimado
- **WHEN** a dívida tem saldo de 240000 centavos e juros mensais de 21600 centavos
- **THEN** o pagamento estimado não excede 261600 centavos nem o orçamento reservado
- **AND** o texto orienta confirmar a quitação com o credor antes de pagar

#### Scenario: Pagamento já lançado
- **WHEN** há pagamentos de dívida no mês, agregados por categoria
- **THEN** os mínimos pendentes descontam esses pagamentos e o extra não repete o mínimo do alvo
- **AND** o consultor explicita que não identifica qual dívida recebeu o pagamento

### Requirement: Simulações explicitam hipóteses e calendário
O consultor SHALL diferenciar estimativa genérica de limites contratuais ou legais e
SHALL mostrar aportes e mês de chegada sem confundir quantidade de aportes com tempo decorrido.

#### Scenario: Dívida sem modalidade ou encargos conhecidos
- **WHEN** a ferramenta simula uma dívida com taxa e parcela constantes
- **THEN** informa juros mensais antes do pagamento e primeiro pagamento no próximo mês
- **AND** não presume que um teto de cartão seja aplicável a qualquer empréstimo

#### Scenario: Aporte no mês atual
- **WHEN** faltam 600000 centavos e a pessoa guarda 40000 centavos por mês desde outubro de 2026
- **THEN** a resposta informa dezembro de 2027, 15 aportes e início neste mês em frase curta
