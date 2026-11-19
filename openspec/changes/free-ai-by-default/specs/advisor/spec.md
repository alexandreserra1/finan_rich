## ADDED Requirements

### Requirement: IA grátis de fábrica
No app e no site, sem configuração guardada, o Consultor SHALL conversar pela IA grátis da Rich,
sem conta e sem chave, dizendo na tela que usa a IA grátis com apelidos. Desligar SHALL ficar
guardado, e o Consultor desligado SHALL oferecer ligar de novo num toque.

#### Scenario: Primeira pergunta
- **WHEN** a pessoa abre o Consultor no app, sem nunca ter configurado a IA, e pergunta
- **THEN** a resposta vem da IA grátis da Rich, e o pedido leva só o identificador aleatório do aparelho

#### Scenario: Desligada
- **WHEN** a pessoa desliga a IA nos Ajustes
- **THEN** nenhuma pergunta sai do aparelho até ela ligar de novo

### Requirement: Computador a um clique
No computador, sem a IA local pronta, o Consultor SHALL oferecer a IA grátis da Rich num clique,
e SHALL NOT ligá-la sem a pessoa escolher.

#### Scenario: Sem Ollama
- **WHEN** o computador não tem o Ollama pronto e nenhuma IA escolhida
- **THEN** o aviso do Consultor mostra "Usar a IA grátis da Rich (sem chave)"
