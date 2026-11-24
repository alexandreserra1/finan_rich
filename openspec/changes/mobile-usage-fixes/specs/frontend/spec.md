## ADDED Requirements

### Requirement: Categoria sugerida no lançamento novo
No lançamento novo, a categoria SHALL vir da descrição, com as regras da importação, enquanto a
pessoa não escolher uma categoria; a escolha dela SHALL prevalecer.

#### Scenario: Gasto de transporte
- **WHEN** a pessoa digita "Uber" na descrição de um gasto novo, sem mexer na categoria
- **THEN** a categoria passa a ser Transporte

### Requirement: Permissão de notificação pedida uma vez
Sem a permissão de notificação, o app SHALL deixar os lembretes do time desligados e SHALL NOT
pedir de novo a cada lançamento; ligar os lembretes nos Ajustes SHALL pedir de novo e avisar se
a permissão não vier.

#### Scenario: A pessoa recusa
- **WHEN** a pessoa recusa a notificação e depois salva outro gasto
- **THEN** o app não pergunta de novo, e os Ajustes mostram os lembretes desligados
